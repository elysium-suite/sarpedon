package main

import "testing"

func TestCellFracKeepsPreReleaseBonus(t *testing.T) {
	for _, c := range []struct {
		pct, thr int
		want     float64
	}{{200, 100, 2}, {150, 100, 1.5}, {100, 100, 1}, {75, 100, 0.75}, {95, 90, 1}, {300, 100, 2}} {
		e := scoreEntry{}
		e.Vulns.VulnsTotal, e.Vulns.VulnsScored = 100, c.pct
		if got := cellFrac(cellFor(e, true, c.thr)); got != c.want {
			t.Errorf("pct %d thr %d: got %v want %v", c.pct, c.thr, got, c.want)
		}
	}
}
