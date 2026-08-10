package book

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Test keys chosen so they are free of overlaps and span the uint64 range.
const (
	keyLow  = uint64(0x1234567890abcdef)
	keyHit  = uint64(0xabcdefabcdef0001)
	keyMid  = uint64(0xabcdefabcdef0002)
	keyHigh = uint64(0xfedcba9876543210)
)

// square converts an algebraic square such as "e4" into the Polyglot square
// value (0 = a1 ... 63 = h8): rank*8 + file.
func square(s string) int {
	return int(s[1]-'1')*8 + int(s[0]-'a')
}

// encMove encodes a Polyglot move from the given squares and promotion piece
// code: to-square in bits 0-5, from-square in bits 6-11, promotion in bits 12-14.
func encMove(from, to string, promo int) uint16 {
	return uint16(square(to)&0x3f | (square(from)&0x3f)<<6 | promo<<12)
}

// appendEntry appends the 16-byte big-endian representation of e to buf.
func appendEntry(buf []byte, e Entry) []byte {
	buf = binary.BigEndian.AppendUint64(buf, e.Key)
	buf = binary.BigEndian.AppendUint16(buf, e.Move)
	buf = binary.BigEndian.AppendUint16(buf, e.Weight)
	buf = binary.BigEndian.AppendUint32(buf, e.Learn)
	return buf
}

