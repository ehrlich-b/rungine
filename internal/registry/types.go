package registry

// Registry represents the complete engine registry.
type Registry struct {
	Meta    Meta                        `toml:"meta"`
	Engines map[string]EngineDefinition `toml:"engines"`
}

// Meta contains registry metadata.
type Meta struct {
	Version string `toml:"version"`
	Updated string `toml:"updated"`
}

// EngineDefinition defines a chess engine available for installation.
type EngineDefinition struct {
	Name            string               `toml:"name"`
	Version         string               `toml:"version"`
	Author          string               `toml:"author"`
	License         string               `toml:"license"`
	Homepage        string               `toml:"homepage"`
	Description     string               `toml:"description"`
	ELOEstimate     int                  `toml:"elo_estimate"`
	RequiresNetwork bool                 `toml:"requires_network"`
	Builds          map[string]Build     `toml:"builds"`
	Networks        map[string]Network   `toml:"networks"`
	Options         map[string]OptionDef `toml:"options"`
	Profiles        map[string]Profile   `toml:"profiles"`
}

// Network defines a neural network file for NNUE/NN engines.
type Network struct {
	URL         string `toml:"url"`
	SHA256      string `toml:"sha256"`
	Size        string `toml:"size"`        // Human-readable size (e.g., "330 MB")
	Description string `toml:"description"`
	GPUMemory   string `toml:"gpu_memory"`  // Recommended GPU memory (e.g., "4 GB")
	Default     bool   `toml:"default"`     // Whether this is the default network
}

// Build defines a platform-specific engine binary.
// Key format: {os}-{arch}-{cpu_feature} e.g., "linux-amd64-avx2"
type Build struct {
	URL     string `toml:"url"`
	SHA256  string `toml:"sha256"`
	Binary  string `toml:"binary"`  // Path within archive to the binary
	Extract string `toml:"extract"` // "zip", "tar", "tar.gz", or empty for raw binary
}

// OptionDef documents a UCI option with recommended values.
type OptionDef struct {
	Type        string `toml:"type"` // "spin", "check", "combo", "string", "button"
	Default     any    `toml:"default"`
	Min         *int   `toml:"min"`
	Max         *int   `toml:"max"`
	Description string `toml:"description"`
	Recommended any    `toml:"recommended"` // Can be int, string, or "auto"
}

// Profile is a named configuration preset.
type Profile map[string]any

// CPUFeatures represents detected CPU capabilities.
type CPUFeatures struct {
	AVX512 bool
	AVX2   bool
	BMI2   bool
	POPCNT bool
	SSE42  bool
}

// InstalledEngine represents a locally installed engine.
type InstalledEngine struct {
	ID           string            `toml:"id"`
	RegistryID   string            `toml:"registry_id"`
	Name         string            `toml:"name"`
	Version      string            `toml:"version"`
	BinaryPath   string            `toml:"binary_path"`
	NetworkPath  string            `toml:"network_path"` // Path to installed neural network (if any)
	InstalledAt  string            `toml:"installed_at"`
	BuildKey     string            `toml:"build_key"`
	NetworkKey   string            `toml:"network_key"` // Which network was installed
	OptionValues map[string]string `toml:"options"`
}
