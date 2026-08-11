// Package book implements a read-only reader for Polyglot opening book
// files (the .bin format used by chess engines).
//
// A Polyglot file is a sequence of fixed-size big-endian records:
//
//	bytes 0-7    uint64 Zobrist key
//	bytes 8-9    uint16 move    (encoding described on Entry.Move)
//	bytes 10-11  uint16 weight  (selection weight)
//	bytes 12-15  uint32 learn   (learning data, ignored by this reader)
//
// Records are sorted by key, so Lookup uses binary search.
//
// The Zobrist key is supplied by the caller: this package deliberately does
// not implement the Polyglot Zobrist hash, which needs the fixed 781-entry
// random table and is out of scope for a reader.
package book

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// entrySize is the size in bytes of a single Polyglot record.
const entrySize = 16

// Squares used by the four castling special cases (king-to-rook encodings).
const (
	sqA1, sqE1, sqH1 = 0, 4, 7
	sqA8, sqE8, sqH8 = 56, 60, 63
)

// Promotion piece codes in Polyglot move encoding (bits 12-14).
const (
	promoNone   = 0
	promoKnight = 1
	promoBishop = 2
	promoRook   = 3
	promoQueen  = 4
)

// Entry is a single 16-byte Polyglot book record.
type Entry struct {
	Key    uint64 // Zobrist hash of the position
	Move   uint16 // Encoded move
	Weight uint16 // Selection weight
	Learn  uint32 // Learning data (ignored by this reader)
}

// UCIMove decodes the Polyglot move encoding into long algebraic notation.
// The 16-bit encoding packs the to-square in bits 0-5, the from-square in
// bits 6-11, and the promotion piece in bits 12-14. A square value is
// rank*8 + file: bits 0-2 give the file (0 = a ... 7 = h) and bits 3-5 the
// rank (0 = 1 ... 7 = 8), so square 0 is a1 and square 63 is h8. A nonzero
// promotion code (1 = knight, 2 = bishop, 3 = rook, 4 = queen) is appended as
// "e7e8q" and friends.
//
// Castling: Polyglot encodes a castle as the king moving onto the rook's
// square (e1h1, e1a1, e8h8, e8a8). These four special cases are decoded to
// the standard UCI castling moves (e1g1, e1c1, e8g8, e8c8) so the result is
// directly acceptable to UCI engines and the notnil/chess UCI notation.
//
// The caller remains responsible for validating that the decoded castle is
// legal in the actual position: the book holds no side-to-move, castling
// rights, or other position state, so a decoded e1g1 is only meaningful when
// applied to a position where White still has kingside castling rights.
func (e Entry) UCIMove() string {
	to := int(e.Move) & 0x3f
	from := (int(e.Move) >> 6) & 0x3f
	promo := (int(e.Move) >> 12) & 0x7

	switch {
	case from == sqE1 && to == sqH1:
		return "e1g1"
	case from == sqE1 && to == sqA1:
		return "e1c1"
	case from == sqE8 && to == sqH8:
		return "e8g8"
	case from == sqE8 && to == sqA8:
		return "e8c8"
	}

	uci := squareName(from) + squareName(to)
	switch promo {
	case promoKnight:
		return uci + "n"
	case promoBishop:
		return uci + "b"
	case promoRook:
		return uci + "r"
	case promoQueen:
		return uci + "q"
	}
	return uci
}

// squareName renders a Polyglot square value (0 = a1 ... 63 = h8) as its
// algebraic name, e.g. square 12 is "e2".
func squareName(sq int) string {
	file := byte(sq % 8)
	rank := byte(sq / 8)
	return string([]byte{'a' + file, '1' + rank})
}

// Book is an open Polyglot opening book backed by a read-only file handle.
// A Book is not safe for concurrent use.
type Book struct {
	f    *os.File
	size int64 // file size in bytes, known to be a multiple of entrySize
}

// Open opens the Polyglot book at path and verifies its size is a whole
// number of entries. A file whose size is not a multiple of the 16-byte
// entry size (for example a truncated download) is rejected with an error
// rather than silently misread.
func Open(path string) (*Book, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if fi.Size()%entrySize != 0 {
		f.Close()
		return nil, fmt.Errorf("book: %s: size %d is not a multiple of %d (truncated or corrupt file)", path, fi.Size(), entrySize)
	}
	return &Book{f: f, size: fi.Size()}, nil
}

// Close releases the underlying file handle.
func (b *Book) Close() error {
	if b == nil || b.f == nil {
		return nil
	}
	return b.f.Close()
}

// Lookup returns every entry whose Zobrist key matches key, sorted by
// weight in descending order (highest selection weight first). It returns
// nil when the key is absent from the book.
//
// The book is sorted by key, so the search is a binary search over the
// records. Offsets are bounded by the file size verified in Open, so a read
// can only fail on a genuine I/O error; such an error yields an empty
// result instead of a panic.
func (b *Book) Lookup(key uint64) []Entry {
	n := int64(b.size / entrySize)

	lo, hi := int64(0), n
	for lo < hi {
		mid := lo + (hi-lo)/2
		e, err := b.at(mid)
		if err != nil {
			return nil
		}
		if e.Key < key {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= n {
		return nil
	}

	first, err := b.at(lo)
	if err != nil || first.Key != key {
		return nil
	}

	var entries []Entry
	for i := lo; i < n; i++ {
		e, err := b.at(i)
		if err != nil || e.Key != key {
			break
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Weight > entries[j].Weight })
	return entries
}

// at reads the entry at zero-based record index i.
func (b *Book) at(i int64) (Entry, error) {
	var buf [entrySize]byte
	if _, err := b.f.ReadAt(buf[:], i*entrySize); err != nil {
		return Entry{}, err
	}
	return decodeEntry(buf[:]), nil
}

// decodeEntry unpacks a 16-byte big-endian Polyglot record.
func decodeEntry(raw []byte) Entry {
	return Entry{
		Key:    binary.BigEndian.Uint64(raw[0:8]),
		Move:   binary.BigEndian.Uint16(raw[8:10]),
		Weight: binary.BigEndian.Uint16(raw[10:12]),
		Learn:  binary.BigEndian.Uint32(raw[12:16]),
	}
}
