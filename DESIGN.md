# Rungine Design Document

## Executive Summary

Rungine is a desktop chess application built on Wails v2 that serves as a universal UCI engine client. The core insight is that chess engines are commodity software - dozens of strong open-source engines exist - but the tooling to *use* them remains fragmented and user-hostile. Rungine makes engine installation one-click, multi-engine analysis seamless, and tournament running accessible.

**Target users:**
- **Chess players** who want to analyze games and positions with strong engines
- **Chess engine developers** who need to test their engines against others (gauntlet matches, SPRT testing, regression testing)
- **Tournament organizers** who want to run engine-vs-engine competitions

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Wails Application                         │
├─────────────────────────────────────────────────────────────────┤
│  Frontend (System WebView)                                       │
│  ├── Board Renderer (SVG)                                       │
│  ├── Move Tree / PGN Display                                    │
│  ├── Engine Output Panels                                       │
│  ├── Settings & Configuration UI                                │
│  └── Wails Bindings (auto-generated TypeScript)                 │
├─────────────────────────────────────────────────────────────────┤
│  Go Backend                                                      │
│  ├── UCI Engine Manager                                         │
│  │   ├── Process spawning and lifecycle                         │
│  │   ├── stdin/stdout goroutine pairs                           │
│  │   └── Graceful shutdown and crash recovery                   │
│  ├── Engine Registry                                            │
│  │   ├── TOML-based engine definitions                          │
│  │   ├── Platform/CPU-aware binary selection                    │
│  │   └── Download, verify, extract, validate                    │
│  ├── Chess Logic                                                │
│  │   ├── PGN parser/writer                                      │
│  │   ├── FEN parser/validator                                   │
│  │   └── Move legality (possibly via notnil/chess)              │
│  ├── Tournament Coordinator                                     │
│  │   ├── Pairing algorithms (Swiss, round-robin)                │
│  │   ├── Game arbitration                                       │
│  │   └── Result aggregation and ELO calculation                 │
│  ├── Game Database (SQLite)                                     │
│  │   ├── Game storage with metadata                             │
│  │   ├── Position indexing for search                           │
│  │   └── Import/export                                          │
│  └── Live Game Fetcher                                          │
│      ├── Lichess API client                                     │
│      └── Chess.com API client                                   │
├─────────────────────────────────────────────────────────────────┤
│  System                                                          │
│  ├── WebView2 (Windows) / WebKit (macOS) / WebKitGTK (Linux)   │
│  └── Native dialogs, menus, file system access                  │
└─────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────┐
│  External Processes (UCI Engines)                                │
│  ├── stockfish (subprocess, communicates via stdin/stdout)      │
│  ├── lc0 (subprocess + neural network files)                    │
│  └── ... any UCI-compatible engine                              │
└─────────────────────────────────────────────────────────────────┘
```

### Why Wails v2?

| Consideration | Wails | Electron | Tauri |
|--------------|-------|----------|-------|
| Binary size | ~10MB | ~300MB | ~5MB |
| Memory footprint | Low | High | Low |
| Language | Go | JS/Node | Rust |
| Process management | Excellent (goroutines) | Awkward | Good |
| Maturity | Good | Excellent | Good |
| Our familiarity | High (Go) | Medium | Low |

Go's goroutines and `os/exec` make concurrent engine management natural. A single binary with embedded assets simplifies distribution.

---

## Core Subsystems

### 1. UCI Engine Manager

The heart of Rungine. Manages chess engine processes.

#### Data Structures

```go
type Engine struct {
    ID          string              // Unique identifier
    Name        string              // Display name
    BinaryPath  string              // Path to executable
    Process     *exec.Cmd           // Running process (nil if stopped)
    stdin       io.WriteCloser      // Write commands here
    stdoutCh    chan string         // Parsed lines from stdout
    options     map[string]UCIOption // Discovered UCI options
    state       EngineState         // idle, thinking, pondering
    mu          sync.Mutex          // Protects writes to stdin
    ctx         context.Context     // For cancellation
    cancel      context.CancelFunc
}

type EngineState int
const (
    EngineIdle EngineState = iota
    EngineThinking
    EnginePondering
)

type UCIOption struct {
    Name    string
    Type    string      // "spin", "check", "combo", "string", "button"
    Default interface{}
    Min     *int        // For spin
    Max     *int        // For spin
    Vars    []string    // For combo
    Value   interface{} // Current value
}

type AnalysisInfo struct {
    EngineID  string
    Depth     int
    SelDepth  int
    Score     Score
    Nodes     int64
    NPS       int64
    Time      time.Duration
    PV        []string    // Principal variation (moves in UCI notation)
    MultiPV   int         // Which line (1-indexed)
    Timestamp time.Time
}

type Score struct {
    Centipawns *int  // Mutually exclusive with Mate
    Mate       *int  // Positive = white mates in N, negative = black
    LowerBound bool
    UpperBound bool
}
```

#### Engine Lifecycle

```
    ┌──────────────────────────────────────────────────────────┐
    │                                                          │
    ▼                                                          │
