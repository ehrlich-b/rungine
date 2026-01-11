# Rungine

A desktop chess application for running, analyzing, and managing UCI chess engines.

## What is this?

Rungine is a UCI omni-client. Chess engines (Stockfish, Leela Chess Zero, etc.) speak the Universal Chess Interface protocol - Rungine makes it trivial to install, configure, and use any of them.

**Key features:**
- One-click engine installation from curated registry
- Multi-engine simultaneous analysis
- Game viewer with full PGN support
- Engine-vs-engine tournaments
- Live game following (Lichess, Chess.com)
- Local game database with search

## Installation

### Pre-built binaries

Download from [Releases](https://github.com/ehrlich-b/rungine/releases).

### Build from source

Requirements:
- Go 1.21+
- Node.js 18+ (for frontend build)
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

```bash
git clone https://github.com/ehrlich-b/rungine.git
cd rungine
wails build
```

Binary will be in `build/bin/`.

## Quick Start

1. Launch Rungine
2. Go to Engine Library → Install Stockfish (one click)
3. Load a PGN or paste a FEN
4. Click Analyze

## Development

```bash
# Live development mode (hot reload)
wails dev

# Run tests
go test ./...

# Build for production
wails build
```

See [DESIGN.md](DESIGN.md) for architecture details.

## Project Status

Early development. See TODO.md for current progress.

## License

MIT

## Acknowledgments

- [Stockfish](https://stockfishchess.org/) and all open-source engine authors
- [Lichess](https://lichess.org/) for API access and inspiration
- [Wails](https://wails.io/) for the framework
