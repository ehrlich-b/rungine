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
- [ ] Polyglot `.bin` book reader (Zobrist lookup, weighted random)
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

## Phase 12: Lichess-grade UX overhaul (CURRENT FOCUS)

Driven by hands-on testing 2026-05-30. Several Phase 7–8 items are technically
"done" but not good enough to actually use; this phase upgrades them.
**North star: clone the Lichess analysis board** — board + vertical eval bar on
the left; a right rail of ranked engine PV lines, the move list, and PGN/FEN
load; first/prev/next/last + flip controls under the board; eval graph along the
bottom. Land these incrementally and verify each in the running app — do not
one-shot.

### 12.0 Shared primitives
- [x] Target-layout spec — DESIGN.md "Appendix: Analysis & Game View Layout (Phase 12)"
- [x] `EvalBar.svelte` (vertical, white-advantage logistic fill, clamped, flips with board); wired into `GameView` beside the board. Analyze reuses it in 12.1
- [x] `uciToArrow(uci, color?, weight?)` + shared `Arrow` type + `whitePov(value, side)` in `lib/chess.ts`; `Board`/`GameView` now import the shared `Arrow` (dupes removed) and `GameView.pvToArrow` uses the helper
- [x] Fixed latent eval-POV bug: `GameView` move-list / move-info evals were shown raw (mover-POV) — a black-favoured move read as good-for-white. Now normalized via `whitePov`, matching the eval graph (which already flipped)

### 12.1 Analysis board — the headline (`views/Analyze.svelte`)
**Slice 1 (done):** the bare panels are replaced with a Lichess-style board.
- [x] PV arrows on the board from the primary engine's lines — top line solid accent, secondary lines faded (`uciToArrow`)
- [x] Multi-PV: set `MultiPV=3` per engine before analysis; render N ranked lines (eval / depth / PV). Required a backend fix — `emitThrottled` now keys the 20Hz gate per `(engine, MultiPV)` (`manager.go`), else lines 2/3 were dropped
- [x] Eval bar (white POV) + large eval number tied to the primary engine's best line
- [x] Lichess layout: board + eval bar left; ranked engine lines right
**Slice 2 (remaining — needs a backend `ApplyMove` binding):**
- [ ] Make moves on the board (click/drag) to explore a line; build a local move tree; re-issue `position` + analysis per move
- [ ] Move list in the right rail; keyboard nav within the explored line (reuse GameView ← → Home End)
- [ ] PV lines show SAN, not raw UCI (needs a `fen + uciMoves → SAN[]` helper; throttle-friendly)

### 12.2 Pre-canned games (none exist today — UI has no sample data)
- [ ] Bundle a few PGNs as frontend assets (famous games and/or `~/repos/ngn/the_game.pgn`, `games.pgn`)
- [ ] "Open game": paste-PGN textarea + starter gallery; parse via backend PGN parser; replay in `GameView` (board + eval graph + move list already there)
- [ ] Add nav entry (`lib/router.ts` + `Nav.svelte`) or fold a "load game" affordance into Analyze

### 12.3 Tournament live view — "goes to running but I can't see it" (FIXED)
**Root cause (confirmed by inspection):** a subscribe-after-emit race. `StartTournament`
launches the scheduler immediately, which fires `tournament:gameStart` within ms — *before*
the frontend finishes its `GetTournament`/`ListTournaments` round-trips and mounts `LiveGames`
to subscribe. The initial `gameStart` events are lost, and because the `move` handler did
`if (!existing) return`, every subsequent move was dropped too, so a low-concurrency match
showed an empty grid for the whole game (only the separate dashboard `gameComplete` path
recorded the finished game — hence "I only see it after it's done"). Event keys matched fine;
the casing hypothesis was wrong.
- [x] Confirmed `gameStart`/`move` fire and `emit` reaches the UI (same path `gameComplete` uses)
- [x] Verified event payload keys match (`fen/ply/uci/san/side/evalCp/evalMate/clockAfterMs`) — ruled out as the cause
- [x] **Fix: snapshot backfill.** New `App.LiveGames(id)` binding returns running games (`app_tournament.go` `LiveGames`/`liveGameSnapshot`); `LiveGames.svelte` `$effect` seeds the grid from it on mount and on `tournamentId` change, so it no longer depends on catching `gameStart`. Also fixes navigating into an already-running tournament
- [x] Fixed a second bug: move handler read `p.side === 'black'` but `chess.Side` serializes to `"w"`/`"b"`, so `movedSide` was always `'w'` → wrong side-to-move indicator and black clock never updated. Now `p.side === 'b'`
- [x] Live-games grid populates on `gameStart` and updates every `move` (was already correct for post-subscription events; backfill covers the rest)
- [x] Open a running game into `GameView` and stream it live — already wired: `openGame`→`GetGameDetail` returns live detail via `buildLiveGameDetail`, and `tournamentId` is passed so GameView subscribes
- [x] Regression test: `tournaments.spec.ts` "live games grid backfills from snapshot without a gameStart event"

### 12.4 Tournament setup declutter ("horrendously busy")
Form crams everything at once (`Tournaments.svelte:509-828`): engine slots, format, TC, adjudication, SPRT, per-slot options, openings, presets.
- [ ] Progressive disclosure: essentials visible (engines, format, time control, Start); collapse Advanced (adjudication, SPRT, per-engine UCI options, openings, max-plies)
- [ ] Regroup into clear sections, consistent spacing, fix cramped/overlapping controls

### 12.5 Layout / overlap pass (global — "overlapping all over the place")
Concrete suspects from the audit:
- [ ] Nested scroll containers: dashboard `overflow:hidden` wrapping tables with `overflow-x:auto` (`Tournaments.svelte:~1110-1116`)
- [ ] Absolute-positioned progress text overlapping the bar (`Tournaments.svelte:~1417`)
- [ ] Fixed `max-height:540px` move list, not viewport-aware (`GameView.svelte:~582`)
- [ ] LiveGames grid has no overflow handling (`LiveGames.svelte:~221`); check-glow gradient oversized on 28px miniboards (`Board.svelte:319-330`)
- [ ] Board arrows clipped by `.grid overflow:hidden` (`Board.svelte:~273`)
- [ ] Walk every view at narrow widths; kill horizontal scroll / overlap

### 12.6 Board polish (folds in open Phase 7 items)
- [ ] SVG piece set (Cburnett/Merida) replacing Unicode glyphs — biggest visual gap vs Lichess
- [ ] Circle annotations (square markers)

### 12.7 Engines
- [x] Install **Blunder 8.0.0** (~2674 CCRL) and **ngn 0.1.0** (~1600) alongside Stockfish 17 / 16.1 — binaries copied to `~/.rungine/engines/custom-{blunder-800,ngn}/` with hand-written `config.toml`; verified playing through the arbiter (Blunder beat ngn 1-0, 75 plies). Other Blunder ladder rungs (v5.0.0–v7.4.0, ~2080–2532 CCRL) sit in `~/repos/ngn/opponents/` if a wider field is wanted.

### Loose ends (pre-existing, low priority)
- [ ] e2e fixture fix for `ListEngineProfiles`/`ApplyEngineProfile` is applied but **uncommitted** (`frontend/tests/e2e/fixtures.ts`) — commit it
- [ ] CLI SPRT shows no live LLR (uses `NewSPRTStopper`, prints only on terminal decision); could switch to `NewSPRTStopperWithProgress`
- [x] `go.mod`: `modernc.org/sqlite` promoted to a direct dependency via `go mod tidy`

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