┌───────┐    Start()    ┌─────────┐   "uciok"   ┌──────────┐  │
│ None  │ ───────────▶  │ Starting │ ─────────▶ │  Ready   │  │
└───────┘               └─────────┘             └──────────┘  │
                             │                       │        │
                             │ timeout/crash         │ "go"   │
                             ▼                       ▼        │
                        ┌─────────┐            ┌──────────┐   │
                        │  Error  │            │ Thinking │   │
                        └─────────┘            └──────────┘   │
                             │                       │        │
                             │                       │"stop"/ │
                             │                       │bestmove│
                             │                       ▼        │
                             │                  ┌──────────┐  │
                             │                  │   Idle   │──┘
                             │                  └──────────┘
                             │                       │
                             │                       │ Stop()
                             ▼                       ▼
                        ┌─────────────────────────────────┐
                        │            Stopped              │
                        └─────────────────────────────────┘
```

#### Goroutine Model

Each running engine has two goroutines:

1. **Reader goroutine**: Continuously reads stdout, parses UCI output, sends to channel
2. **Main goroutine**: Sends commands via stdin (mutex-protected)

```go
func (e *Engine) readLoop() {
    scanner := bufio.NewScanner(e.stdout)
    for scanner.Scan() {
        line := scanner.Text()
        parsed := parseUCILine(line)
        select {
        case e.outputCh <- parsed:
        case <-e.ctx.Done():
            return
        }
    }
}
```

#### UCI Protocol Implementation

Commands we send:
- `uci` - Initialize, get engine identity and options
- `isready` - Synchronization ping
- `setoption name <name> value <value>` - Configure options
- `position fen <fen> moves <moves>` - Set position
- `position startpos moves <moves>` - Set position from start
- `go infinite` - Analyze forever
- `go depth <n>` - Analyze to depth
- `go movetime <ms>` - Analyze for time
- `go wtime <ms> btime <ms> winc <ms> binc <ms>` - Game clock
- `stop` - Stop thinking
- `quit` - Terminate engine

Responses we parse:
- `id name <name>` - Engine name
- `id author <author>` - Engine author
- `option name <name> type <type> ...` - Declare option
- `uciok` - UCI initialization complete
- `readyok` - Ready for commands
- `info depth <n> score cp <n> pv <moves>...` - Analysis info
- `bestmove <move> ponder <move>` - Best move found

#### Error Handling

Engines crash. Networks fail. We handle it:

```go
func (e *Engine) monitor() {
    err := e.Process.Wait()
    if err != nil && e.ctx.Err() == nil {
        // Unexpected crash, not intentional shutdown
        e.handleCrash(err)
    }
}

func (e *Engine) handleCrash(err error) {
    // Notify frontend
    runtime.EventsEmit(e.wailsCtx, "engine:crashed", EngineCrashEvent{
        EngineID: e.ID,
        Error:    err.Error(),
    })
    // Clean up resources
    e.cleanup()
}
```

---

### 2. Engine Registry System

The differentiating feature. Makes engine installation one-click.

#### Registry Format (TOML)

```toml
# Registry metadata
[meta]
version = "1.0.0"
updated = "2024-01-15"

# Engine definition
[engines.stockfish-17]
name = "Stockfish 17"
version = "17"
author = "Stockfish Team"
license = "GPL-3.0"
homepage = "https://stockfishchess.org"
description = "The strongest open-source chess engine"
elo_estimate = 3600
requires_network = false

# Platform-specific builds
# Key format: {os}-{arch}-{cpu_feature}
[engines.stockfish-17.builds.windows-amd64-avx2]
url = "https://github.com/official-stockfish/Stockfish/releases/download/sf_17/stockfish-windows-x86-64-avx2.zip"
sha256 = "abc123..."
binary = "stockfish/stockfish-windows-x86-64-avx2.exe"
extract = "zip"

[engines.stockfish-17.builds.windows-amd64-bmi2]
url = "https://github.com/official-stockfish/Stockfish/releases/download/sf_17/stockfish-windows-x86-64-bmi2.zip"
sha256 = "def456..."
binary = "stockfish/stockfish-windows-x86-64-bmi2.exe"
extract = "zip"

[engines.stockfish-17.builds.linux-amd64-avx2]
url = "https://github.com/official-stockfish/Stockfish/releases/download/sf_17/stockfish-ubuntu-x86-64-avx2.tar"
sha256 = "789abc..."
binary = "stockfish/stockfish-ubuntu-x86-64-avx2"
extract = "tar"

[engines.stockfish-17.builds.darwin-arm64]
url = "https://github.com/official-stockfish/Stockfish/releases/download/sf_17/stockfish-macos-m1-apple-silicon.tar"
sha256 = "fedcba..."
binary = "stockfish/stockfish-macos-m1-apple-silicon"
extract = "tar"

# UCI options documentation
[engines.stockfish-17.options.Hash]
type = "spin"
default = 16
min = 1
max = 33554432
description = "Hash table size in MB"
recommended = 256

[engines.stockfish-17.options.Threads]
type = "spin"
default = 1
min = 1
max = 1024
description = "Number of CPU threads to use"
recommended = "auto"  # Special value: detect CPU cores

[engines.stockfish-17.options.MultiPV]
type = "spin"
default = 1
min = 1
max = 500
description = "Number of principal variations to show"

