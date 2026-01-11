package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

var (
	ErrEngineNotFound   = errors.New("engine not found in registry")
	ErrNoBuildAvailable = errors.New("no compatible build available for this platform")
	ErrInvalidRegistry  = errors.New("invalid registry format")
)

// Manager handles engine registry operations.
type Manager struct {
	registry     *Registry
	registryPath string
	cpuFeatures  CPUFeatures
}

// NewManager creates a new registry manager.
func NewManager(registryPath string, cpuFeatures CPUFeatures) *Manager {
	return &Manager{
		registryPath: registryPath,
		cpuFeatures:  cpuFeatures,
	}
}

// Load reads and parses the registry file.
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.registryPath)
	if err != nil {
		return fmt.Errorf("read registry: %w", err)
	}

	var reg Registry
	if err := toml.Unmarshal(data, &reg); err != nil {
		return fmt.Errorf("parse registry: %w", err)
	}

	if err := m.validate(&reg); err != nil {
		return fmt.Errorf("validate registry: %w", err)
	}

	m.registry = &reg
	return nil
}

// validate checks the registry for required fields and consistency.
func (m *Manager) validate(reg *Registry) error {
	if reg.Meta.Version == "" {
		return fmt.Errorf("%w: missing meta.version", ErrInvalidRegistry)
	}

	for id, engine := range reg.Engines {
		if engine.Name == "" {
			return fmt.Errorf("%w: engine %s missing name", ErrInvalidRegistry, id)
		}
		if len(engine.Builds) == 0 {
			return fmt.Errorf("%w: engine %s has no builds", ErrInvalidRegistry, id)
		}

		for buildKey, build := range engine.Builds {
			if build.URL == "" {
				return fmt.Errorf("%w: engine %s build %s missing URL", ErrInvalidRegistry, id, buildKey)
			}
			if build.SHA256 == "" {
				return fmt.Errorf("%w: engine %s build %s missing SHA256", ErrInvalidRegistry, id, buildKey)
			}
		}
	}

	return nil
}

// ListEngines returns all available engines.
func (m *Manager) ListEngines() []EngineDefinition {
	if m.registry == nil {
		return nil
	}

	engines := make([]EngineDefinition, 0, len(m.registry.Engines))
	for _, e := range m.registry.Engines {
		engines = append(engines, e)
	}
	return engines
}

// GetEngine returns an engine definition by ID.
func (m *Manager) GetEngine(id string) (*EngineDefinition, error) {
	if m.registry == nil {
		return nil, errors.New("registry not loaded")
	}

	engine, ok := m.registry.Engines[id]
	if !ok {
		return nil, ErrEngineNotFound
	}
	return &engine, nil
}

// SelectBuild chooses the optimal build for the current platform and CPU.
func (m *Manager) SelectBuild(engine *EngineDefinition) (*Build, string, error) {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Priority order: most optimized first
	featureSuffixes := m.buildCandidates()

	for _, suffix := range featureSuffixes {
		key := fmt.Sprintf("%s-%s%s", os, arch, suffix)
		if build, ok := engine.Builds[key]; ok {
			return &build, key, nil
		}
	}

	// Try without feature suffix
	key := fmt.Sprintf("%s-%s", os, arch)
	if build, ok := engine.Builds[key]; ok {
		return &build, key, nil
	}

	return nil, "", ErrNoBuildAvailable
}

// buildCandidates returns feature suffixes in priority order based on CPU capabilities.
func (m *Manager) buildCandidates() []string {
	var candidates []string

	// Add suffixes for features this CPU supports, highest priority first
	if m.cpuFeatures.AVX512 {
		candidates = append(candidates, "-avx512")
	}
	if m.cpuFeatures.BMI2 {
		candidates = append(candidates, "-bmi2")
	}
	if m.cpuFeatures.AVX2 {
		candidates = append(candidates, "-avx2")
	}
	if m.cpuFeatures.POPCNT {
		candidates = append(candidates, "-popcnt")
	}
	if m.cpuFeatures.SSE42 {
		candidates = append(candidates, "-sse42")
	}

	// Always try base version last
	candidates = append(candidates, "")

	return candidates
}

// GetBuildURL returns the download URL for an engine on the current platform.
func (m *Manager) GetBuildURL(engineID string) (string, string, error) {
	engine, err := m.GetEngine(engineID)
	if err != nil {
		return "", "", err
	}

	build, buildKey, err := m.SelectBuild(engine)
	if err != nil {
		return "", "", err
	}

	return build.URL, buildKey, nil
}

// EngineInfo returns summary info for display.
type EngineInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Author          string `json:"author"`
	Description     string `json:"description"`
	ELOEstimate     int    `json:"eloEstimate"`
	RequiresNetwork bool   `json:"requiresNetwork"`
	HasBuild        bool   `json:"hasBuild"` // Whether a compatible build exists
}

// ListEngineInfo returns display-friendly info for all engines.
func (m *Manager) ListEngineInfo() []EngineInfo {
	if m.registry == nil {
		return nil
	}

	infos := make([]EngineInfo, 0, len(m.registry.Engines))
	for id, e := range m.registry.Engines {
		_, _, err := m.SelectBuild(&e)
		infos = append(infos, EngineInfo{
			ID:              id,
			Name:            e.Name,
			Version:         e.Version,
			Author:          e.Author,
			Description:     e.Description,
			ELOEstimate:     e.ELOEstimate,
			RequiresNetwork: e.RequiresNetwork,
			HasBuild:        err == nil,
		})
	}
	return infos
}

// LoadFromEmbed loads registry from embedded data.
func (m *Manager) LoadFromEmbed(data []byte) error {
	var reg Registry
	if err := toml.Unmarshal(data, &reg); err != nil {
		return fmt.Errorf("parse registry: %w", err)
	}

	if err := m.validate(&reg); err != nil {
		return fmt.Errorf("validate registry: %w", err)
	}

	m.registry = &reg
	return nil
}

// InstallDir returns the installation directory for engines.
func InstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".rungine", "engines"), nil
}

// ConfigDir returns the config directory.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".rungine"), nil
}

// ParseBuildKey extracts OS, arch, and feature from a build key.
func ParseBuildKey(key string) (os, arch, feature string) {
	parts := strings.Split(key, "-")
	if len(parts) >= 2 {
		os = parts[0]
		arch = parts[1]
	}
	if len(parts) >= 3 {
		feature = parts[2]
	}
	return
}
