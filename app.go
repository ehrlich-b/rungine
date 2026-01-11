package main

import (
	"context"
	_ "embed"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"rungine/internal/registry"
	"rungine/internal/uci"
)

//go:embed registry/engines.toml
var embeddedRegistry []byte

// App struct holds application state and provides Wails bindings.
type App struct {
	ctx       context.Context
	engines   *uci.EngineManager
	registry  *registry.Manager
	installer *registry.Installer
}

// NewApp creates a new App application struct.
func NewApp() *App {
	cpuFeatures := registry.DetectCPUFeatures()
	slog.Info("detected CPU features", "features", cpuFeatures.FeatureString())

	regMgr := registry.NewManager("", cpuFeatures)
	if err := regMgr.LoadFromEmbed(embeddedRegistry); err != nil {
		slog.Warn("failed to load embedded registry", "err", err)
	}

	installer, err := registry.NewInstaller(regMgr)
	if err != nil {
		slog.Warn("failed to create installer", "err", err)
	}

	return &App{
		engines:   uci.NewEngineManager(),
		registry:  regMgr,
		installer: installer,
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Wire up analysis events to frontend
	a.engines.SetAnalysisCallback(func(info uci.AnalysisInfo) {
		runtime.EventsEmit(ctx, "analysis:info", info)
	})

	// Wire up installer events to frontend
	if a.installer != nil {
		a.installer.SetDownloadProgressCallback(func(p registry.DownloadProgress) {
			runtime.EventsEmit(ctx, "download:progress", p)
		})
		a.installer.SetInstallProgressCallback(func(p registry.InstallProgress) {
			runtime.EventsEmit(ctx, "install:progress", p)
		})
	}

	// Auto-register installed engines
	a.loadInstalledEngines()
}

// loadInstalledEngines registers engines that were previously installed.
func (a *App) loadInstalledEngines() {
	if a.installer == nil {
		return
	}

	installed, err := a.installer.ListInstalled()
	if err != nil {
		slog.Warn("failed to list installed engines", "err", err)
		return
	}

	for _, eng := range installed {
		if err := a.engines.RegisterEngine(eng.ID, eng.BinaryPath); err != nil {
			slog.Warn("failed to register installed engine", "id", eng.ID, "err", err)
		}
	}
}

// shutdown is called when the app is closing.
func (a *App) shutdown(_ context.Context) {
	a.engines.Shutdown()
}

// RegisterEngine registers a new engine with the manager.
func (a *App) RegisterEngine(id, binaryPath string) error {
	return a.engines.RegisterEngine(id, binaryPath)
}

// UnregisterEngine removes an engine from the manager.
func (a *App) UnregisterEngine(id string) error {
	return a.engines.UnregisterEngine(id)
}

// ListEngines returns info about all registered engines.
func (a *App) ListEngines() []uci.EngineInfo {
	return a.engines.ListEngines()
}

// StartEngine starts an engine process and initializes UCI.
func (a *App) StartEngine(id string) error {
	return a.engines.StartEngine(id)
}

// StopEngine stops an engine process.
func (a *App) StopEngine(id string) error {
	return a.engines.StopEngine(id)
}

// GetEngineOptions returns the UCI options for an engine.
func (a *App) GetEngineOptions(id string) (map[string]uci.UCIOption, error) {
	engine, err := a.engines.GetEngine(id)
	if err != nil {
		return nil, err
	}
	return engine.Options(), nil
}

// SetEngineOption sets a UCI option on an engine.
func (a *App) SetEngineOption(id, name, value string) error {
	engine, err := a.engines.GetEngine(id)
	if err != nil {
		return err
	}
	return engine.SetOption(name, value)
}

// AnalysisParams holds parameters for starting analysis.
type AnalysisParams struct {
	FEN       string   `json:"fen"`
	Moves     []string `json:"moves"`
	EngineIDs []string `json:"engineIds"`
	Infinite  bool     `json:"infinite"`
	Depth     int      `json:"depth"`
	MoveTime  int      `json:"moveTime"` // milliseconds
}

// StartAnalysis begins analysis on the specified engines.
func (a *App) StartAnalysis(params AnalysisParams) error {
	goParams := uci.GoParams{
		Infinite: params.Infinite,
		Depth:    params.Depth,
	}
	if params.MoveTime > 0 {
		goParams.MoveTime = time.Duration(params.MoveTime) * time.Millisecond
	}
	// Default to infinite if nothing specified
	if !params.Infinite && params.Depth == 0 && params.MoveTime == 0 {
		goParams.Infinite = true
	}
	return a.engines.StartAnalysis(params.FEN, params.Moves, params.EngineIDs, goParams)
}

// StopAnalysis stops analysis on the specified engines.
func (a *App) StopAnalysis(engineIDs []string) error {
	return a.engines.StopAnalysis(engineIDs)
}

// SetAnalysisThrottle sets the UI update rate in Hz.
func (a *App) SetAnalysisThrottle(hz int) {
	a.engines.SetThrottleRate(hz)
}

// ListAvailableEngines returns engines available for installation from the registry.
func (a *App) ListAvailableEngines() []registry.EngineInfo {
	return a.registry.ListEngineInfo()
}

// ListInstalledEngines returns engines that have been installed locally.
func (a *App) ListInstalledEngines() ([]registry.InstalledEngine, error) {
	if a.installer == nil {
		return nil, nil
	}
	return a.installer.ListInstalled()
}

// InstallEngine downloads and installs an engine from the registry.
func (a *App) InstallEngine(engineID string) error {
	if a.installer == nil {
		return nil
	}

	installed, err := a.installer.Install(a.ctx, engineID)
	if err != nil {
		return err
	}

	// Auto-register the newly installed engine
	return a.engines.RegisterEngine(installed.ID, installed.BinaryPath)
}

// UninstallEngine removes an installed engine.
func (a *App) UninstallEngine(engineID string) error {
	if a.installer == nil {
		return nil
	}

	// Stop and unregister first
	a.engines.StopEngine(engineID)
	a.engines.UnregisterEngine(engineID)

	return a.installer.Uninstall(engineID)
}

// GetCPUFeatures returns the detected CPU features.
func (a *App) GetCPUFeatures() string {
	return registry.DetectCPUFeatures().FeatureString()
}
