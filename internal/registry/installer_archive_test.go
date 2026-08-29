package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// writeZipFile builds an in-memory zip archive, writes it to a temp
// file, and returns the path. Names ending in "/" are treated as
// directory entries.
func writeZipFile(t *testing.T, entries map[string][]byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "archive.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}

	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return path
}

// writeTarFile builds a tar (optionally gzipped) archive, writes it to
// a temp file, and returns the path.
func writeTarFile(t *testing.T, gzipped bool, entries map[string][]byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "archive.tar")
	if gzipped {
		path += ".gz"
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar file: %v", err)
	}

	var w io.Writer = f
	var gw *gzip.Writer
	if gzipped {
		gw = gzip.NewWriter(f)
		w = gw
	}

	tw := tar.NewWriter(w)
	for name, data := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0755,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header %q: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write tar data %q: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if gw != nil {
		if err := gw.Close(); err != nil {
			t.Fatalf("close gzip writer: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close tar file: %v", err)
	}
	return path
}

func TestVerifyHashSuccess(t *testing.T) {
	content := []byte("the quick brown fox jumps over the lazy dog\n")
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sum := sha256Hex(content)

	inst := &Installer{}
	if err := inst.verifyHash(path, sum); err != nil {
		t.Errorf("verifyHash(lowercase) unexpected error: %v", err)
	}
	if err := inst.verifyHash(path, strings.ToUpper(sum)); err != nil {
		t.Errorf("verifyHash(uppercase) unexpected error: %v", err)
	}
}

func TestVerifyHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	inst := &Installer{}
	err := inst.verifyHash(path, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("expected error for mismatched hash, got nil")
	}
	if !errors.Is(err, ErrHashMismatch) {
		t.Errorf("expected ErrHashMismatch, got %v", err)
	}
}

