package uci

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

var (
	ErrEngineNotRunning = errors.New("engine not running")
	ErrEngineTimeout    = errors.New("engine timeout")
	ErrEngineCrashed    = errors.New("engine crashed")
)

// Engine represents a UCI chess engine process.
type Engine struct {
	ID         string
	Name       string
	Author     string
	BinaryPath string

	process *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser

	state   EngineState
	options map[string]UCIOption

	outputCh chan ParsedLine
	infoCh   chan AnalysisInfo
	doneCh   chan struct{}

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	logger *slog.Logger
}

// NewEngine creates a new Engine instance.
func NewEngine(id, binaryPath string) *Engine {
	return &Engine{
		ID:         id,
		BinaryPath: binaryPath,
		state:      EngineStateNone,
		options:    make(map[string]UCIOption),
		logger:     slog.Default().With("engine", id),
	}
}

// State returns the current engine state.
func (e *Engine) State() EngineState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// Options returns a copy of the engine's UCI options.
func (e *Engine) Options() map[string]UCIOption {
	e.mu.Lock()
	defer e.mu.Unlock()
	opts := make(map[string]UCIOption, len(e.options))
	for k, v := range e.options {
		opts[k] = v
	}
	return opts
}

// Start launches the engine process and initializes UCI.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.state != EngineStateNone && e.state != EngineStateStopped && e.state != EngineStateError {
		e.mu.Unlock()
		return fmt.Errorf("engine already running (state: %s)", e.state)
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.state = EngineStateStarting
	e.outputCh = make(chan ParsedLine, 100)
	e.infoCh = make(chan AnalysisInfo, 100)
	e.doneCh = make(chan struct{})
	e.mu.Unlock()

	// Start process
	e.process = exec.CommandContext(e.ctx, e.BinaryPath)

	var err error
	e.stdin, err = e.process.StdinPipe()
	if err != nil {
		e.setState(EngineStateError)
		return fmt.Errorf("stdin pipe: %w", err)
	}

	e.stdout, err = e.process.StdoutPipe()
	if err != nil {
		e.setState(EngineStateError)
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := e.process.Start(); err != nil {
		e.setState(EngineStateError)
		return fmt.Errorf("start process: %w", err)
	}

	e.logger.Info("engine process started", "pid", e.process.Process.Pid)

	// Start reader goroutine
	go e.readLoop()

	// Start monitor goroutine
	go e.monitor()

	// Send UCI init and wait for uciok
	if err := e.initUCI(); err != nil {
		e.Stop()
		return err
	}

	return nil
}

