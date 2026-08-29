package tournament

import (
	"math"
	"testing"
)

func TestZForConfidenceAllBranches(t *testing.T) {
	cases := []struct {
		name       string
		confidence float64
		want       float64
	}{
		{"at 0.99", 0.99, 2.576},
		{"above 0.99", 0.9999, 2.576},
		{"at 0.975", 0.975, 2.241},
		{"at 0.95", 0.95, 1.96},
		{"at 0.90", 0.90, 1.645},
		{"at 0.80", 0.80, 1.282},
		{"below every named threshold", 0.5, 1.96},
		{"at zero", 0, 1.96},
	}
	for _, c := range cases {
		if got := zForConfidence(c.confidence); got != c.want {
			t.Errorf("zForConfidence(%v) = %v, want exactly %v", c.confidence, got, c.want)
		}
	}
}

func TestTrinomialMeanSEDraws(t *testing.T) {
	mean, se := trinomialMeanSE(10, 5, 5)
	if math.Abs(mean-0.625) > 1e-9 {
		t.Errorf("trinomialMeanSE(10,5,5) mean = %v, want 0.625", mean)
	}
	if math.Abs(se-0.09270248108869579) > 1e-9 {
		t.Errorf("trinomialMeanSE(10,5,5) se = %v, want ~0.09270248108869579", se)
	}
}

func TestTrinomialMeanSENoGames(t *testing.T) {
	mean, se := trinomialMeanSE(0, 0, 0)
	if mean != 0.5 {
		t.Errorf("trinomialMeanSE(0,0,0) mean = %v, want exactly 0.5", mean)
	}
	if se != 0 {
		t.Errorf("trinomialMeanSE(0,0,0) se = %v, want exactly 0", se)
	}
	if math.IsNaN(mean) || math.IsNaN(se) {
		t.Errorf("trinomialMeanSE(0,0,0) = (%v, %v), want no NaN", mean, se)
	}
}

func TestTrinomialMeanSESymmetric(t *testing.T) {
	cases := []struct {
		wins, draws, losses int
	}{
		{10, 5, 5},
		{6, 7, 2},
		{3, 0, 17},
	}
	for _, c := range cases {
		mean1, se1 := trinomialMeanSE(c.wins, c.draws, c.losses)
		mean2, se2 := trinomialMeanSE(c.losses, c.draws, c.wins)
		if math.Abs(mean1+mean2-1) > 1e-9 {
			t.Errorf("mean(%d,%d,%d) = %v and mean(%d,%d,%d) = %v, sum = %v, want 1",
				c.wins, c.draws, c.losses, mean1, c.losses, c.draws, c.wins, mean2, mean1+mean2)
		}
		if math.Abs(se1-se2) > 1e-9 {
			t.Errorf("se(%d,%d,%d) = %v and se(%d,%d,%d) = %v, must be identical",
				c.wins, c.draws, c.losses, se1, c.losses, c.draws, c.wins, se2)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 3, "hel"},
		{"hi", 10, "hi"},
		{"", 5, ""},
		{"hello", 5, "hello"},
	}
	for _, c := range cases {
		if got := truncate(c.s, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

func TestIntervalsWiderAtHigherConfidence(t *testing.T) {
	cases := []struct {
		wins, draws, losses int
	}{
		{60, 0, 40},
		{60, 20, 30},
		{100, 0, 100},
	}
	for _, c := range cases {
		lo95, mid95, hi95 := ScoreInterval(c.wins, c.draws, c.losses, 0.95)
		lo99, mid99, hi99 := ScoreInterval(c.wins, c.draws, c.losses, 0.99)
		if mid95 != mid99 {
			t.Errorf("ScoreInterval(%d,%d,%d) mean changed with confidence: %v vs %v",
				c.wins, c.draws, c.losses, mid95, mid99)
		}
		if w99, w95 := hi99-lo99, hi95-lo95; w99 < w95 {
			t.Errorf("ScoreInterval(%d,%d,%d) width at 0.99 = %v < width at 0.95 = %v",
				c.wins, c.draws, c.losses, w99, w95)
		}

		eLo95, eMid95, eHi95 := EloInterval(c.wins, c.draws, c.losses, 0.95)
		eLo99, eMid99, eHi99 := EloInterval(c.wins, c.draws, c.losses, 0.99)
		if eMid95 != eMid99 {
			t.Errorf("EloInterval(%d,%d,%d) mid changed with confidence: %v vs %v",
				c.wins, c.draws, c.losses, eMid95, eMid99)
		}
		if w99, w95 := eHi99-eLo99, eHi95-eLo95; w99 < w95 {
			t.Errorf("EloInterval(%d,%d,%d) width at 0.99 = %v < width at 0.95 = %v",
				c.wins, c.draws, c.losses, w99, w95)
		}
	}
}
