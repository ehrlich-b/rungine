package uci

import "time"

// EngineState represents the current state of a UCI engine.
type EngineState int

const (
	EngineStateNone EngineState = iota
	EngineStateStarting
	EngineStateReady
	EngineStateThinking
	EngineStatePondering
	EngineStateStopped
	EngineStateError
)

func (s EngineState) String() string {
	switch s {
	case EngineStateNone:
		return "none"
	case EngineStateStarting:
		return "starting"
	case EngineStateReady:
		return "ready"
	case EngineStateThinking:
		return "thinking"
	case EngineStatePondering:
		return "pondering"
	case EngineStateStopped:
		return "stopped"
	case EngineStateError:
		return "error"
	default:
		return "unknown"
	}
}

// UCIOptionType represents the type of a UCI option.
type UCIOptionType string

const (
	OptionTypeSpin   UCIOptionType = "spin"
	OptionTypeCheck  UCIOptionType = "check"
	OptionTypeCombo  UCIOptionType = "combo"
	OptionTypeString UCIOptionType = "string"
	OptionTypeButton UCIOptionType = "button"
)

// UCIOption represents a configurable engine option.
type UCIOption struct {
	Name    string
	Type    UCIOptionType
	Default string
	Min     *int
	Max     *int
	Vars    []string // For combo type
	Value   string   // Current value
}

// Score represents an engine evaluation score.
type Score struct {
	Centipawns *int
	Mate       *int
	LowerBound bool
	UpperBound bool
}

// IsMate returns true if the score is a mate score.
func (s Score) IsMate() bool {
	return s.Mate != nil
}

// String returns a human-readable representation of the score.
func (s Score) String() string {
	if s.Mate != nil {
		if *s.Mate > 0 {
			return "M" + itoa(*s.Mate)
		}
		return "-M" + itoa(-*s.Mate)
	}
	if s.Centipawns != nil {
		cp := *s.Centipawns
		sign := ""
		if cp > 0 {
			sign = "+"
		}
		return sign + ftoa(float64(cp)/100.0)
	}
	return "?"
}

// AnalysisInfo represents a single analysis update from the engine.
type AnalysisInfo struct {
	EngineID       string
	Depth          int
	SelDepth       int
	Score          Score
	Nodes          int64
	NPS            int64
	Time           time.Duration
	PV             []string // Principal variation in UCI notation
	MultiPV        int      // Which line (1-indexed, 0 if not multi-pv)
	CurrMove       string
	CurrMoveNumber int
	HashFull       int // Per mille
	TBHits         int64
	Timestamp      time.Time
}

// BestMove represents the engine's chosen move.
type BestMove struct {
	Move   string
	Ponder string
}

// EngineIdentity holds engine identification info.
type EngineIdentity struct {
	Name   string
	Author string
}

// GoParams specifies parameters for the "go" command.
type GoParams struct {
	Infinite  bool
	Depth     int
	Nodes     int64
	MoveTime  time.Duration
	WhiteTime time.Duration
	BlackTime time.Duration
	WhiteInc  time.Duration
	BlackInc  time.Duration
	MovesToGo int
	SearchMoves []string
	Ponder    bool
}

// helper functions to avoid fmt import for simple conversions
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func ftoa(f float64) string {
	if f == 0 {
		return "0.00"
	}
	neg := f < 0
	if neg {
		f = -f
	}
	// Round to 2 decimal places
	f = float64(int(f*100+0.5)) / 100
	whole := int(f)
	frac := int((f-float64(whole))*100 + 0.5)

	result := itoa(whole) + "."
	if frac < 10 {
		result += "0"
	}
	result += itoa(frac)
	if neg {
		result = "-" + result
	}
	return result
}