[engines.stockfish-17.options.SyzygyPath]
type = "string"
default = ""
description = "Path to Syzygy tablebases"

# Pre-configured profiles
[engines.stockfish-17.profiles.analysis]
Hash = 1024
Threads = "auto"
MultiPV = 3

[engines.stockfish-17.profiles.tournament]
Hash = 512
Threads = "auto"
MultiPV = 1

[engines.stockfish-17.profiles.quick]
Hash = 128
Threads = 2
MultiPV = 1
```

#### CPU Feature Detection

Modern engines have builds optimized for specific CPU features. We detect and select automatically:

```go
import "github.com/klauspost/cpuid/v2"

type CPUFeatures struct {
    AVX512 bool
    AVX2   bool
    BMI2   bool
    POPCNT bool
    SSE42  bool
}

func detectCPUFeatures() CPUFeatures {
    return CPUFeatures{
        AVX512: cpuid.CPU.Supports(cpuid.AVX512F),
        AVX2:   cpuid.CPU.Supports(cpuid.AVX2),
        BMI2:   cpuid.CPU.Supports(cpuid.BMI2),
        POPCNT: cpuid.CPU.Supports(cpuid.POPCNT),
        SSE42:  cpuid.CPU.Supports(cpuid.SSE42),
    }
}

func selectBuild(engine EngineDefinition, features CPUFeatures) *Build {
    os := runtime.GOOS
    arch := runtime.GOARCH

    // Priority order: most optimized first
    candidates := []string{
        fmt.Sprintf("%s-%s-avx512", os, arch),
        fmt.Sprintf("%s-%s-bmi2", os, arch),
        fmt.Sprintf("%s-%s-avx2", os, arch),
        fmt.Sprintf("%s-%s-popcnt", os, arch),
        fmt.Sprintf("%s-%s", os, arch),  // Fallback
    }

    for _, key := range candidates {
        if build, ok := engine.Builds[key]; ok {
            if isCompatible(key, features) {
                return &build
            }
        }
    }
    return nil
}
```

#### Installation Flow

```
User clicks "Install Stockfish"
           │
           ▼
┌─────────────────────────┐
│  Detect OS/Arch/CPU     │
│  Select optimal build   │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Download binary        │──▶ Progress events to frontend
│  (with resume support)  │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Verify SHA256 hash     │──▶ Fail: delete, notify user
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Extract archive        │
│  (zip/tar/tar.gz)       │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Set executable perms   │    (Unix only)
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Validate: run "uci"    │──▶ Must receive "uciok" within 5s
│  and check for "uciok"  │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Save to local config   │
│  Engine ready to use    │
└─────────────────────────┘
```

#### Directory Structure

```
~/.rungine/
├── config.toml              # User preferences
├── engines/
│   ├── stockfish-17/
│   │   ├── stockfish.exe    # The binary
│   │   └── config.toml      # Per-engine overrides
│   ├── lc0-0.31/
│   │   ├── lc0.exe
│   │   ├── config.toml
│   │   └── networks/
│   │       ├── t82.pb.gz    # Neural network
│   │       └── maia-1900.pb.gz
│   └── berserk-12/
│       └── berserk.exe
├── games.db                 # SQLite database
├── registry-cache.toml      # Cached remote registry
└── tablebases/              # User-provided Syzygy files
    └── (symlink or actual files)
```

---

### 3. PGN Parser

PGN (Portable Game Notation) is the standard interchange format. Our parser must handle:
- Tag pairs: `[Event "World Championship"]`
- Movetext with variations: `1. e4 e5 (1... c5 2. Nf3) 2. Nf3`
- Comments: `{This is a comment}`
- NAG symbols: `!`, `?`, `!!`, `??`, `!?`, `?!`, `$1`-`$255`
- Recursive variations (unlimited depth)
- Game termination markers: `1-0`, `0-1`, `1/2-1/2`, `*`

#### Data Structures

```go
type Game struct {
    Tags      map[string]string  // PGN tag pairs
    Moves     *MoveNode          // Root of move tree
    Result    string             // "1-0", "0-1", "1/2-1/2", "*"
}

type MoveNode struct {
    Move       string       // SAN notation: "e4", "Nxf7+", "O-O-O"
    Comment    string       // Text annotation
    NAGs       []int        // Numeric Annotation Glyphs
    Variations []*MoveNode  // Alternative continuations
    Next       *MoveNode    // Main line continuation
    Parent     *MoveNode    // For navigation
    Ply        int          // Half-move number (0 = before first move)
}
```

#### Parser Strategy

Two-phase parsing:
1. **Tokenization**: Break input into tokens (tags, moves, comments, NAGs, parens)
2. **Tree construction**: Build move tree respecting variation nesting

```go
type TokenType int
const (
    TokenTag TokenType = iota      // [Event "..."]
    TokenMove                      // e4, Nf3, O-O
    TokenComment                   // {text}
    TokenNAG                       // $1, !, ?
    TokenVariationStart            // (
    TokenVariationEnd              // )
    TokenResult                    // 1-0, 0-1, 1/2-1/2, *
    TokenMoveNumber                // 1., 1...
)

