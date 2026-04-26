package tournament

import "testing"

func TestEstimateUsageDefaultsOneThreadNoHash(t *testing.T) {
	pairings := []Pairing{{
		White: EngineSpec{Name: "A"},
		Black: EngineSpec{Name: "B"},
	}}
	u := EstimateUsage(pairings, 1)
	if u.Threads != 2 {
		t.Errorf("Threads = %d, want 2 (1+1 default)", u.Threads)
	}
	if u.HashMB != 0 {
		t.Errorf("HashMB = %d, want 0 (no Hash option)", u.HashMB)
	}
}

func TestEstimateUsageReadsThreadsAndHash(t *testing.T) {
	pairings := []Pairing{{
		White: EngineSpec{Name: "A", Options: map[string]string{"Threads": "4", "Hash": "256"}},
		Black: EngineSpec{Name: "B", Options: map[string]string{"Threads": "2", "Hash": "128"}},
	}}
	u := EstimateUsage(pairings, 1)
	if u.Threads != 6 {
		t.Errorf("Threads = %d, want 6", u.Threads)
	}
	if u.HashMB != 384 {
		t.Errorf("HashMB = %d, want 384", u.HashMB)
	}
}

func TestEstimateUsageOptionNameCaseInsensitive(t *testing.T) {
	pairings := []Pairing{{
		White: EngineSpec{Name: "A", Options: map[string]string{"threads": "3", "hash": "64"}},
		Black: EngineSpec{Name: "B", Options: map[string]string{"THREADS": "5"}},
	}}
	u := EstimateUsage(pairings, 1)
	if u.Threads != 8 {
		t.Errorf("Threads = %d, want 8", u.Threads)
	}
	if u.HashMB != 64 {
		t.Errorf("HashMB = %d, want 64", u.HashMB)
	}
}

func TestEstimateUsagePicksTopConcurrentPairings(t *testing.T) {
	heavy := EngineSpec{Name: "Heavy", Options: map[string]string{"Threads": "8", "Hash": "1024"}}
	light := EngineSpec{Name: "Light", Options: map[string]string{"Threads": "1", "Hash": "16"}}
	pairings := []Pairing{
		{White: light, Black: light},
		{White: heavy, Black: heavy},
		{White: heavy, Black: light},
		{White: light, Black: heavy},
	}
	// Concurrency 2: worst-case is the two heaviest pairings.
	// heavy/heavy = 16 threads, 2048 MB
	// heavy/light = 9 threads, 1040 MB (or light/heavy, same cost)
	u := EstimateUsage(pairings, 2)
	wantThreads := 16 + 9
	wantHash := 2048 + 1040
	if u.Threads != wantThreads {
		t.Errorf("Threads = %d, want %d", u.Threads, wantThreads)
	}
	if u.HashMB != wantHash {
		t.Errorf("HashMB = %d, want %d", u.HashMB, wantHash)
	}
}

func TestEstimateUsageConcurrencyExceedsPairingsClamps(t *testing.T) {
	pairings := []Pairing{{
		White: EngineSpec{Name: "A", Options: map[string]string{"Threads": "2"}},
		Black: EngineSpec{Name: "B", Options: map[string]string{"Threads": "2"}},
	}}
	u := EstimateUsage(pairings, 8)
	if u.Threads != 4 {
		t.Errorf("Threads = %d, want 4 (only one pairing exists)", u.Threads)
	}
}

func TestEstimateUsageInvalidValuesUseDefaults(t *testing.T) {
	pairings := []Pairing{{
		White: EngineSpec{Name: "A", Options: map[string]string{"Threads": "abc", "Hash": "-1"}},
		Black: EngineSpec{Name: "B", Options: map[string]string{"Threads": "0"}},
	}}
	u := EstimateUsage(pairings, 1)
	if u.Threads != 2 {
		t.Errorf("Threads = %d, want 2 (both fall back to 1)", u.Threads)
	}
	if u.HashMB != 0 {
		t.Errorf("HashMB = %d, want 0", u.HashMB)
	}
}

func TestEstimateUsageEmpty(t *testing.T) {
	u := EstimateUsage(nil, 4)
	if u.Threads != 0 || u.HashMB != 0 {
		t.Errorf("EstimateUsage(nil) = %+v, want zero", u)
	}
}
