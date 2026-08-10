# Rungine TODO

**Mission:** A world-class engine tournament runner and viewer. The runner schedules and arbitrates UCI engine matches at scale; the viewer presents live games, crosstables, and analysis with TCEC-quality polish.

Reference points: Cute Chess and fast-chess (runner correctness), TCEC broadcast (live viewer UX), Banksia GUI (engine management UX). Goal is to beat all three on the combined experience.

Status tracker. Check items as completed.

---

## Phase 1: Core Engine Infrastructure

### UCI Protocol (`internal/uci/`)

- [x] Define core data structures (`Engine`, `EngineState`, `UCIOption`, `AnalysisInfo`, `Score`)
- [x] Implement UCI line parser (`parseInfoLine`, `parseOptionLine`, `parseIdLine`)
- [x] Implement Engine struct with process lifecycle (Start, Stop, stdin/stdout goroutines)
- [x] Implement `uci` / `uciok` handshake
- [x] Implement `isready` / `readyok` synchronization
- [x] Implement `setoption` for configuring engine options
- [x] Implement `position` command (FEN and startpos variants)
- [x] Implement `go` command variants (infinite, depth, movetime, time controls)
- [x] Implement `stop` and `bestmove` handling
- [x] Implement crash detection
- [x] Add context-based cancellation throughout
- [x] Expose engine management via Wails bindings
- [x] Write unit tests for UCI parsing (table-driven)
- [x] Write integration tests with Stockfish

### Engine Manager

- [x] Create EngineManager to handle multiple concurrent engines
- [x] Implement engine instance lifecycle tracking
- [x] Stream analysis info to frontend via Wails events
- [x] Throttle UI events to prevent frontend flooding (10-20Hz)

---

## Phase 2: Engine Registry

### Registry Parser (`internal/registry/`)

- [x] Define TOML schema types (`EngineDefinition`, `Build`, `UCIOptionDef`, `Profile`)
- [x] Parse `registry/engines.toml`
- [x] Validate registry entries on load

### CPU Detection

- [x] Add `github.com/klauspost/cpuid/v2` dependency
- [x] Detect CPU features (AVX512, AVX2, BMI2, POPCNT, SSE42)
- [x] Select optimal build for current platform

### Engine Installation

- [x] Implement download with progress streaming
- [x] Implement SHA256 verification
- [x] Implement archive extraction (zip, tar, tar.gz)
- [x] Set executable permissions (Unix)
- [x] Validate engine by running `uci` and checking for `uciok`
- [x] Save installed engine config to `~/.rungine/engines/`
- [x] Expose installation via Wails bindings with progress events

### Populate Registry

- [x] Add Stockfish 17 entry (all platforms, all CPU variants)
- [x] Add Leela Chess Zero entry (with network file handling)
- [x] Add 2-3 other popular engines (Berserk, Koivisto)

---

## Phase 3: Notation Parsers

### FEN (`internal/fen/`)

- [x] Parse FEN string into position struct
- [x] Validate FEN components
- [x] Generate FEN from position
- [x] Unit tests with valid/invalid FEN strings

### PGN (`internal/pgn/`)

- [x] Tokenizer (tags, moves, comments, NAGs, variations)
- [x] Move tree construction with variation support
- [x] Standard 7-tag roster
- [x] Recursive variations (unlimited depth)
- [x] NAG symbols (`!`, `?`, `$N`)
- [x] Comments (`{...}`)
- [x] Write PGN from game tree
- [x] Unit tests for PGN parsing edge cases
- [x] Embedded annotations: write `[%eval ...]` and `[%clk ...]` per ply (writer in `internal/tournament/arbiter.go`)
- [x] Embedded annotations: read `[%eval ...]` and `[%clk ...]` per ply

---

## Phase 4: Build Hygiene & Chess Core

### Fix the broken build