type Token struct {
    Type  TokenType
    Value string
    Line  int  // For error reporting
    Col   int
}
```

Variation handling uses a stack:
```go
func buildMoveTree(tokens []Token) *MoveNode {
    root := &MoveNode{}
    current := root
    var stack []*MoveNode  // For variation nesting

    for _, tok := range tokens {
        switch tok.Type {
        case TokenMove:
            node := &MoveNode{Move: tok.Value, Parent: current}
            current.Next = node
            current = node

        case TokenVariationStart:
            stack = append(stack, current)
            // Variation branches from current's parent
            current = current.Parent

        case TokenVariationEnd:
            current = stack[len(stack)-1]
            stack = stack[:len(stack)-1]

        case TokenComment:
            current.Comment = tok.Value

        case TokenNAG:
            current.NAGs = append(current.NAGs, parseNAG(tok.Value))
        }
    }
    return root
}
```

---

### 4. Tournament System

Run engine-vs-engine matches with proper arbitration.

#### Tournament Types

**Round Robin**: Every engine plays every other engine
- `n` engines → `n*(n-1)/2` pairings
- Each pairing plays 2 games (alternate colors)
- Total games: `n*(n-1)`

**Swiss**: Pair by score, good for many engines
- Configurable number of rounds
- Same-score players paired
- No repeat pairings

**Gauntlet**: One engine vs all others
- Test a single engine against the field
- Useful for development/tuning

#### Game Arbitration

```go
type Arbiter struct {
    game          *GameState
    whiteEngine   *Engine
    blackEngine   *Engine
    timeControl   TimeControl
    adjudication  AdjudicationRules
}

type TimeControl struct {
    Initial   time.Duration  // Starting time
    Increment time.Duration  // Added per move
    Moves     int            // Moves in time control (0 = sudden death)
}

type AdjudicationRules struct {
    // Resign threshold
    ResignScore     int  // Centipawns (e.g., -1000)
    ResignMoves     int  // Consecutive moves below threshold

    // Draw adjudication
    DrawScore       int  // e.g., 10 cp
    DrawMoves       int  // e.g., 10 consecutive moves

    // Tablebase adjudication
    UseTablebases   bool
    TablebasePath   string

    // Rule-based draws
    FiftyMoveRule   bool
    ThreefoldRep    bool
    InsufficientMat bool
}
```

#### Game Flow

```
┌────────────────────────────────────────────────────────────────┐
│                      Tournament Game                            │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────┐         ┌─────────┐         ┌─────────┐          │
│  │ White   │◀───────▶│ Arbiter │◀───────▶│  Black  │          │
│  │ Engine  │  move   │         │  move   │ Engine  │          │
│  └─────────┘         └─────────┘         └─────────┘          │
│       │                   │                   │                │
│       │   position cmd    │                   │                │
│       │◀──────────────────│                   │                │
│       │                   │                   │                │
│       │   go wtime/btime  │                   │                │
│       │◀──────────────────│                   │                │
│       │                   │                   │                │
│       │   bestmove e2e4   │                   │                │
│       │──────────────────▶│                   │                │
│       │                   │                   │                │
│       │                   │   position cmd    │                │
│       │                   │──────────────────▶│                │
│       │                   │                   │                │
│       │                   │   go wtime/btime  │                │
│       │                   │──────────────────▶│                │
│       │                   │                   │                │
│       │                   │   bestmove e7e5   │                │
│       │                   │◀──────────────────│                │
│       │                   │                   │                │
│                    ... continues ...                            │
│                                                                 │
│  Termination conditions checked after each move:               │
│  - Checkmate                                                   │
│  - Stalemate                                                   │
│  - Time forfeit                                                │
│  - Resign threshold                                            │
│  - Draw adjudication                                           │
│  - Tablebase adjudication                                      │
│  - 50-move rule                                                │
│  - Threefold repetition                                        │
│  - Insufficient material                                       │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

#### Opening Books

Support Polyglot `.bin` format for opening variety:

```go
type PolyglotEntry struct {
    Key    uint64  // Zobrist hash of position
    Move   uint16  // Encoded move
    Weight uint16  // Selection weight
    Learn  uint32  // Learning data (ignored)
}

func (b *Book) GetMoves(position *Position) []BookMove {
    key := position.ZobristHash()
    // Binary search for key in sorted book
    entries := b.lookup(key)

    // Convert to weighted selection
    var moves []BookMove
    var totalWeight int
    for _, e := range entries {
        moves = append(moves, BookMove{
            Move:   decodePolyglotMove(e.Move),
            Weight: int(e.Weight),
        })
        totalWeight += int(e.Weight)
    }

    // Normalize weights to probabilities
    for i := range moves {
        moves[i].Probability = float64(moves[i].Weight) / float64(totalWeight)
    }
    return moves
}
```

#### ELO Calculation

Use Bayesian ELO or Ordo-style calculation:

