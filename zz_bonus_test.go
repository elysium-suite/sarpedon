package main

import (
	"context"
	"testing"
	"time"
)

// TestSprintBonus drives practiceScores() against a throwaway mongod on :27099 and checks the
// Weekly Sprint "pre-release x2" rule for all four cases. Start mongod on 27099 first.
func TestSprintBonus(t *testing.T) {
	dbURI = "mongodb://localhost:27099"
	R := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC) // release time (in the past -> released)

	sarpConfig.Image = []imageData{
		{Name: "rel-win", Practice: true},
		{Name: "unrel-lin", Practice: true},
		{Name: "cisco1", Practice: true},
	}
	sarpConfig.Release = []releaseData{{Comp: "comp-rel", Time: R}}

	practiceCache = []practiceComp{
		{ID: "comp-rel", Kind: "windows", Practice: true, Levels: []practiceLevel{{Level: 1, Image: "rel-win", Threshold: 100}}},
		{ID: "comp-unrel", Kind: "linux", Practice: true, Levels: []practiceLevel{{Level: 1, Image: "unrel-lin", Threshold: 100}}},
		{ID: "comp-cisco", Kind: "cisco", Practice: true, Levels: []practiceLevel{{Level: 1, Image: "cisco1", Threshold: 100}}},
	}
	practiceAt = time.Now() // keep fetchPractices on the cache (no levelsvc)

	initDatabase()
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := mongoClient.Ping(pingCtx, nil); err != nil {
		t.Skip("no throwaway mongod on :27099 (start: mongod --port 27099 --dbpath <tmp>)")
	}
	db := mongoClient.Database(dbName)
	db.Collection("scores").Drop(context.TODO())
	db.Collection("scoreboard").Drop(context.TODO())

	mk := func(img, alias string, scored int, ts time.Time) interface{} {
		return scoreEntry{Time: ts, Team: teamData{ID: alias, Alias: alias},
			Image: imageData{Name: img}, Vulns: vulnWrapper{VulnsScored: scored, VulnsTotal: 10}}
	}
	// full report history (scores)
	hist := []interface{}{
		mk("rel-win", "alpha", 3, R.Add(-2*time.Hour)), // pre-release: reaches 6
		mk("rel-win", "alpha", 6, R.Add(-1*time.Hour)),
		mk("rel-win", "alpha", 7, R.Add(1*time.Hour)), // post-release: reaches 9
		mk("rel-win", "alpha", 9, R.Add(2*time.Hour)),
		mk("rel-win", "bravo", 10, R.Add(-3*time.Hour)), // finished 100% before release
		mk("rel-win", "bravo", 10, R.Add(1*time.Hour)),  // VM left running: re-reports after release
		mk("rel-win", "bravo", 10, R.Add(2*time.Hour)),
		mk("unrel-lin", "charlie", 8, R.Add(1*time.Hour)),
		mk("cisco1", "delta", 8, R.Add(1*time.Hour)),
	}
	if _, err := db.Collection("scores").InsertMany(context.TODO(), hist); err != nil {
		t.Fatal(err)
	}
	// latest per (image, team) — what getTop()/the board reads
	latest := []interface{}{
		mk("rel-win", "alpha", 9, R.Add(2*time.Hour)),
		mk("rel-win", "bravo", 10, R.Add(2*time.Hour)),
		mk("unrel-lin", "charlie", 8, R.Add(1*time.Hour)),
		mk("cisco1", "delta", 8, R.Add(1*time.Hour)),
	}
	if _, err := db.Collection("scoreboard").InsertMany(context.TODO(), latest); err != nil {
		t.Fatal(err)
	}

	sc, err := practiceScores()
	if err != nil {
		t.Fatal(err)
	}
	check := func(label, img, alias string, wantPct int) {
		e, ok := sc[img][alias]
		if !ok {
			t.Fatalf("%s: %s/%s missing from board", label, img, alias)
		}
		got := pctOf(e)
		if got != wantPct {
			t.Errorf("%s: %s/%s got %d%%, want %d%%", label, img, alias, got, wantPct)
		} else {
			t.Logf("%s: %s/%s = %d%% OK", label, img, alias, got)
		}
	}
	check("case1 released pre6/post9", "rel-win", "alpha", 150)
	check("case4 100%% pre, reported after", "rel-win", "bravo", 200)
	check("case2 unreleased win/lin doubled", "unrel-lin", "charlie", 160)
	check("case3 cisco untouched", "cisco1", "delta", 80)
}
