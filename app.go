package main

import (
	"context"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"rungine/internal/uci"
)

// App struct holds application state and provides Wails bindings.
type App struct {
	ctx     context.Context
	engines *uci.EngineManager
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		engines: uci.NewEngineManager(),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Wire up analysis events to frontend
	a.engines.SetAnalysisCallback(func(info uci.AnalysisInfo) {
		runtime.EventsEmit(ctx, "analysis:info", info)
	})
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