```go
// Simple ELO update (can be replaced with Bayesian)
func calculateELO(games []GameResult) map[string]float64 {
    ratings := make(map[string]float64)
    K := 32.0  // K-factor

    // Initialize all players at 1500
    for _, g := range games {
        if _, ok := ratings[g.White]; !ok {
            ratings[g.White] = 1500
        }
        if _, ok := ratings[g.Black]; !ok {
            ratings[g.Black] = 1500
        }
    }

    // Iterate to convergence
    for i := 0; i < 100; i++ {
        for _, g := range games {
            rW := ratings[g.White]
            rB := ratings[g.Black]

            // Expected scores
            eW := 1 / (1 + math.Pow(10, (rB-rW)/400))
            eB := 1 - eW

            // Actual scores
            var sW, sB float64
            switch g.Result {
            case "1-0":
                sW, sB = 1, 0
            case "0-1":
                sW, sB = 0, 1
            case "1/2-1/2":
                sW, sB = 0.5, 0.5
            }

            // Update
            ratings[g.White] += K * (sW - eW)
            ratings[g.Black] += K * (sB - eB)
        }
    }
    return ratings
}
```

#### SPRT (Sequential Probability Ratio Test)

For engine developers testing patches (e.g., "is my new eval term an improvement?"), implement SPRT:

```go
// SPRT determines if we have enough evidence to accept/reject a hypothesis
// about ELO difference. Used by Fishtest and other serious testing frameworks.
type SPRTResult int
const (
    SPRTContinue SPRTResult = iota  // Need more games
    SPRTAcceptH1                     // New version is stronger
    SPRTAcceptH0                     // No improvement (reject patch)
)

type SPRTConfig struct {
    ELO0  float64  // Null hypothesis: true ELO diff is this (e.g., 0)
    ELO1  float64  // Alt hypothesis: true ELO diff is this (e.g., 5)
    Alpha float64  // Type I error rate (e.g., 0.05)
    Beta  float64  // Type II error rate (e.g., 0.05)
}

func (s *SPRTConfig) Test(wins, losses, draws int) SPRTResult {
    // Log-likelihood ratio calculation
    // LLR = sum of log(P(result|H1) / P(result|H0)) for each game

    n := float64(wins + losses + draws)
    if n == 0 {
        return SPRTContinue
    }

    // Convert ELO difference to expected score
    p0 := 1 / (1 + math.Pow(10, -s.ELO0/400))
    p1 := 1 / (1 + math.Pow(10, -s.ELO1/400))

    // LLR using trinomial model (simplified)
    llr := float64(wins)*math.Log(p1/p0) +
           float64(losses)*math.Log((1-p1)/(1-p0)) +
           float64(draws)*math.Log((0.5)/(0.5))  // Draws don't discriminate

    // Wald boundaries
    lowerBound := math.Log(s.Beta / (1 - s.Alpha))
    upperBound := math.Log((1 - s.Beta) / s.Alpha)

    if llr >= upperBound {
        return SPRTAcceptH1  // Patch is stronger
    }
    if llr <= lowerBound {
        return SPRTAcceptH0  // Patch is not stronger
    }
    return SPRTContinue  // Keep testing
}
```

SPRT advantages over fixed-game-count testing:
- Stops early when result is clear (saves compute)
- Provides statistical guarantees on error rates
- Standard in engine development (Fishtest uses SPRT with ELO0=0, ELO1=2 for non-regression)

---

### 5. Game Database

SQLite-based storage for games, with position indexing for search.

#### Schema

```sql
-- Core game storage
CREATE TABLE games (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Standard PGN tags
    event TEXT,
    site TEXT,
    date TEXT,              -- PGN date format: "2024.01.15" or "????.??.??"
    round TEXT,
    white TEXT NOT NULL,
    black TEXT NOT NULL,
    result TEXT NOT NULL,   -- "1-0", "0-1", "1/2-1/2", "*"

    -- Optional tags
    white_elo INTEGER,
    black_elo INTEGER,
    eco TEXT,               -- ECO opening code
    opening TEXT,           -- Opening name
    time_control TEXT,
    termination TEXT,       -- "Normal", "Time forfeit", etc.

    -- Full PGN (for faithful round-trip)
    pgn TEXT NOT NULL,

    -- Metadata
    source TEXT,            -- "import", "lichess", "chesscom", "tournament"
    source_id TEXT,         -- External ID (e.g., Lichess game ID)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Position index for "find games with this position"
CREATE TABLE positions (
    game_id INTEGER NOT NULL,
    ply INTEGER NOT NULL,           -- Half-move number
    fen_hash INTEGER NOT NULL,      -- 64-bit hash of position
    PRIMARY KEY (game_id, ply),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE
);

CREATE INDEX idx_positions_hash ON positions(fen_hash);

-- Full-text search on player names and events
CREATE VIRTUAL TABLE games_fts USING fts5(
    white, black, event, site, opening,
    content='games',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER games_ai AFTER INSERT ON games BEGIN
    INSERT INTO games_fts(rowid, white, black, event, site, opening)
    VALUES (new.id, new.white, new.black, new.event, new.site, new.opening);
END;

CREATE TRIGGER games_ad AFTER DELETE ON games BEGIN
    INSERT INTO games_fts(games_fts, rowid, white, black, event, site, opening)
    VALUES ('delete', old.id, old.white, old.black, old.event, old.site, old.opening);
END;

-- Tags for organization
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL
);

CREATE TABLE game_tags (
    game_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (game_id, tag_id),
    FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);
```

#### Position Hashing

Use Zobrist hashing for position lookup:

