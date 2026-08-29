package uci

import (
	"strings"
	"testing"
	"time"
)

func TestRegisterEngine(t *testing.T) {
	m := NewEngineManager()

	if err := m.RegisterEngine("e1", "/opt/sf1"); err != nil {
		t.Fatalf("RegisterEngine(e1) unexpected error: %v", err)
	}

	err := m.RegisterEngine("e1", "/opt/sf2")
	if err == nil {
		t.Fatal("RegisterEngine(e1) duplicate: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("duplicate register error = %q, want to contain %q", err.Error(), "already registered")
	}

	eng, err := m.GetEngine("e1")
	if err != nil {
		t.Fatalf("GetEngine(e1) error: %v", err)
	}
	if eng.BinaryPath != "/opt/sf1" {
		t.Errorf("BinaryPath = %q, want %q (first registration must be preserved)", eng.BinaryPath, "/opt/sf1")
	}
}

func TestUnregisterEngine(t *testing.T) {
	m := NewEngineManager()

	if err := m.UnregisterEngine("missing"); err == nil {
		t.Error("UnregisterEngine(missing): expected error, got nil")
	}

	if err := m.RegisterEngine("e1", "/opt/sf1"); err != nil {
		t.Fatalf("RegisterEngine(e1) error: %v", err)
	}

	if err := m.UnregisterEngine("e1"); err != nil {
		t.Fatalf("UnregisterEngine(e1) on never-started engine: unexpected error: %v", err)
	}
	if _, err := m.GetEngine("e1"); err == nil {
		t.Error("GetEngine(e1) after unregister: expected error, got nil")
	}
}

func TestGetEngineUnknownID(t *testing.T) {
	m := NewEngineManager()
	eng, err := m.GetEngine("nope")
	if err == nil {
		t.Error("GetEngine(unknown): expected error, got nil")
	}
	if eng != nil {
		t.Errorf("GetEngine(unknown) = %v, want nil", eng)
	}
}

func TestListEngines(t *testing.T) {
	m := NewEngineManager()

	got := m.ListEngines()
	if len(got) != 0 {
		t.Fatalf("ListEngines() empty manager = %v, want length 0", got)
	}

	if err := m.RegisterEngine("e1", "/opt/e1"); err != nil {
		t.Fatalf("RegisterEngine(e1) error: %v", err)
	}
	if err := m.RegisterEngine("e2", "/opt/e2"); err != nil {
		t.Fatalf("RegisterEngine(e2) error: %v", err)
	}

	got = m.ListEngines()
	if len(got) != 2 {
		t.Fatalf("ListEngines() after 2 registers = %v, want length 2", got)
	}

	byID := make(map[string]EngineInfo, len(got))
	for _, info := range got {
		if _, dup := byID[info.ID]; dup {
			t.Errorf("ListEngines() returned duplicate ID %q", info.ID)
		}
		byID[info.ID] = info
	}

	for _, id := range []string{"e1", "e2"} {
		info, ok := byID[id]
		if !ok {
			t.Errorf("ListEngines() missing engine %q", id)
			continue
		}
		if info.BinaryPath != "/opt/"+id {
			t.Errorf("engine %s BinaryPath = %q, want %q", id, info.BinaryPath, "/opt/"+id)
		}
		if info.State != "none" {
			t.Errorf("engine %s State = %q, want %q", id, info.State, "none")
		}
	}
}

func TestSetThrottleRate(t *testing.T) {
	m := NewEngineManager()
	if m.throttleInterval != 50*time.Millisecond {
		t.Fatalf("fresh manager throttleInterval = %v, want 50ms", m.throttleInterval)
	}

	tests := []struct {
		name string
		hz   int
		want time.Duration
	}{
		{"zero uses default", 0, 50 * time.Millisecond},
		{"negative uses default", -5, 50 * time.Millisecond},
		{"hundred hertz", 100, 10 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.SetThrottleRate(tt.hz)
			if m.throttleInterval != tt.want {
				t.Errorf("SetThrottleRate(%d) throttleInterval = %v, want %v", tt.hz, m.throttleInterval, tt.want)
			}
		})
	}
}

