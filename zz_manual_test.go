package main

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// TestRemoveManual: a throwaway mongod on :27099. Removing a manual score deletes it from the
// history and the board; an older agent score, if any, comes back on the board.
func TestRemoveManual(t *testing.T) {
	dbURI = "mongodb://localhost:27099"
	sarpConfig.Team = []teamData{{ID: "tid", Alias: "ann"}}
	initDatabase()
	pc, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if mongoClient.Ping(pc, nil) != nil {
		t.Skip("no throwaway mongod on :27099")
	}
	db := mongoClient.Database(dbName)
	db.Collection("scores").Drop(context.TODO())
	db.Collection("scoreboard").Drop(context.TODO())
	now := time.Now().UTC()
	team := teamData{ID: "tid", Alias: "ann"}
	img := imageData{Name: "img1"}
	v := func(n int) vulnWrapper { return vulnWrapper{VulnsScored: n, VulnsTotal: 100} }
	db.Collection("scores").InsertOne(context.TODO(), scoreEntry{Time: now.Add(-time.Hour), Team: team, Image: img, Vulns: v(40)})
	db.Collection("scores").InsertOne(context.TODO(), bson.M{"team": bson.M{"id": "tid"}, "image": bson.M{"name": "img1"}, "time": now, "score": 100, "manual": true, "vulns": bson.M{"vulnsscored": 100, "vulnstotal": 100}})
	db.Collection("scoreboard").InsertOne(context.TODO(), scoreEntry{Time: now, Team: team, Image: img, Vulns: v(100)})

	if !manualImages("ann")["img1"] {
		t.Fatal("manual score not detected")
	}
	n, err := removeManual(team, "img1")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	top, _ := getTop()
	if len(top) != 1 || top[0].Vulns.VulnsScored != 40 {
		t.Fatalf("board should fall back to the agent's 40: %+v", top)
	}
	if manualImages("ann")["img1"] {
		t.Fatal("still manual")
	}
	if n, _ := removeManual(team, "img1"); n != 0 {
		t.Fatal("second removal should find nothing")
	}
	// no history left: the board entry goes away
	db.Collection("scores").DeleteMany(context.TODO(), bson.D{})
	db.Collection("scores").InsertOne(context.TODO(), bson.M{"team": bson.M{"id": "tid"}, "image": bson.M{"name": "img1"}, "time": now, "manual": true})
	removeManual(team, "img1")
	if top, _ := getTop(); len(top) != 0 {
		t.Fatalf("board should be empty: %+v", top)
	}
}