func TestVerifyHashMissingFile(t *testing.T) {
	inst := &Installer{}
	err := inst.verifyHash(filepath.Join(t.TempDir(), "nope.bin"), strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if errors.Is(err, ErrHashMismatch) {
		t.Errorf("missing file should not be ErrHashMismatch, got %v", err)
	}
}

func TestExtractZipHappyPath(t *testing.T) {
	content := []byte("ELF stockfish binary")
	zipPath := writeZipFile(t, map[string][]byte{
		"bin/stockfish": content,
	})

	destDir := t.TempDir()
	inst := &Installer{}
	got, err := inst.extractZip(zipPath, destDir, "stockfish")
	if err != nil {
		t.Fatalf("extractZip() error: %v", err)
	}

	want := filepath.Join(destDir, "bin", "stockfish")
	if got != want {
		t.Errorf("extractZip() path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("binary content = %q, want %q", string(data), string(content))
	}
}

func TestExtractZipDirectoryEntry(t *testing.T) {
	zipPath := writeZipFile(t, map[string][]byte{
		"bin/":          {},
		"bin/stockfish": []byte("content"),
	})

	destDir := t.TempDir()
	inst := &Installer{}
	got, err := inst.extractZip(zipPath, destDir, "stockfish")
	if err != nil {
		t.Fatalf("extractZip() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "bin")); err != nil {
		t.Errorf("directory entry not created: %v", err)
	}
	if got != filepath.Join(destDir, "bin", "stockfish") {
		t.Errorf("extractZip() path = %q, want %q", got, filepath.Join(destDir, "bin", "stockfish"))
	}
}

func TestExtractZipBinaryNotFound(t *testing.T) {
	zipPath := writeZipFile(t, map[string][]byte{
		"readme.txt": []byte("hello"),
	})

	destDir := t.TempDir()
	inst := &Installer{}
	got, err := inst.extractZip(zipPath, destDir, "stockfish")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "binary stockfish not found in archive") {
		t.Errorf("error = %q, want containing %q", err.Error(), "binary stockfish not found in archive")
	}
	if got != "" {
		t.Errorf("path = %q, want empty string", got)
	}
}

func TestExtractZipBlocksPathTraversal(t *testing.T) {
	base := t.TempDir()
	destDir := filepath.Join(base, "extract")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("mkdir destDir: %v", err)
	}
	escapedPath := filepath.Join(base, "escape.txt")

	zipPath := writeZipFile(t, map[string][]byte{
		"bin/stockfish": []byte("legit"),
		"../escape.txt": []byte("evil"),
	})

	inst := &Installer{}
	got, err := inst.extractZip(zipPath, destDir, "stockfish")
	if err != nil {
		t.Fatalf("extractZip() error: %v", err)
	}

	if got != filepath.Join(destDir, "bin", "stockfish") {
		t.Errorf("binary path = %q, want %q", got, filepath.Join(destDir, "bin", "stockfish"))
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("legit entry not extracted: %v", err)
	}
	if _, err := os.Stat(escapedPath); !os.IsNotExist(err) {
		t.Errorf("escape file should not exist, err = %v", err)
	}
}

func TestExtractTarGzippedHappyPath(t *testing.T) {
	content := []byte("gzipped tar binary")
	tarPath := writeTarFile(t, true, map[string][]byte{
		"bin/stockfish": content,
	})

	destDir := t.TempDir()
	inst := &Installer{}
	got, err := inst.extractTar(tarPath, destDir, "stockfish", true)
	if err != nil {
		t.Fatalf("extractTar() error: %v", err)
	}

	want := filepath.Join(destDir, "bin", "stockfish")
	if got != want {
		t.Errorf("extractTar() path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read extracted tar binary: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("tar binary content = %q, want %q", string(data), string(content))
	}
}

func TestExtractTarPlainHappyPath(t *testing.T) {
	content := []byte("plain tar binary")
	tarPath := writeTarFile(t, false, map[string][]byte{
		"bin/stockfish": content,
	})

	destDir := t.TempDir()
	inst := &Installer{}
	got, err := inst.extractTar(tarPath, destDir, "stockfish", false)
	if err != nil {
		t.Fatalf("extractTar() error: %v", err)
	}

	want := filepath.Join(destDir, "bin", "stockfish")
	if got != want {
		t.Errorf("extractTar() path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read extracted tar binary: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("tar binary content = %q, want %q", string(data), string(content))
	}
}

func TestExtractTarBinaryNotFound(t *testing.T) {
	tarPath := writeTarFile(t, false, map[string][]byte{
		"readme.txt": []byte("hello"),
	})

	destDir := t.TempDir()
	inst := &Installer{}
	got, err := inst.extractTar(tarPath, destDir, "stockfish", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "binary stockfish not found in archive") {
		t.Errorf("error = %q, want containing %q", err.Error(), "binary stockfish not found in archive")
	}
	if got != "" {
		t.Errorf("path = %q, want empty string", got)
	}
}

func TestExtractDispatcher(t *testing.T) {
	inst := &Installer{}

	t.Run("zip", func(t *testing.T) {
		zipPath := writeZipFile(t, map[string][]byte{
			"bin/stockfish": []byte("zip content"),
		})
		destDir := t.TempDir()
		got, err := inst.extract(zipPath, destDir, "stockfish", "zip")
		if err != nil {
			t.Fatalf("extract(zip) error: %v", err)
		}
		if got != filepath.Join(destDir, "bin", "stockfish") {
			t.Errorf("path = %q, want %q", got, filepath.Join(destDir, "bin", "stockfish"))
		}
	})

	t.Run("tar", func(t *testing.T) {
		tarPath := writeTarFile(t, false, map[string][]byte{
			"bin/stockfish": []byte("tar content"),
		})
		destDir := t.TempDir()
		got, err := inst.extract(tarPath, destDir, "stockfish", "tar")
		if err != nil {
			t.Fatalf("extract(tar) error: %v", err)
		}
		if got != filepath.Join(destDir, "bin", "stockfish") {
			t.Errorf("path = %q, want %q", got, filepath.Join(destDir, "bin", "stockfish"))
		}
	})

	t.Run("tar.gz", func(t *testing.T) {
		tarPath := writeTarFile(t, true, map[string][]byte{
			"bin/stockfish": []byte("tar.gz content"),
		})
		destDir := t.TempDir()
		got, err := inst.extract(tarPath, destDir, "stockfish", "tar.gz")
		if err != nil {
			t.Fatalf("extract(tar.gz) error: %v", err)
		}
		if got != filepath.Join(destDir, "bin", "stockfish") {
			t.Errorf("path = %q, want %q", got, filepath.Join(destDir, "bin", "stockfish"))
		}
	})

	t.Run("tgz", func(t *testing.T) {
		tarPath := writeTarFile(t, true, map[string][]byte{
			"bin/stockfish": []byte("tgz content"),
		})
		destDir := t.TempDir()
		got, err := inst.extract(tarPath, destDir, "stockfish", "tgz")
		if err != nil {
			t.Fatalf("extract(tgz) error: %v", err)
		}
		if got != filepath.Join(destDir, "bin", "stockfish") {
			t.Errorf("path = %q, want %q", got, filepath.Join(destDir, "bin", "stockfish"))
		}
	})

	t.Run("raw binary", func(t *testing.T) {
		content := []byte("raw binary content")
		src := filepath.Join(t.TempDir(), "raw.bin")
		if err := os.WriteFile(src, content, 0644); err != nil {
			t.Fatalf("write raw binary: %v", err)
		}
		destDir := t.TempDir()

		got, err := inst.extract(src, destDir, "stockfish", "")
		if err != nil {
			t.Fatalf("extract(raw) error: %v", err)
		}
		want := filepath.Join(destDir, "stockfish")
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if _, err := os.Stat(src); !os.IsNotExist(err) {
			t.Errorf("source file should be moved (not exist), stat err = %v", err)
		}
		data, err := os.ReadFile(want)
		if err != nil {
			t.Fatalf("read moved binary: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("moved binary content = %q, want %q", string(data), string(content))
		}
	})

	t.Run("unknown format", func(t *testing.T) {
		destDir := t.TempDir()
		_, err := inst.extract("whatever", destDir, "stockfish", "rar")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrInvalidArchive) {
			t.Errorf("expected ErrInvalidArchive, got %v", err)
		}
	})
}