- [x] `npm install && npm run build` produces `frontend/dist/`
- [x] `go build ./...` succeeds (embed resolves)
- [x] Fix macOS test failures in `internal/registry/registry_test.go` (parameterize OS/arch on `Manager`)
- [x] Verify `wails dev` launches with hot reload (requires `wails` CLI)
- [x] CI: `go build ./...`, `go test ./...`, frontend type check on push

### Chess core (`internal/chess/`)

Wraps `github.com/notnil/chess` with a UCI-oriented API for the arbiter.

- [x] Pick library: `github.com/notnil/chess` for v1 (replace later if needed)
- [x] `Game` type with FEN load and PGN export
- [x] `PushUCI` applies UCI moves, rejects illegal/malformed
- [x] Detect checkmate
- [x] Detect stalemate
- [x] Detect 50-move rule (auto-claimed)
- [x] Detect insufficient material
- [x] Detect threefold repetition (auto-claimed)
- [x] `Resign` / `Adjudicate` for arbiter-imposed terminations (time forfeit, illegal move, score-based)
- [x] `MovesUCI` / `MovesSAN` history accessors

---

## Phase 5: Tournament Engine (Backend)

### Game arbiter (`internal/tournament/arbiter.go`)

- [x] `Arbiter` struct: white engine, black engine, position, clocks, adjudication rules
- [x] Game loop: position cmd, go with appropriate clock, await bestmove
- [x] Per-side clock tracking with increment
- [x] Validate engine bestmove against legal moves (illegal move = forfeit)
- [x] Detect time forfeit
- [x] Detect engine crash mid-game (forfeit, optionally restart per rules)
- [x] Detect natural termination (mate / stalemate / 50-move / threefold / insufficient)
- [x] Resign adjudication (eval below threshold for N consecutive plies)
- [x] Draw adjudication (eval near zero for N consecutive plies, after min ply)
- [x] Capture per-ply move record (move, eval, depth, time used, clocks remaining) on `Result`
- [x] Stream move-by-move events for live view (Wails events) — `tournament:gameStart`, `tournament:move`, `tournament:gameComplete`
- [x] Produce final PGN with embedded `[%eval]` and `[%clk]` annotations

### Time controls

- [x] Sudden death (`90+0`)
- [x] Increment (`60+0.6`)
- [x] Moves + time (`40/15+0`) — clock bonus applied at period boundary, `movestogo` decrements within period
- [x] Fixed depth (`d=N`) for testing
- [x] Fixed nodes (`n=N`) for testing
- [x] Fixed movetime (`mt=ms`) for testing — known issue: time forfeits unexpectedly under load with Stockfish 18, investigate

### Opening selection

- [x] Load opening PGN file, sample positions at configurable ply — `LoadOpeningsFromPGN` in `internal/tournament/openings.go`
- [x] Polyglot `.bin` book reader (binary-search Zobrist lookup) — `internal/book`
- [ ] Weighted random move selection on top of `internal/book`
- [x] Pair mode: same opening played twice with colors flipped (in `formats.go` via `PairMode`)

### Concurrent scheduling (`internal/tournament/scheduler.go`)

- [x] Run N games in parallel (configurable Concurrency; semaphore-bounded fan-out)
- [x] Per-game engine instances (engines hold state, never share — EngineFactory spawns fresh per game)
- [x] Pair queue: scheduler accepts a slice of Pairings and dispatches them in order
- [x] Per-game lifecycle callbacks (OnGameStart, OnGameComplete) for live progress reporting
- [x] Resource accounting: total threads, hash across concurrent games — `EstimateUsage` in `internal/tournament/resources.go`
- [x] Pause / resume tournament — `Scheduler.Pause`/`Resume`/`Paused`
- [x] Resume from on-disk state after process restart — finished tournaments hydrate from SQLite; running rows recover as `interrupted`

---

## Phase 6: Tournament Formats & Rating

### Formats

- [x] Match (two engines, N games) — `BuildMatch` in `internal/tournament/formats.go`
- [x] Round robin (N engines, `N*(N-1)` games) — `BuildRoundRobin`
- [x] Gauntlet (one engine vs field) — `BuildGauntlet`
- [x] Swiss (configurable rounds, no repeat pairings, score-paired) — `BuildSwissRound`