// writeBook writes the given entries (assumed sorted by key) to a temporary
// file, opens it with Open, and registers a cleanup to close it.
func writeBook(t *testing.T, entries []Entry) *Book {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.bin")
	var raw []byte
	for _, e := range entries {
		raw = appendEntry(raw, e)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("writing test book: %v", err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%s): %v", path, err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestLookupHitWeightOrder(t *testing.T) {
	b := writeBook(t, []Entry{
		{Key: keyLow, Move: encMove("e2", "e4", 0), Weight: 1, Learn: 0},
		{Key: keyHit, Move: encMove("d2", "d4", 0), Weight: 5, Learn: 1},
		{Key: keyHit, Move: encMove("g1", "f3", 0), Weight: 20, Learn: 2},
		{Key: keyHit, Move: encMove("c2", "c4", 0), Weight: 10, Learn: 3},
		{Key: keyMid, Move: encMove("e2", "e4", 0), Weight: 2, Learn: 0},
		{Key: keyHigh, Move: encMove("d7", "d5", 0), Weight: 3, Learn: 0},
	})

	got := b.Lookup(keyHit)
	want := []Entry{
		{Key: keyHit, Move: encMove("g1", "f3", 0), Weight: 20, Learn: 2},
		{Key: keyHit, Move: encMove("c2", "c4", 0), Weight: 10, Learn: 3},
		{Key: keyHit, Move: encMove("d2", "d4", 0), Weight: 5, Learn: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lookup(%#x):\n got %+v\nwant %+v", keyHit, got, want)
	}
}

func TestLookupMiss(t *testing.T) {
	b := writeBook(t, []Entry{
		{Key: keyLow, Move: encMove("e2", "e4", 0), Weight: 1},
		{Key: keyMid, Move: encMove("d7", "d5", 0), Weight: 1},
		{Key: keyHigh, Move: encMove("c7", "c5", 0), Weight: 1},
	})

	cases := []struct {
		name string
		key  uint64
	}{
		{"below first key", keyLow - 1},
		{"between keys", keyLow + 1},
		{"just below present key", keyMid - 1},
		{"just above present key", keyMid + 1},
		{"above last key", keyHigh + 1},
		{"zero", 0},
		{"max uint64", math.MaxUint64},
	}
	for _, tc := range cases {
		if got := b.Lookup(tc.key); len(got) != 0 {
			t.Errorf("%s: Lookup(%#x) = %+v, want empty", tc.name, tc.key, got)
		}
	}
}

func TestLookupFirstAndLastKey(t *testing.T) {
	b := writeBook(t, []Entry{
		{Key: keyLow, Move: encMove("e2", "e4", 0), Weight: 7, Learn: 42},
		{Key: keyMid, Move: encMove("d7", "d5", 0), Weight: 1},
		{Key: keyHigh, Move: encMove("c7", "c5", 0), Weight: 9, Learn: 7},
	})

	first := b.Lookup(keyLow)
	if len(first) != 1 || first[0].Move != encMove("e2", "e4", 0) || first[0].Weight != 7 || first[0].Learn != 42 {
		t.Fatalf("Lookup(first key) = %+v, want the single leading entry", first)
	}

	last := b.Lookup(keyHigh)
	if len(last) != 1 || last[0].Move != encMove("c7", "c5", 0) || last[0].Weight != 9 || last[0].Learn != 7 {
		t.Fatalf("Lookup(last key) = %+v, want the single trailing entry", last)
	}
}

func TestLookupEmptyBook(t *testing.T) {
	b := writeBook(t, nil)
	if got := b.Lookup(keyLow); len(got) != 0 {
		t.Fatalf("Lookup on empty book = %+v, want empty", got)
	}
}

func TestUCIMoveDecoding(t *testing.T) {
	cases := []struct {
		name string
		move uint16
		want string
	}{
		// Normal moves: from-square in bits 6-11, to-square in bits 0-5.
		{"normal e2e4", 0x031C, "e2e4"}, // from e2(12) to e4(28)
		{"normal g1f3", 0x0195, "g1f3"}, // from g1(6) to f3(21)
		{"normal a2a3", 0x0210, "a2a3"}, // from a2(8) to a3(16)
		{"normal h1h8", 0x01FF, "h1h8"}, // from h1(7) to h8(63)

		// Promotions: promotion piece in bits 12-14 (1=knight, 2=bishop,
		// 3=rook, 4=queen).
		{"promotion queen", 0x4D3C, "e7e8q"},
		{"promotion knight", 0x1C38, "a7a8n"},
		{"promotion bishop", 0x2241, "b2b1b"},
		{"promotion rook", 0x3CFB, "d7d8r"},

		// Castling special cases: the king is encoded as moving onto the
		// rook's square and is decoded to the standard UCI castle.
		{"castling white kingside", 0x0107, "e1g1"},  // e1h1
		{"castling white queenside", 0x0100, "e1c1"}, // e1a1
		{"castling black kingside", 0x0F3F, "e8g8"},  // e8h8
		{"castling black queenside", 0x0F38, "e8c8"}, // e8a8
	}
	for _, tc := range cases {
		if got := (Entry{Move: tc.move}).UCIMove(); got != tc.want {
			t.Errorf("%s: UCIMove(%#x) = %q, want %q", tc.name, tc.move, got, tc.want)
		}
	}
}

// TestUCIMovePromotionAndCastlingThroughFile exercises the decoding end to
// end via a file lookup, confirming the stored bytes are what encMove writes.
func TestUCIMoveThroughFile(t *testing.T) {
	b := writeBook(t, []Entry{
		{Key: keyLow, Move: encMove("e1", "h1", 0), Weight: 1}, // white kingside castle
		{Key: keyMid, Move: encMove("e7", "e8", 4), Weight: 2}, // queen promotion
	})
	if got := b.Lookup(keyLow)[0].UCIMove(); got != "e1g1" {
		t.Errorf("file castle decode = %q, want e1g1", got)
	}
	if got := b.Lookup(keyMid)[0].UCIMove(); got != "e7e8q" {
		t.Errorf("file promotion decode = %q, want e7e8q", got)
	}
}

func TestTruncatedFile(t *testing.T) {
	raw := appendEntry(nil, Entry{Key: keyLow, Move: encMove("e2", "e4", 0), Weight: 1})
	raw = appendEntry(raw, Entry{Key: keyMid, Move: encMove("d7", "d5", 0), Weight: 1})

	truncated := byte(0xAB)

	var cases = []struct {
		name string
		size int
	}{
		{"one entry plus partial record", len(raw) + 7},
		{"partial single record", 8},
		{"single byte", 1},
	}
	for _, tc := range cases {
		path := filepath.Join(t.TempDir(), "trunc.bin")
		data := make([]byte, tc.size)
		copy(data, raw)
		if tc.size > len(raw) {
			data[len(raw)] = truncated
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("%s: writing file: %v", tc.name, err)
		}
		if _, err := Open(path); err == nil {
			t.Errorf("%s: Open succeeded on a file of %d bytes, want error", tc.name, tc.size)
		}
	}

	// A whole number of entries must still open: only sizes that are not a
	// multiple of 16 are rejected.
	full := filepath.Join(t.TempDir(), "full.bin")
	if err := os.WriteFile(full, raw, 0o600); err != nil {
		t.Fatalf("writing full file: %v", err)
	}
	b, err := Open(full)
	if err != nil {
		t.Fatalf("Open on whole entries failed: %v", err)
	}
	defer b.Close()
}
