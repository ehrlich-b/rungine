package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"rungine/internal/database"
	"rungine/internal/registry"
	"rungine/internal/uci"
)

//go:embed registry/engines.toml
var embeddedRegistry []byte

// App struct holds application state and provides Wails bindings.
type App struct {
	ctx         context.Context
	engines     *uci.EngineManager
	registry    *registry.Manager
	installer   *registry.Installer
	db          *database.DB
	tournaments *TournamentManager
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

	db, err := database.Open("")
	if err != nil {
		slog.Warn("failed to open database", "err", err)
	}

	return &App{
		engines:     uci.NewEngineManager(),
		registry:    regMgr,
		installer:   installer,
		db:          db,
		tournaments: newTournamentManager(installer, db),
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

	if a.tournaments != nil {
		a.tournaments.bindContext(ctx)
		if err := a.tournaments.hydrateFromDB(ctx); err != nil {
			slog.Warn("failed to hydrate tournaments from db", "err", err)
		}
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
	if a.db != nil {
		_ = a.db.Close()
	}
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

// PickEngineBinary opens an OS file dialog so the user can select a
// UCI engine binary. Returns the absolute path, or an empty string if
// the user cancelled.
func (a *App) PickEngineBinary() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("no app context yet")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select UCI engine binary",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// AddCustomEngine registers an external UCI binary as an installed
// engine. binaryPath must point to an executable that responds to the
// uci handshake. displayName is shown in the UI; if empty, the binary
// filename (sans extension) is used.
func (a *App) AddCustomEngine(binaryPath, displayName string) (registry.InstalledEngine, error) {
	if a.installer == nil {
		return registry.InstalledEngine{}, fmt.Errorf("installer not available")
	}
	if a.ctx == nil {
		return registry.InstalledEngine{}, fmt.Errorf("no app context yet")
	}
	installed, err := a.installer.RegisterExternal(a.ctx, binaryPath, displayName)
	if err != nil {
		return registry.InstalledEngine{}, err
	}
	if err := a.engines.RegisterEngine(installed.ID, installed.BinaryPath); err != nil {
		// Roll back the on-disk record so the UI doesn't show a half-broken entry.
		_ = a.installer.Uninstall(installed.ID)
		return registry.InstalledEngine{}, fmt.Errorf("register engine: %w", err)
	}
	return *installed, nil
}

// GetCPUFeatures returns the detected CPU features.
func (a *App) GetCPUFeatures() string {
	return registry.DetectCPUFeatures().FeatureString()
}

// EngineOptionDef is a JSON-friendly view of registry.OptionDef where
// the heterogeneous Default and Recommended values are stringified.
type EngineOptionDef struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Default     string   `json:"default"`
	Min         *int     `json:"min,omitempty"`
	Max         *int     `json:"max,omitempty"`
	Vars        []string `json:"vars,omitempty"`
	Description string   `json:"description,omitempty"`
	Recommended string   `json:"recommended,omitempty"`
}

// EngineOptionConfig describes the editable UCI options for an installed
// engine: definitions sourced from the registry, plus the user's current
// override values.
type EngineOptionConfig struct {
	Definitions []EngineOptionDef `json:"definitions"`
	Values      map[string]string `json:"values"`
}

// GetEngineOptionConfig returns option definitions + current overrides
// for an installed engine.
func (a *App) GetEngineOptionConfig(engineID string) (EngineOptionConfig, error) {
	cfg := EngineOptionConfig{
		Definitions: []EngineOptionDef{},
		Values:      map[string]string{},
	}
	if a.installer == nil {
		return cfg, nil
	}
	installed, err := a.installer.GetInstalled(engineID)
	if err != nil {
		return cfg, err
	}
	if installed.OptionValues != nil {
		cfg.Values = installed.OptionValues
	}
	def, err := a.registry.GetEngine(installed.RegistryID)
	if err != nil {
		return cfg, nil // No registry def: still let user edit blank.
	}
	for name, opt := range def.Options {
		cfg.Definitions = append(cfg.Definitions, EngineOptionDef{
			Name:        name,
			Type:        opt.Type,
			Default:     anyToString(opt.Default),
			Min:         opt.Min,
			Max:         opt.Max,
			Description: opt.Description,
			Recommended: anyToString(opt.Recommended),
		})
	}
	return cfg, nil
}

// SetEngineOptionConfig persists per-engine UCI option overrides.
func (a *App) SetEngineOptionConfig(engineID string, options map[string]string) error {
	if a.installer == nil {
		return nil
	}
	return a.installer.UpdateOptions(engineID, options)
}

// EngineProfile lists a named UCI option preset.
type EngineProfile struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

// ListEngineProfiles returns the registry-defined profiles for an
// installed engine. Custom (non-registry) engines have no profiles.
func (a *App) ListEngineProfiles(engineID string) ([]EngineProfile, error) {
	if a.installer == nil {
		return nil, nil
	}
	installed, err := a.installer.GetInstalled(engineID)
	if err != nil {
		return nil, err
	}
	if installed.RegistryID == "" {
		return nil, nil
	}
	def, err := a.registry.GetEngine(installed.RegistryID)
	if err != nil {
		return nil, nil
	}
	out := make([]EngineProfile, 0, len(def.Profiles))
	for name, p := range def.Profiles {
		values := map[string]string{}
		for k, v := range p {
			values[k] = anyToString(v)
		}
		out = append(out, EngineProfile{Name: name, Values: values})
	}
	return out, nil
}

// ApplyEngineProfile overlays the profile values onto the engine's
// stored option overrides. Returns the new full override map.
func (a *App) ApplyEngineProfile(engineID, profileName string) (map[string]string, error) {
	if a.installer == nil {
		return nil, nil
	}
	return a.installer.ApplyProfile(engineID, profileName)
}

func anyToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		// TOML often decodes integers as float64.
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// StartTournament kicks off a tournament asynchronously.
func (a *App) StartTournament(spec TournamentSpec) (string, error) {
	if a.tournaments == nil {
		return "", nil
	}
	return a.tournaments.Start(spec)
}

// StopTournament cancels a running tournament.
func (a *App) StopTournament(id string) error {
	if a.tournaments == nil {
		return nil
	}
	return a.tournaments.Stop(id)
}

// DeleteTournament removes a finished tournament from history.
func (a *App) DeleteTournament(id string) error {
	if a.tournaments == nil {
		return nil
	}
	return a.tournaments.Delete(id)
}

// GetTournament returns a snapshot of one tournament.
func (a *App) GetTournament(id string) (TournamentSummary, error) {
	if a.tournaments == nil {
		return TournamentSummary{}, nil
	}
	return a.tournaments.Get(id)
}

// ListTournaments returns snapshots of all tournaments.
func (a *App) ListTournaments() []TournamentSummary {
	if a.tournaments == nil {
		return nil
	}
	return a.tournaments.List()
}

// GetGameDetail returns the per-ply replay of one game in a tournament.
func (a *App) GetGameDetail(tournamentID string, gameNumber int) (GameDetail, error) {
	if a.tournaments == nil {
		return GameDetail{}, nil
	}
	return a.tournaments.GetGameDetail(tournamentID, gameNumber)
}

// GetTournamentPGN returns all games of a tournament concatenated as PGN.
func (a *App) GetTournamentPGN(tournamentID string) (string, error) {
	if a.tournaments == nil {
		return "", nil
	}
	return a.tournaments.GetTournamentPGN(tournamentID)
}

// LiveGames returns snapshots of all currently-running games in a
// tournament so the GUI can seed its live grid on mount.
func (a *App) LiveGames(tournamentID string) ([]LiveGameSnapshot, error) {
	if a.tournaments == nil {
		return nil, nil
	}
	return a.tournaments.LiveGames(tournamentID)
}
