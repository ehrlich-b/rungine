package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileValueToString(t *testing.T) {
	cases := []struct {
		name     string
		v        any
		cpuCount int
		want     string
	}{
		{"auto string resolves to cpuCount", "auto", 8, "8"},
		{"plain string unchanged", "hash128", 8, "hash128"},
		{"bool true", true, 8, "true"},
		{"bool false", false, 8, "false"},
		{"int", 16, 8, "16"},
		{"int64", int64(32), 8, "32"},
		{"integer-valued float64", 1024.0, 8, "1024"},
		{"negative integer-valued float64", -2.0, 8, "-2"},
		{"non-integer float64", 3.5, 8, "3.5"},
		{"nil fallback", nil, 8, "<nil>"},
		{"slice fallback", []int{1, 2}, 8, "[1 2]"},
	}
	for _, tc := range cases {
		if got := profileValueToString(tc.v, tc.cpuCount); got != tc.want {
			t.Errorf("profileValueToString(%v, %d) = %q, want %q", tc.v, tc.cpuCount, got, tc.want)
		}
	}
}

func TestUniqueCustomIDNoCollision(t *testing.T) {
	i := &Installer{installDir: t.TempDir()}
	if got := i.uniqueCustomID("My Engine"); got != "custom-my-engine" {
		t.Errorf("uniqueCustomID(%q) = %q, want %q", "My Engine", got, "custom-my-engine")
	}
}

func TestUniqueCustomIDCollisionsIncrement(t *testing.T) {
	dir := t.TempDir()
	i := &Installer{installDir: dir}

	if err := os.MkdirAll(filepath.Join(dir, "custom-my-engine"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := i.uniqueCustomID("My Engine"); got != "custom-my-engine-2" {
		t.Errorf("uniqueCustomID after one collision = %q, want %q", got, "custom-my-engine-2")
	}

	if err := os.MkdirAll(filepath.Join(dir, "custom-my-engine-2"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := i.uniqueCustomID("My Engine"); got != "custom-my-engine-3" {
		t.Errorf("uniqueCustomID after two collisions = %q, want %q", got, "custom-my-engine-3")
	}
}

func TestUniqueCustomIDEmptySlugFallback(t *testing.T) {
	// "!!!" slugifies to the empty string, which uniqueCustomID replaces with
	// the literal word "custom" before the "custom-" prefix is applied, so the
	// result is "custom-custom", not an empty or malformed id.
	dir := t.TempDir()
	i := &Installer{installDir: dir}
	if got := i.uniqueCustomID("!!!"); got != "custom-custom" {
		t.Errorf("uniqueCustomID(%q) = %q, want %q", "!!!", got, "custom-custom")
	}
}

func TestUniqueCustomIDDoesNotCreateDir(t *testing.T) {
	dir := t.TempDir()
	i := &Installer{installDir: dir}
	id := i.uniqueCustomID("My Engine")
	path := filepath.Join(dir, id)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%q) = %v, want IsNotExist (uniqueCustomID must not create the directory)", path, err)
	}
}
