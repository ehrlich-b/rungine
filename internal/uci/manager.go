package uci

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// EngineManager manages multiple concurrent chess engines.
type EngineManager struct {
	engines map[string]*Engine
	mu      sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// Event callback for streaming analysis to frontend
	onAnalysis func(info AnalysisInfo)

	// Throttling
	throttleInterval time.Duration
	lastEmit         map[string]time.Time
	throttleMu       sync.Mutex

	logger *slog.Logger
}

// NewEngineManager creates a new engine manager.
func NewEngineManager() *EngineManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &EngineManager{
		engines:          make(map[string]*Engine),
		ctx:              ctx,
		cancel:           cancel,
		throttleInterval: 50 * time.Millisecond, // 20Hz default
		lastEmit:         make(map[string]time.Time),
		logger:           slog.Default().With("component", "engine-manager"),
	}
}

// SetAnalysisCallback sets the callback for analysis info updates.
// The callback is invoked from a goroutine; it should be safe for concurrent use.
func (m *EngineManager) SetAnalysisCallback(cb func(info AnalysisInfo)) {
	m.mu.Lock()
	m.onAnalysis = cb
	m.mu.Unlock()
}

// SetThrottleRate sets the minimum interval between analysis updates per engine.
func (m *EngineManager) SetThrottleRate(hz int) {
	if hz <= 0 {
		hz = 20
	}
	m.throttleInterval = time.Second / time.Duration(hz)
}

// RegisterEngine creates and registers a new engine instance.
func (m *EngineManager) RegisterEngine(id, binaryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.engines[id]; exists {
		return fmt.Errorf("engine %s already registered", id)
	}

	engine := NewEngine(id, binaryPath)
	m.engines[id] = engine
	m.logger.Info("engine registered", "id", id, "path", binaryPath)
	return nil
}

// UnregisterEngine removes an engine from the manager.
// If the engine is running, it will be stopped first.
func (m *EngineManager) UnregisterEngine(id string) error {
	m.mu.Lock()
	engine, exists := m.engines[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("engine %s not found", id)
	}
	delete(m.engines, id)
	m.mu.Unlock()

	if engine.State() != EngineStateNone && engine.State() != EngineStateStopped {
		engine.Stop()
	}

	m.logger.Info("engine unregistered", "id", id)
	return nil
}

// GetEngine returns an engine by ID.
func (m *EngineManager) GetEngine(id string) (*Engine, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	engine, exists := m.engines[id]
	if !exists {
		return nil, fmt.Errorf("engine %s not found", id)
	}
	return engine, nil
}

// ListEngines returns info about all registered engines.
func (m *EngineManager) ListEngines() []EngineInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]EngineInfo, 0, len(m.engines))
	for _, e := range m.engines {
		infos = append(infos, EngineInfo{
			ID:         e.ID,
			Name:       e.Name,
			Author:     e.Author,
			BinaryPath: e.BinaryPath,
			State:      e.State().String(),
		})
	}
	return infos
}

// StartEngine starts an engine process.
func (m *EngineManager) StartEngine(id string) error {
	engine, err := m.GetEngine(id)
	if err != nil {
		return err
	}

	if err := engine.Start(m.ctx); err != nil {
		return fmt.Errorf("start engine %s: %w", id, err)
	}

	// Start analysis streaming goroutine
	go m.streamAnalysis(engine)

	return nil
}

// StopEngine stops an engine process.
func (m *EngineManager) StopEngine(id string) error {
	engine, err := m.GetEngine(id)
	if err != nil {
		return err
	}
	return engine.Stop()
}

// StartAnalysis begins analysis on one or more engines.
func (m *EngineManager) StartAnalysis(fen string, moves []string, engineIDs []string, params GoParams) error {
	for _, id := range engineIDs {
		engine, err := m.GetEngine(id)
		if err != nil {
			return err
		}

		if engine.State() != EngineStateReady {
			return fmt.Errorf("engine %s not ready (state: %s)", id, engine.State())
		}

		if err := engine.SetPosition(fen, moves); err != nil {
			return fmt.Errorf("set position on %s: %w", id, err)
		}

		if err := engine.Go(params); err != nil {
			return fmt.Errorf("start analysis on %s: %w", id, err)
		}
	}
	return nil
}

// StopAnalysis stops analysis on all specified engines.
func (m *EngineManager) StopAnalysis(engineIDs []string) error {
	var lastErr error
	for _, id := range engineIDs {
		engine, err := m.GetEngine(id)
		if err != nil {
			lastErr = err
			continue
		}
		if err := engine.StopSearch(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// StopAll stops all running engines.
func (m *EngineManager) StopAll() {
	m.mu.RLock()
	engines := make([]*Engine, 0, len(m.engines))
	for _, e := range m.engines {
		engines = append(engines, e)
	}
	m.mu.RUnlock()

	for _, e := range engines {
		e.Stop()
	}
}

// Shutdown stops all engines and cancels the manager context.
func (m *EngineManager) Shutdown() {
	m.logger.Info("shutting down engine manager")
	m.StopAll()
	m.cancel()
}

// streamAnalysis reads from an engine's info channel and dispatches to the callback.
func (m *EngineManager) streamAnalysis(engine *Engine) {
	infoCh := engine.InfoChannel()
	for {
		select {
		case info, ok := <-infoCh:
			if !ok {
				return
			}
			m.emitThrottled(info)
		case <-m.ctx.Done():
			return
		}
	}
}

// emitThrottled sends analysis info to the callback, throttled per engine.
func (m *EngineManager) emitThrottled(info AnalysisInfo) {
	m.throttleMu.Lock()
	last, exists := m.lastEmit[info.EngineID]
	now := time.Now()

	// Always emit if it's a significant update (bestmove comes through differently,
	// but deep depths or final results should go through)
	shouldEmit := !exists ||
		now.Sub(last) >= m.throttleInterval ||
		info.Depth >= 20 || // Always emit deep analysis
		len(info.PV) == 0 // Status updates without PV

	if shouldEmit {
		m.lastEmit[info.EngineID] = now
	}
	m.throttleMu.Unlock()

	if !shouldEmit {
		return
	}

	m.mu.RLock()
	cb := m.onAnalysis
	m.mu.RUnlock()

	if cb != nil {
		cb(info)
	}
}

// EngineInfo provides summary info about an engine for the frontend.
type EngineInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Author     string `json:"author"`
	BinaryPath string `json:"binaryPath"`
	State      string `json:"state"`
}
