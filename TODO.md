# Rungine TODO

Project status tracker. Check items as completed.

---

## Phase 1: Core Engine Infrastructure

### UCI Protocol (`internal/uci/`)

- [x] Define core data structures (`Engine`, `EngineState`, `UCIOption`, `AnalysisInfo`, `Score`)
- [x] Implement UCI line parser (`parseInfoLine`, `parseOptionLine`, `parseIdLine`)
- [x] Implement Engine struct with process lifecycle (Start, Stop, stdin/stdout goroutines)
- [x] Implement `uci` command and `uciok` response handling
- [x] Implement `isready` / `readyok` synchronization
- [x] Implement `setoption` for configuring engine options
- [x] Implement `position` command (FEN and startpos variants)
- [x] Implement `go` command variants (infinite, depth, movetime, time controls)
- [x] Implement `stop` and `bestmove` handling
- [x] Implement crash detection and recovery
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

## Phase 3: Chess Logic

### FEN Parser (`internal/fen/`)

- [x] Parse FEN string into position struct
- [x] Validate FEN components (piece placement, side to move, castling, en passant, halfmove, fullmove)
- [x] Generate FEN from position
- [x] Unit tests with valid/invalid FEN strings

### PGN Parser (`internal/pgn/`)

- [x] Implement tokenizer (tags, moves, comments, NAGs, variations)
- [x] Implement move tree construction with variation support
- [x] Parse standard 7-tag roster
- [x] Handle recursive variations (unlimited depth)
- [x] Handle NAG symbols (!, ?, $N)
- [x] Handle comments ({...})
- [x] Write PGN from game tree
- [x] Unit tests for PGN parsing edge cases
- [ ] Benchmark: target 10,000+ games/second import speed

---

## Phase 4: Frontend - Core UI

### Chessboard Rendering

- [ ] Create SVG-based board component
- [ ] Render pieces (use or create SVG piece set)
- [ ] Implement click-to-select and click-to-move
- [ ] Implement drag-and-drop moves
- [ ] Highlight last move
- [ ] Highlight check
- [ ] Show legal move indicators
- [ ] Flip board functionality
- [ ] Coordinate labels (a-h, 1-8)

### Position Navigation

- [ ] Bind Go backend game state to frontend
- [ ] Navigate to position by ply
- [ ] Keyboard navigation (arrow keys)
- [ ] Button controls (first, prev, next, last)

### Move List Display

- [ ] Display main line in SAN notation
- [ ] Show move numbers correctly
- [ ] Highlight current move
- [ ] Click move to navigate
- [ ] Display variations (collapsible)
- [ ] Display comments
- [ ] Display NAG symbols as glyphs

### Engine Panel

- [ ] Display engine name and status
- [ ] Display current depth, score (cp/mate), nodes, NPS
- [ ] Display principal variation (clickable to preview)
- [ ] Multiple engine panels (tabbed or stacked)
- [ ] Start/stop analysis buttons
- [ ] Engine selector dropdown

### Basic Layout

- [ ] Main layout: board left, moves center, engine panels right
- [ ] Responsive sizing
- [ ] Dark theme (default)

---

## Phase 5: Database

### SQLite Setup (`internal/database/`)

- [ ] Add `modernc.org/sqlite` dependency
- [ ] Create database file at `~/.rungine/games.db`
- [ ] Implement schema migration system
- [ ] Create games table
- [ ] Create positions table with FEN hash index
- [ ] Create FTS5 virtual table for search
- [ ] Create tags and game_tags tables

### Game Operations

- [ ] Insert game with position indexing
- [ ] Batch import PGN file (transaction batching)
- [ ] Search games by player, event, opening
- [ ] Search games by position (FEN hash lookup)
- [ ] Export game(s) to PGN

### Wails Bindings

- [ ] `ImportPGN(path string) (count int, err error)`
- [ ] `SearchGames(query) []GameSummary`
- [ ] `GetGame(id int) Game`
- [ ] `DeleteGame(id int) error`
- [ ] `TagGame(gameID, tag string) error`

---

## Phase 6: Frontend - Extended UI

### Game Database UI

- [ ] Game list with columns (White, Black, Result, Date, Event, ECO)
- [ ] Search/filter form (player names, date range, result)
- [ ] Position search (paste FEN, find games with position)
- [ ] Load game from database into viewer
- [ ] Import PGN file dialog with progress
- [ ] Delete game confirmation

### Engine Library UI

