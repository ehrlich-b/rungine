package uci

import (
	"strconv"
	"strings"
	"time"
)

// ParsedLine represents a parsed UCI response line.
type ParsedLine struct {
	Type string
	Data any
}

// ParseLine parses a single UCI output line from an engine.
func ParseLine(line string) ParsedLine {
	line = strings.TrimSpace(line)
	if line == "" {
		return ParsedLine{Type: "empty"}
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ParsedLine{Type: "empty"}
	}

	switch parts[0] {
	case "id":
		return parseIDLine(parts[1:])
	case "uciok":
		return ParsedLine{Type: "uciok"}
	case "readyok":
		return ParsedLine{Type: "readyok"}
	case "bestmove":
		return parseBestMoveLine(parts[1:])
	case "info":
		return parseInfoLine(parts[1:])
	case "option":
		return parseOptionLine(parts[1:])
	default:
		return ParsedLine{Type: "unknown", Data: line}
	}
}

func parseIDLine(parts []string) ParsedLine {
	if len(parts) < 2 {
		return ParsedLine{Type: "id"}
	}

	switch parts[0] {
	case "name":
		return ParsedLine{
			Type: "id_name",
			Data: strings.Join(parts[1:], " "),
		}
	case "author":
		return ParsedLine{
			Type: "id_author",
			Data: strings.Join(parts[1:], " "),
		}
	default:
		return ParsedLine{Type: "id"}
	}
}

func parseBestMoveLine(parts []string) ParsedLine {
	bm := BestMove{}
	if len(parts) >= 1 {
		bm.Move = parts[0]
	}
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "ponder" && i+1 < len(parts) {
			bm.Ponder = parts[i+1]
			break
		}
	}
	return ParsedLine{Type: "bestmove", Data: bm}
}