### Scoring

- [x] Score table per engine (W/D/L, points) — `BuildStandings` in `internal/tournament/scoring.go`
- [x] Crosstable (head-to-head matrix) — `BuildCrosstable`
- [x] ELO calculation with iterative convergence (Ordo-style performance-rating fit) — `EstimateElos`
- [x] ELO confidence intervals (trinomial-variance normal approximation) — `EloInterval`
- [x] LOS, draw ratio, performance rating — `LikelihoodOfSuperiority`, `DrawRatio`, `PerformanceRating`

### SPRT (engine development mode)

- [x] SPRT calculator: LLR, lower/upper bounds from `elo0` / `elo1` / `alpha` / `beta` — `internal/tournament/sprt.go`
- [x] SPRT termination (accept H0 / accept H1 / continue) — `Scheduler.ShouldStop` + `NewSPRTStopper`
- [x] Live LLR display during match — `NewSPRTStopperWithProgress` + `tournament:sprt` event + dashboard panel

---

## Phase 7: GUI Foundation

### Frontend stack

- [x] Svelte + Vite + TypeScript (Svelte 5 runes, vite 5)
- [x] App shell: top nav (Tournaments, Engines, Settings), main pane, status bar
- [x] Wails event subscription helpers — `frontend/src/lib/wails.ts`
- [x] Dark theme default, light theme optional — `frontend/src/lib/theme.ts`, `styles/theme.css`
- [x] Accent color setting — six-color palette in Settings via `setAccent` (`frontend/src/lib/theme.ts`)

### Chessboard component