```go
// Pre-generated random numbers for Zobrist hashing
var zobristPiece [12][64]uint64    // [piece][square]
var zobristCastling [16]uint64     // [castling rights bitmask]
var zobristEnPassant [8]uint64     // [file]
var zobristBlackToMove uint64

func init() {
    rng := rand.New(rand.NewSource(0x12345678))  // Fixed seed for consistency
    for p := 0; p < 12; p++ {
        for sq := 0; sq < 64; sq++ {
            zobristPiece[p][sq] = rng.Uint64()
        }
    }
    // ... etc
}

func (p *Position) ZobristHash() uint64 {
    var h uint64

    // XOR in pieces
    for sq := 0; sq < 64; sq++ {
        if piece := p.board[sq]; piece != NoPiece {
            h ^= zobristPiece[piece][sq]
        }
    }

    // Castling rights
    h ^= zobristCastling[p.castling]

    // En passant
    if p.enPassant != NoSquare {
        h ^= zobristEnPassant[p.enPassant.File()]
    }

    // Side to move
    if p.sideToMove == Black {
        h ^= zobristBlackToMove
    }

    return h
}
```

#### Batch Import

For importing large PGN files:

```go
func (db *Database) ImportPGN(r io.Reader, progressFn func(int)) error {
    tx, _ := db.conn.Begin()
    defer tx.Rollback()

    stmtGame, _ := tx.Prepare(`
        INSERT INTO games (event, site, date, round, white, black, result,
                          white_elo, black_elo, eco, pgn, source)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'import')
    `)
    stmtPos, _ := tx.Prepare(`
        INSERT INTO positions (game_id, ply, fen_hash) VALUES (?, ?, ?)
    `)

    parser := pgn.NewParser(r)
    count := 0

    for {
        game, err := parser.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            continue  // Skip malformed games
        }

        result, _ := stmtGame.Exec(
            game.Tags["Event"], game.Tags["Site"], game.Tags["Date"],
            game.Tags["Round"], game.Tags["White"], game.Tags["Black"],
            game.Result, /* ... */)

        gameID, _ := result.LastInsertId()

        // Index positions
        pos := chess.StartingPosition()
        stmtPos.Exec(gameID, 0, pos.ZobristHash())

        for ply, move := range game.Moves.MainLine() {
            pos = pos.Apply(move)
            stmtPos.Exec(gameID, ply+1, pos.ZobristHash())
        }

        count++
        if count%1000 == 0 {
            progressFn(count)
        }
    }

    return tx.Commit()
}
```

---

### 6. Live Game Integration

#### Lichess API

Lichess has excellent API support:

```go
type LichessClient struct {
    httpClient *http.Client
    token      string  // Optional OAuth token
    baseURL    string
}

// Stream a broadcast
func (c *LichessClient) StreamBroadcast(ctx context.Context, roundID string) (<-chan BroadcastUpdate, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET",
        fmt.Sprintf("%s/api/broadcast/-/-/%s", c.baseURL, roundID), nil)
    req.Header.Set("Accept", "application/x-ndjson")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }

    ch := make(chan BroadcastUpdate)
    go func() {
        defer resp.Body.Close()
        defer close(ch)

        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            var update BroadcastUpdate
            json.Unmarshal(scanner.Bytes(), &update)
            select {
            case ch <- update:
            case <-ctx.Done():
                return
            }
        }
    }()

    return ch, nil
}

// Fetch user's recent games
func (c *LichessClient) GetUserGames(username string, max int) ([]Game, error) {
    resp, err := c.httpClient.Get(
        fmt.Sprintf("%s/api/games/user/%s?max=%d&pgnInJson=true",
            c.baseURL, username, max))
    // ... parse NDJSON response
}
```

#### Chess.com API

More limited but useful:

```go
type ChessComClient struct {
    httpClient *http.Client
    baseURL    string
}

// Get monthly archives list
func (c *ChessComClient) GetArchives(username string) ([]string, error) {
    resp, _ := c.httpClient.Get(
        fmt.Sprintf("%s/pub/player/%s/games/archives", c.baseURL, username))
    // Returns list of monthly archive URLs
}

// Get games from a monthly archive
func (c *ChessComClient) GetMonthlyGames(archiveURL string) ([]Game, error) {
    resp, _ := c.httpClient.Get(archiveURL)
    // Returns PGN games for that month
}
```

---

### 7. Frontend Architecture

#### Wails Bindings

Go functions exposed to frontend:

```go
// app.go - Main application struct bound to Wails
type App struct {
    ctx           context.Context
    engineManager *uci.Manager
    database      *database.Database
    registry      *registry.Registry
}

// Engine operations
func (a *App) ListEngines() []EngineInfo { ... }
func (a *App) InstallEngine(id string) error { ... }
func (a *App) StartEngine(id string) error { ... }
func (a *App) StopEngine(id string) error { ... }
func (a *App) SetEngineOption(engineID, option, value string) error { ... }

// Analysis
func (a *App) StartAnalysis(fen string, engineIDs []string) error { ... }
func (a *App) StopAnalysis() error { ... }

// Game operations
func (a *App) LoadPGN(pgn string) (*GameInfo, error) { ... }
func (a *App) GetPosition(ply int) (*PositionInfo, error) { ... }
func (a *App) MakeMove(from, to string) (*MoveResult, error) { ... }

// Database
func (a *App) ImportPGN(path string) (int, error) { ... }
func (a *App) SearchGames(query SearchQuery) ([]GameSummary, error) { ... }

// Live
func (a *App) FollowLichessBroadcast(roundID string) error { ... }
```

