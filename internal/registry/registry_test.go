package registry

import (
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	tomlData := []byte(`
[meta]
version = "1.0.0"
updated = "2024-01-15"

[engines.stockfish-17]
name = "Stockfish 17"
version = "17"
author = "Stockfish Team"
license = "GPL-3.0"
description = "The strongest open-source chess engine"
elo_estimate = 3600

[engines.stockfish-17.builds.linux-amd64-avx2]
url = "https://example.com/sf-linux-avx2.tar"
sha256 = "abc123"
binary = "stockfish"
extract = "tar"

[engines.stockfish-17.builds.linux-amd64]
url = "https://example.com/sf-linux.tar"
sha256 = "def456"
binary = "stockfish"
extract = "tar"

[engines.stockfish-17.builds.windows-amd64-avx2]
url = "https://example.com/sf-win-avx2.zip"
sha256 = "ghi789"
binary = "stockfish.exe"
extract = "zip"

[engines.stockfish-17.options.Hash]
type = "spin"
default = 16
min = 1
max = 33554432
description = "Hash table size in MB"

[engines.stockfish-17.profiles.analysis]
Hash = 1024
Threads = 4
`)

	cpuFeatures := CPUFeatures{AVX2: true, POPCNT: true, SSE42: true}
	mgr := NewManager("", cpuFeatures)

	err := mgr.LoadFromEmbed(tomlData)
	if err != nil {
		t.Fatalf("LoadFromEmbed() error: %v", err)
	}

	// Test ListEngines
	engines := mgr.ListEngines()
	if len(engines) != 1 {
		t.Errorf("ListEngines() = %d engines, want 1", len(engines))
	}

	// Test GetEngine
	engine, err := mgr.GetEngine("stockfish-17")
	if err != nil {
		t.Errorf("GetEngine() error: %v", err)
	}
	if engine.Name != "Stockfish 17" {
		t.Errorf("engine.Name = %q, want %q", engine.Name, "Stockfish 17")
	}
	if engine.ELOEstimate != 3600 {
		t.Errorf("engine.ELOEstimate = %d, want 3600", engine.ELOEstimate)
	}

	// Test GetEngine not found
	_, err = mgr.GetEngine("nonexistent")
	if err != ErrEngineNotFound {
		t.Errorf("GetEngine(nonexistent) error = %v, want ErrEngineNotFound", err)
	}
}

func TestSelectBuild(t *testing.T) {
	tomlData := []byte(`
[meta]
version = "1.0.0"

[engines.test-engine]
name = "Test Engine"

[engines.test-engine.builds.linux-amd64-avx512]
url = "https://example.com/avx512.tar"
sha256 = "avx512hash"
binary = "engine"

[engines.test-engine.builds.linux-amd64-avx2]
url = "https://example.com/avx2.tar"
sha256 = "avx2hash"
binary = "engine"

[engines.test-engine.builds.linux-amd64]
url = "https://example.com/base.tar"
sha256 = "basehash"
binary = "engine"
`)

	tests := []struct {
		name        string
		cpuFeatures CPUFeatures
		wantBuildKey string
	}{
		{
			name:        "avx512 cpu gets avx512 build",
			cpuFeatures: CPUFeatures{AVX512: true, AVX2: true, POPCNT: true},
			wantBuildKey: "linux-amd64-avx512",
		},
		{
			name:        "avx2 cpu gets avx2 build",
			cpuFeatures: CPUFeatures{AVX2: true, POPCNT: true},
			wantBuildKey: "linux-amd64-avx2",
		},
		{
			name:        "basic cpu gets base build",
			cpuFeatures: CPUFeatures{POPCNT: true},
			wantBuildKey: "linux-amd64",
		},
		{
			name:        "no features gets base build",
			cpuFeatures: CPUFeatures{},
			wantBuildKey: "linux-amd64",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewManager("", tc.cpuFeatures)
			if err := mgr.LoadFromEmbed(tomlData); err != nil {
				t.Fatalf("LoadFromEmbed() error: %v", err)
			}

			engine, _ := mgr.GetEngine("test-engine")
			_, buildKey, err := mgr.SelectBuild(engine)
			if err != nil {
				t.Fatalf("SelectBuild() error: %v", err)
			}
			if buildKey != tc.wantBuildKey {
				t.Errorf("SelectBuild() buildKey = %q, want %q", buildKey, tc.wantBuildKey)
			}
		})
	}
}