- [x] 8x8 board with toggleable file/rank labels — CSS grid in `Board.svelte` (SVG would be a re-implementation, not a feature)
- [ ] SVG piece set (permissive license — Cburnett or Merida) — currently Unicode glyphs
- [x] Render position from FEN
- [x] Last-move highlight
- [x] Check highlight (red radial overlay on king square when in check)
- [x] Arrow annotations (engine PVs) — circles still TODO
- [ ] Circle annotations (square markers for analysis)
- [x] Flip board (`flipped` prop + 'F' shortcut)
- [x] Animated piece transitions during replay (150ms slide via Web Animations API on the move's destination)

### Move list / PGN navigation

- [x] Render mainline in SAN — `GameView.svelte` two-column move pairs
- [x] Click move to jump
- [x] Keyboard navigation (← → Home End, also F to flip)
- [ ] Variation rendering (collapsible) — N/A for tournament games (no variations); deferred
- [ ] NAG glyphs — N/A for tournament games (no NAGs); deferred
- [x] Inline engine annotations (eval, depth, clock)

---

## Phase 8: Tournament Viewer UI (the differentiator)

The "world-class" half. TCEC-quality live broadcast.

### Tournament setup

- [x] Format picker (match, round-robin, gauntlet) — Swiss/SPRT still unsupported in GUI
- [x] Engine multi-select from installed
- [x] Per-engine UCI option overrides at tournament level — per-slot options textarea
- [x] Time control picker — movetime, T+I, depth, nodes
- [x] Opening source: standard startpos / specific FEN — PGN/Polyglot still TODO
- [x] Concurrency setting
- [x] Save tournament config presets — localStorage-backed preset list in setup form

### Live tournament dashboard

- [x] Header: tournament name, status, progress (X/N games)
- [x] Standings updating as games complete (with ELO + 95% CI)
- [x] Crosstable updates as games complete (rendered when 3+ engines)
- [x] Live games grid (mini-boards with current ply, last move, eval)
- [x] Click into a game for full view

### Single-game live view

- [x] Big board with current position
- [x] Move list with engine eval annotations
- [x] Per-engine panel: name, depth, eval, nodes, NPS, PV — `tournament:engineInfo` event + `GameView` panel
- [x] Eval graph (white POV, click-to-jump, arrow-key nav)
- [x] Clock display (both sides, ticking) — `frontend/src/components/LiveGames.svelte`
- [x] PV-on-board overlay (arrows for engine's planned line) — Board accepts `arrows` prop; GameView feeds side-to-move's PV[0] when viewing the live ply

### Replay finished games

- [x] Step through with engine analysis preserved (cached during run)
- [x] Keyboard navigation (← → Home End F)
- [x] Scrubber across game timeline — range slider in `GameView.svelte`
- [x] Export game PGN with `[%eval]` and `[%clk]` — Copy PGN button

### Tournament results page

- [x] Final crosstable (rendered when 3+ engines)
- [x] ELO with CI per engine
- [x] All games table (white / black / result / reason)
- [x] Export full tournament PGN
- [x] Save tournament to database — `internal/database` SQLite, hydrated on app startup

---

## Phase 9: Engine Library UI

- [x] List engines from registry (installed vs available) — `frontend/src/views/Engines.svelte`
- [x] One-click install (progress shown as text; graphical bar still TODO)
- [x] CPU feature display (StatusBar + Settings)
- [x] Configure UCI options (spin / check / combo / string / button editors)
- [x] Engine profiles (analysis, tournament, quick) — registry profiles listed and applied via `App.ApplyEngineProfile`
- [x] Remove installed engine
- [x] Add custom engine (browse for binary, validates via UCI handshake) — `App.PickEngineBinary` + `App.AddCustomEngine`

---

## Phase 10: Database

Scoped to tournament storage, not generic game library.

- [x] SQLite at `~/.rungine/rungine.db` via `modernc.org/sqlite`
- [x] Migration system — `schema_version` table, append-only `migrations[]` slice
- [x] Tournaments table (config, status, results)
- [x] Games table (linked to tournament, includes embedded analysis)
- [x] Engine version table (track which build played which game) — SHA256 stored per persisted engine, surfaced as short hash in GameView header
- [ ] Position index (Zobrist hash) for repetition lookup and game search
- [x] Tournament list / search UI — history panel with delete, re-run, winner column, timestamp
- [x] Re-run tournament with same config — copies stored `TournamentSpec` back into the setup form

---

## Phase 11: Polish & Release

### Performance

- [ ] Startup <1s
- [ ] Engine startup-to-uciok <500ms
- [ ] Idle memory <100MB
- [ ] Binary <15MB
- [ ] Profile under heavy concurrency (32 parallel games)
- [ ] Frontend frame rate during live updates (target 60fps)

### Distribution

- [x] GitHub Actions: build for darwin-arm64, darwin-amd64, linux-amd64, windows-amd64 — `.github/workflows/release.yml`, manual + tag-triggered
- [x] Release on tag — `softprops/action-gh-release@v2` attaches per-platform archives to a `v*` tag release
- [ ] Notarize macOS builds (requires Apple Developer credentials)
- [ ] Code sign Windows (if feasible)

### Docs

- [ ] README screenshots / GIFs of live tournament
- [ ] Tournament config schema reference
- [ ] Keyboard shortcuts reference
- [ ] Registry contributor guide

---

## Backlog (post-1.0)

Not on the critical path to "world-class tournament viewer and runner".

- [ ] Lichess broadcast follow (watch external tournaments)
- [ ] Chess.com archive import
- [ ] Generic PGN library import / massive game database
- [x] Standalone analysis mode (load FEN, run engines, no tournament)
- [ ] Opening book editor
- [ ] Cloud / remote engine analysis
- [ ] Chess960 / variants (via Fairy-Stockfish)
- [ ] Knockout format
- [ ] Pentanomial SPRT
- [ ] Tablebase adjudication (Syzygy WDL probing)
- [ ] Plugin system for custom analysis tools
- [ ] Registry GPG signature verification
- [ ] Auto-update for engines
- [ ] PGN import benchmark (10,000+ games/sec)

---

## Notes

- Commit at logical milestones, not after every small change
- Run `make dev` frequently to verify nothing is broken
- Test with Stockfish as reference UCI implementation
- Keep frontend minimal; complex logic belongs in Go