- [ ] List available engines from registry
- [ ] Show installed vs available status
- [ ] Install button with progress bar
- [ ] Configure engine options UI (spin, check, combo, string)
- [ ] Engine profiles (quick-switch configurations)
- [ ] Remove installed engine

### Settings UI

- [ ] Hash table size default
- [ ] Default thread count
- [ ] Board theme selection
- [ ] Piece set selection (if multiple)
- [ ] Analysis throttle rate
- [ ] Tablebase path configuration

---

## Phase 7: Tournament System

### Game Arbiter (`internal/tournament/`)

- [ ] Define time control struct
- [ ] Define adjudication rules struct
- [ ] Implement arbiter game loop (position, go, bestmove cycle)
- [ ] Track clocks per side
- [ ] Detect game termination (checkmate, stalemate, time forfeit)
- [ ] Implement resign adjudication (threshold + move count)
- [ ] Implement draw adjudication (threshold + move count)
- [ ] Detect 50-move rule
- [ ] Detect threefold repetition
- [ ] Detect insufficient material

### Tournament Coordinator

- [ ] Round-robin pairing
- [ ] Swiss pairing (basic)
- [ ] Gauntlet mode (one vs field)
- [ ] Opening book integration (Polyglot .bin support)
- [ ] Tournament progress tracking
- [ ] ELO/rating calculation from results
- [ ] Save tournament results to database

### Tournament UI

- [ ] Create tournament wizard (select engines, time control, opening book)
- [ ] Live game display during tournament
- [ ] Tournament standings/crosstable
- [ ] View individual games from tournament
- [ ] Export tournament PGN

---

## Phase 8: Live Game Integration

### Lichess Client (`internal/live/`)

- [ ] HTTP client with optional OAuth token
- [ ] Stream broadcast rounds (NDJSON)
- [ ] Fetch user's recent games
- [ ] Parse Lichess PGN format

### Chess.com Client

- [ ] Fetch monthly game archives
- [ ] Parse Chess.com PGN format

### Live UI

- [ ] Follow broadcast input (Lichess round URL)
- [ ] Display updating position
- [ ] Show live move list
- [ ] Analyze with engine while watching

---

## Phase 9: Advanced Features

### Opening Book (`internal/book/`)

- [ ] Polyglot .bin file reader
- [ ] Zobrist hashing for position lookup
- [ ] Weighted move selection
- [ ] Display book moves in UI

### Tablebase (`internal/tablebase/`)

- [ ] Syzygy WDL probing (optional, path-configured)
- [ ] Display tablebase result in engine panel
- [ ] Use in tournament adjudication

### SPRT Testing

- [ ] Implement SPRT calculation for engine testing
- [ ] Display SPRT status (LLR, bounds, result)
- [ ] Use in tournament mode for development testing

---

## Phase 10: Polish and Release

### Testing

- [ ] Unit test coverage for all internal packages
- [ ] Integration tests for engine lifecycle
- [ ] End-to-end tests for critical paths
- [ ] Manual testing checklist for releases

### Performance

- [ ] Profile and optimize PGN import speed
- [ ] Verify <1 second app startup
- [ ] Verify <500ms engine startup to first output
- [ ] Verify <100MB idle memory usage
- [ ] Verify <15MB binary size

### Documentation

- [ ] Update README with screenshots
- [ ] Document keyboard shortcuts
- [ ] Document registry format for contributors
- [ ] Write contributing guide

### Distribution

- [ ] GitHub Actions build workflow (Linux, Windows, macOS)
- [ ] Create GitHub releases with binaries
- [ ] Notarize macOS builds
- [ ] Code sign Windows builds (if feasible)

---

## Backlog (Post-MVP)

Ideas for future versions, not planned for initial release:

- [ ] Chess960/Fischer Random support
- [ ] Fairy-Stockfish integration for variants
- [ ] Cloud analysis (remote engine)
- [ ] Export analysis to Lichess study
- [ ] Opening book editor
- [ ] Analysis graph (score over time)
- [ ] Endgame training mode
- [ ] Engine development mode (I/O log viewer)
- [ ] Registry signature verification (GPG)
- [ ] Auto-update for engines
- [ ] Plugin system for custom analysis tools

---

## Notes

- Commit at logical milestones, not after every small change
- Run `make dev` frequently to verify nothing is broken
- Test with Stockfish as reference UCI implementation
- Keep frontend minimal; complex logic belongs in Go