func recordingManager() (*EngineManager, *[]AnalysisInfo) {
	m := NewEngineManager()
	got := make([]AnalysisInfo, 0, 4)
	m.SetAnalysisCallback(func(info AnalysisInfo) {
		got = append(got, info)
	})
	return m, &got
}

func TestEmitThrottledWindowElapsed(t *testing.T) {
	m, got := recordingManager()
	info := AnalysisInfo{EngineID: "e1", MultiPV: 1, PV: []string{"e2e4"}}

	m.emitThrottled(info)
	if len(*got) != 1 {
		t.Fatalf("after first emit, calls = %d, want 1", len(*got))
	}

	key := "e1:1"
	m.lastEmit[key] = time.Now().Add(-time.Hour)

	m.emitThrottled(info)
	if len(*got) != 2 {
		t.Errorf("after backdated window elapsed, calls = %d, want 2", len(*got))
	}
}

func TestEmitThrottledFirstCallEmits(t *testing.T) {
	m, got := recordingManager()
	m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 1, PV: []string{"e2e4"}})
	if len(*got) != 1 {
		t.Errorf("calls = %d, want 1", len(*got))
	}
}

func TestEmitThrottledWithinInterval(t *testing.T) {
	m, got := recordingManager()
	info := AnalysisInfo{EngineID: "e1", MultiPV: 1, PV: []string{"e2e4"}}
	m.emitThrottled(info)
	m.emitThrottled(info)
	if len(*got) != 1 {
		t.Errorf("calls = %d, want 1 (second call within interval is throttled)", len(*got))
	}
}

func TestEmitThrottledDeepDepthBypass(t *testing.T) {
	m, got := recordingManager()
	first := AnalysisInfo{EngineID: "e1", MultiPV: 1, Depth: 5, PV: []string{"e2e4"}}
	m.emitThrottled(first)
	m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 1, Depth: 20, PV: []string{"e2e4"}})
	if len(*got) != 2 {
		t.Errorf("calls = %d, want 2 (depth >= 20 must bypass throttle)", len(*got))
	}
}

func TestEmitThrottledEmptyPVBypass(t *testing.T) {
	m, got := recordingManager()
	first := AnalysisInfo{EngineID: "e1", MultiPV: 1, Depth: 5, PV: []string{"e2e4"}}
	m.emitThrottled(first)
	m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 1, Depth: 5})
	if len(*got) != 2 {
		t.Errorf("calls = %d, want 2 (empty PV must bypass throttle)", len(*got))
	}
}

func TestEmitThrottledIndependentKeys(t *testing.T) {
	t.Run("different MultiPV", func(t *testing.T) {
		m, got := recordingManager()
		m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 1, PV: []string{"e2e4"}})
		m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 2, PV: []string{"d2d4"}})
		if len(*got) != 2 {
			t.Errorf("calls = %d, want 2 (per-MultiPV keys throttle independently)", len(*got))
		}
	})

	t.Run("different EngineID", func(t *testing.T) {
		m, got := recordingManager()
		m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 1, PV: []string{"e2e4"}})
		m.emitThrottled(AnalysisInfo{EngineID: "e2", MultiPV: 1, PV: []string{"d2d4"}})
		if len(*got) != 2 {
			t.Errorf("calls = %d, want 2 (per-EngineID keys throttle independently)", len(*got))
		}
	})
}

func TestEmitThrottledNoCallback(t *testing.T) {
	m := NewEngineManager()
	m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 1, PV: []string{"e2e4"}})
	m.emitThrottled(AnalysisInfo{EngineID: "e1", MultiPV: 1, PV: []string{"e2e4"}})
}