func parseInfoLine(parts []string) ParsedLine {
	info := AnalysisInfo{
		Timestamp: time.Now(),
	}

	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "depth":
			if i+1 < len(parts) {
				info.Depth, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "seldepth":
			if i+1 < len(parts) {
				info.SelDepth, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "multipv":
			if i+1 < len(parts) {
				info.MultiPV, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "score":
			i = parseScore(parts, i+1, &info.Score)
		case "nodes":
			if i+1 < len(parts) {
				info.Nodes, _ = strconv.ParseInt(parts[i+1], 10, 64)
				i++
			}
		case "nps":
			if i+1 < len(parts) {
				info.NPS, _ = strconv.ParseInt(parts[i+1], 10, 64)
				i++
			}
		case "time":
			if i+1 < len(parts) {
				ms, _ := strconv.ParseInt(parts[i+1], 10, 64)
				info.Time = time.Duration(ms) * time.Millisecond
				i++
			}
		case "pv":
			// PV continues to end of line
			info.PV = parts[i+1:]
			i = len(parts) // Exit loop
		case "currmove":
			if i+1 < len(parts) {
				info.CurrMove = parts[i+1]
				i++
			}
		case "currmovenumber":
			if i+1 < len(parts) {
				info.CurrMoveNumber, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "hashfull":
			if i+1 < len(parts) {
				info.HashFull, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "tbhits":
			if i+1 < len(parts) {
				info.TBHits, _ = strconv.ParseInt(parts[i+1], 10, 64)
				i++
			}
		case "string":
			// Rest of line is string output, ignore for now
			i = len(parts)
		}
	}

	return ParsedLine{Type: "info", Data: info}
}

func parseScore(parts []string, start int, score *Score) int {
	i := start
	for i < len(parts) {
		switch parts[i] {
		case "cp":
			if i+1 < len(parts) {
				cp, _ := strconv.Atoi(parts[i+1])
				score.Centipawns = &cp
				i++
			}
		case "mate":
			if i+1 < len(parts) {
				mate, _ := strconv.Atoi(parts[i+1])
				score.Mate = &mate
				i++
			}
		case "lowerbound":
			score.LowerBound = true
		case "upperbound":
			score.UpperBound = true
		default:
			// Unknown token, end of score section
			return i - 1
		}
		i++
	}
	return i - 1
}

func parseOptionLine(parts []string) ParsedLine {
	opt := UCIOption{}

	// Parse: name <name> type <type> [default <default>] [min <min>] [max <max>] [var <var>...]
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "name":
			// Collect name until we hit "type"
			nameEnd := i + 1
			for nameEnd < len(parts) && parts[nameEnd] != "type" {
				nameEnd++
			}
			opt.Name = strings.Join(parts[i+1:nameEnd], " ")
			i = nameEnd - 1
		case "type":
			if i+1 < len(parts) {
				opt.Type = UCIOptionType(parts[i+1])
				i++
			}
		case "default":
			// Collect default value - for string type it might be multi-word
			if opt.Type == OptionTypeString {
				// String default goes to end or next keyword
				defEnd := i + 1
				for defEnd < len(parts) && !isOptionKeyword(parts[defEnd]) {
					defEnd++
				}
				if defEnd > i+1 {
					opt.Default = strings.Join(parts[i+1:defEnd], " ")
				}
				i = defEnd - 1
			} else if i+1 < len(parts) {
				opt.Default = parts[i+1]
				i++
			}
		case "min":
			if i+1 < len(parts) {
				min, _ := strconv.Atoi(parts[i+1])
				opt.Min = &min
				i++
			}
		case "max":
			if i+1 < len(parts) {
				max, _ := strconv.Atoi(parts[i+1])
				opt.Max = &max
				i++
			}
		case "var":
			if i+1 < len(parts) {
				opt.Vars = append(opt.Vars, parts[i+1])
				i++
			}
		}
	}

	opt.Value = opt.Default
	return ParsedLine{Type: "option", Data: opt}
}

func isOptionKeyword(s string) bool {
	switch s {
	case "name", "type", "default", "min", "max", "var":
		return true
	}
	return false
}

// BuildGoCommand constructs a "go" command string from GoParams.
func BuildGoCommand(p GoParams) string {
	var parts []string
	parts = append(parts, "go")

	if p.Infinite {
		parts = append(parts, "infinite")
		return strings.Join(parts, " ")
	}

	if p.Ponder {
		parts = append(parts, "ponder")
	}

	if p.Depth > 0 {
		parts = append(parts, "depth", strconv.Itoa(p.Depth))
	}

	if p.Nodes > 0 {
		parts = append(parts, "nodes", strconv.FormatInt(p.Nodes, 10))
	}

	if p.MoveTime > 0 {
		parts = append(parts, "movetime", strconv.FormatInt(p.MoveTime.Milliseconds(), 10))
	}

	if p.WhiteTime > 0 {
		parts = append(parts, "wtime", strconv.FormatInt(p.WhiteTime.Milliseconds(), 10))
	}

	if p.BlackTime > 0 {
		parts = append(parts, "btime", strconv.FormatInt(p.BlackTime.Milliseconds(), 10))
	}

	if p.WhiteInc > 0 {
		parts = append(parts, "winc", strconv.FormatInt(p.WhiteInc.Milliseconds(), 10))
	}

	if p.BlackInc > 0 {
		parts = append(parts, "binc", strconv.FormatInt(p.BlackInc.Milliseconds(), 10))
	}

	if p.MovesToGo > 0 {
		parts = append(parts, "movestogo", strconv.Itoa(p.MovesToGo))
	}

	if len(p.SearchMoves) > 0 {
		parts = append(parts, "searchmoves")
		parts = append(parts, p.SearchMoves...)
	}

	return strings.Join(parts, " ")
}

// BuildPositionCommand constructs a "position" command string.
func BuildPositionCommand(fen string, moves []string) string {
	var parts []string
	parts = append(parts, "position")

	if fen == "" || fen == "startpos" {
		parts = append(parts, "startpos")
	} else {
		parts = append(parts, "fen", fen)
	}

	if len(moves) > 0 {
		parts = append(parts, "moves")
		parts = append(parts, moves...)
	}

	return strings.Join(parts, " ")
}

// BuildSetOptionCommand constructs a "setoption" command string.
func BuildSetOptionCommand(name, value string) string {
	if value == "" {
		return "setoption name " + name
	}
	return "setoption name " + name + " value " + value
}