#### Event Streaming

Backend → Frontend via Wails events:

```go
// Emit analysis updates
runtime.EventsEmit(ctx, "analysis:info", AnalysisInfo{
    EngineID:  engine.ID,
    Depth:     15,
    Score:     Score{Centipawns: ptr(45)},
    PV:        []string{"e2e4", "e7e5", "g1f3"},
    Nodes:     15000000,
    NPS:       2500000,
})

// Emit download progress
runtime.EventsEmit(ctx, "download:progress", DownloadProgress{
    EngineID:   "stockfish-17",
    Downloaded: 5000000,
    Total:      10000000,
    Percent:    50,
})
```

Frontend listens:
```typescript
import { EventsOn } from '../wailsjs/runtime';

EventsOn('analysis:info', (info: AnalysisInfo) => {
    updateAnalysisDisplay(info);
});
```

#### Component Structure

```
frontend/src/
├── main.ts                 # Entry point
├── App.svelte              # Root component (or .tsx/.vue)
├── components/
│   ├── Board/
│   │   ├── Board.svelte    # Chess board
│   │   ├── Piece.svelte    # Single piece (SVG)
│   │   └── Square.svelte   # Board square
│   ├── MoveList/
│   │   ├── MoveList.svelte # PGN move display
│   │   └── MoveNode.svelte # Single move with variations
│   ├── Engine/
│   │   ├── EnginePanel.svelte    # Analysis output
│   │   ├── EngineSelector.svelte # Engine dropdown
│   │   └── EngineOptions.svelte  # UCI options editor
│   ├── Database/
│   │   ├── GameList.svelte       # Search results
│   │   └── GameFilters.svelte    # Search form
│   └── Settings/
│       └── Settings.svelte       # App configuration
├── stores/
│   ├── game.ts             # Current game state
│   ├── engines.ts          # Engine list and status
│   └── settings.ts         # User preferences
├── lib/
│   ├── chess.ts            # Board logic utilities
│   └── pgn.ts              # PGN display formatting
└── styles/
    ├── global.css
    ├── board.css
    └── themes/
        ├── dark.css
        └── light.css
```

---

## Data Flow Examples

### Example 1: User Starts Analysis

```
User clicks "Analyze"
        │
        ▼
┌───────────────────┐
│ Frontend: Board   │
│ component calls   │
│ StartAnalysis()   │
└─────────┬─────────┘
          │ Wails binding call
          ▼
┌───────────────────┐
│ Go: App.Start     │
│ Analysis(fen,     │
│ ["stockfish"])    │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: EngineManager │
│ sends "position"  │
│ then "go infinite"│
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Stockfish process │
│ writes to stdout: │
│ "info depth 15..."│
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: Engine reader │
│ goroutine parses  │
│ UCI info line     │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: EventsEmit    │
│ "analysis:info"   │
└─────────┬─────────┘
          │ Wails event
          ▼
┌───────────────────┐
│ Frontend: updates │
│ EnginePanel with  │
│ depth, score, PV  │
└───────────────────┘
```

### Example 2: User Installs Engine

```
User clicks "Install Stockfish"
        │
        ▼
┌───────────────────┐
│ Frontend calls    │
│ InstallEngine     │
│ ("stockfish-17")  │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: Registry      │
│ - Load engine def │
│ - Detect CPU      │
│ - Select build    │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: Downloader    │
│ - GET binary URL  │◀──┐
│ - Stream to file  │   │ EventsEmit("download:progress")
└─────────┬─────────┘───┘
          │
          ▼
┌───────────────────┐
│ Go: Verify SHA256 │──▶ Fail? Delete + error
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: Extract       │
│ (zip/tar)         │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: Validate      │
│ - Run "uci"       │
│ - Wait "uciok"    │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Go: Save config   │
│ Return success    │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ Frontend: refresh │
│ engine list       │
└───────────────────┘
```

---

## Error Handling Strategy

### Categories

1. **User errors**: Invalid input, unsupported format
   - Return clear error message, don't crash

2. **Engine errors**: Crash, timeout, invalid output
   - Detect via process exit or timeout
   - Notify user, offer restart
   - Don't corrupt game state

3. **Network errors**: Download failure, API errors
   - Retry with backoff
   - Resume partial downloads
   - Cache aggressively

4. **System errors**: Disk full, permission denied
   - Surface to user with actionable message
   - Don't silently fail

### Error Types (Go)

```go
type RungineError struct {
    Code    ErrorCode
    Message string
    Cause   error
}

type ErrorCode int
const (
    ErrInvalidPGN ErrorCode = iota
    ErrInvalidFEN
    ErrEngineCrash
    ErrEngineTimeout
    ErrDownloadFailed
    ErrHashMismatch
    ErrDatabaseError
    ErrNetworkError
)

func (e *RungineError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Cause)
    }
    return e.Message
}
```

---

## Security Considerations