func TestSelectBuildNoneAvailable(t *testing.T) {
	tomlData := []byte(`
[meta]
version = "1.0.0"

[engines.mac-only]
name = "Mac Only Engine"

[engines.mac-only.builds.darwin-arm64]
url = "https://example.com/mac.tar"
sha256 = "machash"
binary = "engine"
`)

	mgr := NewManager("", CPUFeatures{AVX2: true})
	if err := mgr.LoadFromEmbed(tomlData); err != nil {
		t.Fatalf("LoadFromEmbed() error: %v", err)
	}

	engine, _ := mgr.GetEngine("mac-only")
	_, _, err := mgr.SelectBuild(engine)
	if err != ErrNoBuildAvailable {
		t.Errorf("SelectBuild() error = %v, want ErrNoBuildAvailable", err)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr bool
	}{
		{
			name: "valid registry",
			toml: `
[meta]
version = "1.0.0"
[engines.test]
name = "Test"
[engines.test.builds.linux-amd64]
url = "https://example.com/test.tar"
sha256 = "hash"
`,
			wantErr: false,
		},
		{
			name: "missing meta version",
			toml: `
[meta]
[engines.test]
name = "Test"
[engines.test.builds.linux-amd64]
url = "https://example.com/test.tar"
sha256 = "hash"
`,
			wantErr: true,
		},
		{
			name: "missing engine name",
			toml: `
[meta]
version = "1.0.0"
[engines.test]
[engines.test.builds.linux-amd64]
url = "https://example.com/test.tar"
sha256 = "hash"
`,
			wantErr: true,
		},
		{
			name: "missing builds",
			toml: `
[meta]
version = "1.0.0"
[engines.test]
name = "Test"
`,
			wantErr: true,
		},
		{
			name: "missing build url",
			toml: `
[meta]
version = "1.0.0"
[engines.test]
name = "Test"
[engines.test.builds.linux-amd64]
sha256 = "hash"
`,
			wantErr: true,
		},
		{
			name: "missing build sha256",
			toml: `
[meta]
version = "1.0.0"
[engines.test]
name = "Test"
[engines.test.builds.linux-amd64]
url = "https://example.com/test.tar"
`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewManager("", CPUFeatures{})
			err := mgr.LoadFromEmbed([]byte(tc.toml))
			if tc.wantErr && err == nil {
				t.Error("LoadFromEmbed() expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("LoadFromEmbed() unexpected error: %v", err)
			}
		})
	}
}

func TestCPUFeatures(t *testing.T) {
	tests := []struct {
		features    CPUFeatures
		wantBest    string
		wantString  string
	}{
		{
			features:   CPUFeatures{AVX512: true, AVX2: true, POPCNT: true},
			wantBest:   "avx512",
			wantString: "AVX512, AVX2, POPCNT",
		},
		{
			features:   CPUFeatures{AVX2: true, BMI2: true, POPCNT: true, SSE42: true},
			wantBest:   "bmi2",
			wantString: "AVX2, BMI2, POPCNT, SSE42",
		},
		{
			features:   CPUFeatures{POPCNT: true},
			wantBest:   "popcnt",
			wantString: "POPCNT",
		},
		{
			features:   CPUFeatures{},
			wantBest:   "",
			wantString: "none",
		},
	}

	for _, tc := range tests {
		t.Run(tc.wantString, func(t *testing.T) {
			if got := tc.features.BestFeature(); got != tc.wantBest {
				t.Errorf("BestFeature() = %q, want %q", got, tc.wantBest)
			}
			if got := tc.features.FeatureString(); got != tc.wantString {
				t.Errorf("FeatureString() = %q, want %q", got, tc.wantString)
			}
		})
	}
}

func TestParseBuildKey(t *testing.T) {
	tests := []struct {
		key         string
		wantOS      string
		wantArch    string
		wantFeature string
	}{
		{"linux-amd64-avx2", "linux", "amd64", "avx2"},
		{"darwin-arm64", "darwin", "arm64", ""},
		{"windows-amd64-bmi2", "windows", "amd64", "bmi2"},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			os, arch, feature := ParseBuildKey(tc.key)
			if os != tc.wantOS {
				t.Errorf("ParseBuildKey() os = %q, want %q", os, tc.wantOS)
			}
			if arch != tc.wantArch {
				t.Errorf("ParseBuildKey() arch = %q, want %q", arch, tc.wantArch)
			}
			if feature != tc.wantFeature {
				t.Errorf("ParseBuildKey() feature = %q, want %q", feature, tc.wantFeature)
			}
		})
	}
}

func TestListEngineInfo(t *testing.T) {
	tomlData := []byte(`
[meta]
version = "1.0.0"

[engines.has-build]
name = "Has Build"
description = "Engine with compatible build"
elo_estimate = 3000

[engines.has-build.builds.linux-amd64]
url = "https://example.com/engine.tar"
sha256 = "hash"

[engines.no-build]
name = "No Build"
description = "Engine without compatible build"

[engines.no-build.builds.darwin-arm64]
url = "https://example.com/mac.tar"
sha256 = "hash"
`)

	mgr := NewManager("", CPUFeatures{})
	if err := mgr.LoadFromEmbed(tomlData); err != nil {
		t.Fatalf("LoadFromEmbed() error: %v", err)
	}

	infos := mgr.ListEngineInfo()
	if len(infos) != 2 {
		t.Fatalf("ListEngineInfo() = %d engines, want 2", len(infos))
	}

	// Find each engine
	var hasBuild, noBuild *EngineInfo
	for i := range infos {
		if infos[i].ID == "has-build" {
			hasBuild = &infos[i]
		}
		if infos[i].ID == "no-build" {
			noBuild = &infos[i]
		}
	}

	if hasBuild == nil || !hasBuild.HasBuild {
		t.Error("has-build engine should have HasBuild=true")
	}
	if noBuild == nil || noBuild.HasBuild {
		t.Error("no-build engine should have HasBuild=false")
	}
}
