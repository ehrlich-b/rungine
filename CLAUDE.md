# Rungine - Claude Code Instructions

## Project Overview

Rungine is a desktop chess application built with Wails v2 (Go backend + web frontend). It's a UCI omni-client where chess engines are the "servers". Core value prop: make running *any* UCI engine trivially easy while providing a polished game viewing and analysis experience.

## Tech Stack

- **Backend:** Go with Wails v2
- **Frontend:** Minimal JS (vanilla or Svelte preferred), embedded in binary
- **Database:** SQLite (prefer `modernc.org/sqlite` for pure Go)
- **IPC:** Wails bindings (Go ↔ TypeScript)

## Project Structure

```
/cmd/rungine/main.go             # Wails entry point
/internal/
    /uci/                        # UCI protocol and engine process management
    /registry/                   # Engine registry, downloads, CPU detection
    /pgn/                        # PGN parsing and writing
    /fen/                        # FEN parsing and validation
    /tournament/                 # Tournament coordination
    /database/                   # SQLite operations
    /live/                       # Lichess/Chess.com API clients
    /book/                       # Polyglot opening book reader
    /tablebase/                  # Syzygy tablebase probing
/frontend/                       # Web frontend
    /src/
    /dist/                       # Built assets (embedded)
/registry/
    engines.toml                 # Engine registry definitions
```

## Anchor Documents

When asked to "reanchor and proceed", read these files:
- README.md
- CLAUDE.md (this file)
- DESIGN.md
- TODO.md (if exists)

## Code Style

### Go
- Standard `gofmt` formatting
- Use `internal/` for non-exported packages
- Error handling: return errors, don't panic (except truly unrecoverable)
- Use `context.Context` for cancellation in long-running operations
- Prefer table-driven tests
- Use `slog` for structured logging

### Frontend
- Minimize dependencies - every KB counts in WebView
- Dark mode default
- Keep state minimal; Go backend is source of truth
- Use Wails bindings for all backend communication

### General
- No over-engineering - implement what's needed now
- Prefer editing existing files over creating new ones
- No emoji in code or commits
- Single-sentence commit messages

## Key Patterns

### UCI Engine Management
Engines run as subprocesses. Each engine gets:
- Dedicated goroutine for stdout reading
- Mutex-protected stdin writes
- Context for cancellation
- Channel for analysis info streaming

### Concurrent Analysis
Multiple engines analyze simultaneously via goroutines. Results stream through channels to frontend via Wails events.

### Registry System
Engine registry is TOML-based. Contains:
- Download URLs per platform/CPU feature
- SHA256 hashes for verification
- UCI option definitions
- Pre-configured profiles

## Testing

- Unit tests for UCI parsing, PGN parsing, FEN validation
- Integration tests for engine lifecycle
- Test against Stockfish as reference implementation
- Table-driven tests for tournament pairing logic

## Dependencies Policy

**Allowed:**
- `github.com/wailsapp/wails/v2`
- `modernc.org/sqlite` (pure Go SQLite)
- `github.com/klauspost/cpuid/v2` (CPU detection)
- `github.com/gorilla/websocket` (if needed for live streaming)
- `github.com/notnil/chess` (evaluate before using)

**Avoid:**
- Heavy web frameworks
- ORMs
- Logging frameworks beyond stdlib

## Performance Targets

- Binary size: <15MB
- App startup: <1 second
- Engine startup to first output: <500ms
- PGN import: 10,000+ games/second
- Memory idle: <100MB

## Common Tasks

### Adding a new engine to registry
1. Research license and redistribution rights
2. Find official download URLs for all platforms
3. Generate SHA256 hashes
4. Document UCI options
5. Add entry to `registry/engines.toml`
6. Test installation flow

### Implementing new UCI command
1. Add parsing in `/internal/uci/protocol.go`
2. Add method to Engine struct in `/internal/uci/engine.go`
3. Add Wails binding if frontend needs access
4. Test with Stockfish

### Adding frontend feature
1. Implement Go backend logic first
2. Expose via Wails binding
3. Build minimal frontend UI
4. Test IPC flow

## Debugging

- Engine I/O: Log all UCI traffic during development
- Wails bindings: Check browser devtools for binding errors
- Process issues: Use `ps` and check for zombie engine processes
- SQLite: Use `sqlite3` CLI to inspect database directly

## Don't

- Don't add features beyond what's requested
- Don't create abstractions for one-time operations
- Don't add backwards-compatibility shims
- Don't commit mid-refactor with broken code
- Don't bundle test fixtures in production binary