1. **Engine binaries**: Only download from registry URLs, verify SHA256
2. **User PGN input**: Sanitize before display, no script execution
3. **API tokens**: Store securely (OS keychain if possible), never log
4. **File paths**: Validate user-provided paths, prevent directory traversal
5. **Process spawning**: Only execute known engine binaries

---

## Performance Optimization

### Startup Time
- Lazy-load engine definitions
- Don't validate engines until used
- SQLite connection pooling

### Analysis Updates
- Throttle UI updates to 10-20Hz (every 50-100ms)
- Don't re-render entire board on every info line
- Batch position hash lookups in database

### PGN Import
- Use prepared statements
- Batch inserts in transactions (1000 games per tx)
- Index after bulk insert, not during

### Memory
- Stream large PGN files, don't load entirely
- Limit engine output buffer sizes
- Clean up finished engine processes promptly

---

## Testing Strategy

### Unit Tests

```go
// uci/protocol_test.go
func TestParseInfoLine(t *testing.T) {
    tests := []struct {
        input    string
        expected AnalysisInfo
    }{
        {
            input: "info depth 20 seldepth 25 score cp 35 nodes 1500000 nps 2500000 pv e2e4 e7e5 g1f3",
            expected: AnalysisInfo{
                Depth: 20, SelDepth: 25,
                Score: Score{Centipawns: ptr(35)},
                Nodes: 1500000, NPS: 2500000,
                PV: []string{"e2e4", "e7e5", "g1f3"},
            },
        },
        {
            input: "info depth 30 score mate 5 pv d8h4 g2g3 h4g3",
            expected: AnalysisInfo{
                Depth: 30,
                Score: Score{Mate: ptr(5)},
                PV: []string{"d8h4", "g2g3", "h4g3"},
            },
        },
    }

    for _, tc := range tests {
        t.Run(tc.input, func(t *testing.T) {
            got := parseInfoLine(tc.input)
            // assertions...
        })
    }
}
```

### Integration Tests

```go
// uci/engine_integration_test.go
func TestStockfishLifecycle(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    sfPath := os.Getenv("STOCKFISH_PATH")
    if sfPath == "" {
        t.Skip("STOCKFISH_PATH not set")
    }

    engine, err := NewEngine(sfPath)
    require.NoError(t, err)

    err = engine.Start(context.Background())
    require.NoError(t, err)
    defer engine.Stop()

    // Should receive UCI options
    require.NotEmpty(t, engine.Options())

    // Set position and analyze
    engine.SetPosition("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1")

    infoCh := engine.Go(GoParams{Depth: 10})

    var lastInfo AnalysisInfo
    for info := range infoCh {
        lastInfo = info
    }

    require.GreaterOrEqual(t, lastInfo.Depth, 10)
    require.NotEmpty(t, lastInfo.PV)
}
```

---

## Future Considerations

### Potential Extensions (Not in MVP)
- Chess960/Fischer Random support
- Other variants (via Fairy-Stockfish)
- Cloud engine analysis (remote Stockfish instances)
- Analysis sharing (export to Lichess study)
- Opening book editor
- Neural network training integration

### Technical Debt to Avoid
- Don't bake in assumptions about UCI protocol versions
- Keep frontend framework-agnostic where possible
- Document all registry format decisions for future changes
- Plan for registry signature verification (GPG or similar)

---

## Appendix: UCI Protocol Reference

### Minimal UCI Conversation

```
GUI -> Engine: uci
Engine -> GUI: id name Stockfish 17
Engine -> GUI: id author the Stockfish developers
Engine -> GUI: option name Hash type spin default 16 min 1 max 33554432
Engine -> GUI: option name Threads type spin default 1 min 1 max 1024
Engine -> GUI: uciok

GUI -> Engine: isready
Engine -> GUI: readyok

GUI -> Engine: position startpos moves e2e4 e7e5
GUI -> Engine: go depth 20

Engine -> GUI: info depth 1 score cp 50 nodes 20 pv d2d4
Engine -> GUI: info depth 2 score cp 40 nodes 150 pv g1f3 b8c6
... (more info lines)
Engine -> GUI: info depth 20 score cp 35 nodes 15000000 nps 2500000 pv g1f3 b8c6 ...
Engine -> GUI: bestmove g1f3 ponder b8c6

GUI -> Engine: quit
```

### Info Line Fields

| Field | Description |
|-------|-------------|
| `depth N` | Search depth in plies |
| `seldepth N` | Selective search depth |
| `score cp N` | Score in centipawns from engine's POV |
| `score mate N` | Mate in N moves (negative = being mated) |
| `score lowerbound` | Score is a lower bound |
| `score upperbound` | Score is an upper bound |
| `nodes N` | Nodes searched |
| `nps N` | Nodes per second |
| `time N` | Time searched in ms |
| `pv move1 move2...` | Principal variation |
| `multipv N` | Which line (for MultiPV) |
| `currmove move` | Currently searching this move |
| `currmovenumber N` | Move number being searched |
| `hashfull N` | Hash table fill (permill) |
| `tbhits N` | Tablebase hits |

---

## Appendix: Engine Registry Complete Example

See `registry/engines.toml` in repository for full examples.

---

*Document version: 1.0*
*Last updated: 2024-01-15*