// Stop terminates the engine process.
func (e *Engine) Stop() error {
	e.mu.Lock()
	if e.state == EngineStateNone || e.state == EngineStateStopped {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	e.logger.Info("stopping engine")

	// Try graceful quit
	e.sendCommand("quit")

	// Give it a moment to exit gracefully
	select {
	case <-e.doneCh:
		// Clean exit
	case <-time.After(500 * time.Millisecond):
		// Force kill
		if e.process != nil && e.process.Process != nil {
			e.process.Process.Kill()
		}
	}

	e.cancel()
	e.setState(EngineStateStopped)
	return nil
}

// InfoChannel returns the channel for analysis info updates.
func (e *Engine) InfoChannel() <-chan AnalysisInfo {
	return e.infoCh
}

// SetOption sets a UCI option value.
func (e *Engine) SetOption(name, value string) error {
	if e.State() != EngineStateReady && e.State() != EngineStateThinking {
		return ErrEngineNotRunning
	}

	cmd := BuildSetOptionCommand(name, value)
	if err := e.sendCommand(cmd); err != nil {
		return err
	}

	e.mu.Lock()
	if opt, ok := e.options[name]; ok {
		opt.Value = value
		e.options[name] = opt
	}
	e.mu.Unlock()

	return nil
}

// SetPosition sends a position command to the engine.
func (e *Engine) SetPosition(fen string, moves []string) error {
	if e.State() != EngineStateReady {
		return ErrEngineNotRunning
	}

	cmd := BuildPositionCommand(fen, moves)
	return e.sendCommand(cmd)
}

// Go starts the engine searching with the given parameters.
func (e *Engine) Go(params GoParams) error {
	state := e.State()
	if state != EngineStateReady {
		return fmt.Errorf("engine not ready (state: %s)", state)
	}

	e.setState(EngineStateThinking)

	cmd := BuildGoCommand(params)
	return e.sendCommand(cmd)
}

// StopSearch stops the current search.
func (e *Engine) StopSearch() error {
	if e.State() != EngineStateThinking && e.State() != EngineStatePondering {
		return nil
	}

	return e.sendCommand("stop")
}

// IsReady sends isready and waits for readyok.
func (e *Engine) IsReady(timeout time.Duration) error {
	if e.State() == EngineStateNone || e.State() == EngineStateStopped {
		return ErrEngineNotRunning
	}

	if err := e.sendCommand("isready"); err != nil {
		return err
	}

	return e.waitFor("readyok", timeout)
}

func (e *Engine) initUCI() error {
	if err := e.sendCommand("uci"); err != nil {
		return err
	}

	// Wait for uciok with timeout
	timeout := 5 * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case line, ok := <-e.outputCh:
			if !ok {
				return ErrEngineCrashed
			}
			switch line.Type {
			case "id_name":
				e.mu.Lock()
				e.Name = line.Data.(string)
				e.mu.Unlock()
				e.logger.Info("engine identified", "name", e.Name)
			case "id_author":
				e.mu.Lock()
				e.Author = line.Data.(string)
				e.mu.Unlock()
			case "option":
				opt := line.Data.(UCIOption)
				e.mu.Lock()
				e.options[opt.Name] = opt
				e.mu.Unlock()
			case "uciok":
				e.setState(EngineStateReady)
				e.logger.Info("UCI initialization complete", "options", len(e.options))
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("%w: waiting for uciok", ErrEngineTimeout)
		case <-e.ctx.Done():
			return e.ctx.Err()
		}
	}
}

func (e *Engine) waitFor(responseType string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case line, ok := <-e.outputCh:
			if !ok {
				return ErrEngineCrashed
			}
			if line.Type == responseType {
				return nil
			}
			// Process other lines while waiting
			e.handleLine(line)
		case <-timer.C:
			return fmt.Errorf("%w: waiting for %s", ErrEngineTimeout, responseType)
		case <-e.ctx.Done():
			return e.ctx.Err()
		}
	}
}

func (e *Engine) handleLine(line ParsedLine) {
	switch line.Type {
	case "info":
		info := line.Data.(AnalysisInfo)
		info.EngineID = e.ID
		select {
		case e.infoCh <- info:
		default:
			// Channel full, drop oldest
			select {
			case <-e.infoCh:
			default:
			}
			e.infoCh <- info
		}
	case "bestmove":
		e.setState(EngineStateReady)
	}
}

func (e *Engine) sendCommand(cmd string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stdin == nil {
		return ErrEngineNotRunning
	}

	e.logger.Debug("sending command", "cmd", cmd)
	_, err := fmt.Fprintln(e.stdin, cmd)
	return err
}

func (e *Engine) readLoop() {
	defer close(e.outputCh)
	defer close(e.infoCh)

	scanner := bufio.NewScanner(e.stdout)
	for scanner.Scan() {
		line := scanner.Text()
		e.logger.Debug("received", "line", line)

		parsed := ParseLine(line)
		select {
		case e.outputCh <- parsed:
		case <-e.ctx.Done():
			return
		}

		// Also handle info/bestmove directly for channel dispatch
		e.handleLine(parsed)
	}

	if err := scanner.Err(); err != nil && e.ctx.Err() == nil {
		e.logger.Error("read error", "err", err)
	}
}

func (e *Engine) monitor() {
	defer close(e.doneCh)

	err := e.process.Wait()
	if err != nil && e.ctx.Err() == nil {
		// Unexpected crash
		e.logger.Error("engine crashed", "err", err)
		e.setState(EngineStateError)
	}
}

func (e *Engine) setState(state EngineState) {
	e.mu.Lock()
	e.state = state
	e.mu.Unlock()
}
