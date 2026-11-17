package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// monitor_ip.go reconstructs the admin Monitor per-IP drill-down: /monitor/ip/:ip
// shows one source IP's unregistered attempts, its /update score reports, and an
// activity timeline (48 equal buckets colored by event count).

type ipBucket struct {
	Tip   string
	Color string
}

func getUnregisteredByIP(ip string) ([]unregEntry, error) {
	initDatabase()
	entries := []unregEntry{}
	coll := mongoClient.Database(dbName).Collection("unregistered")
	fo := options.Find().SetSort(bson.D{{"lastseen", -1}})
	cur, err := coll.Find(context.TODO(), bson.D{{"sourceip", ip}}, fo)
	if err != nil {
		return entries, err
	}
	if err := cur.All(context.TODO(), &entries); err != nil {
		return entries, err
	}
	loc, _ := time.LoadLocation(sarpConfig.Timezone)
	for i := range entries {
		entries[i].FirstSeen = entries[i].FirstSeen.In(loc)
		entries[i].LastSeen = entries[i].LastSeen.In(loc)
	}
	return entries, nil
}

func getScoresByIP(ip string, limit int64) ([]scoreEntry, error) {
	initDatabase()
	entries := []scoreEntry{}
	coll := mongoClient.Database(dbName).Collection("scores")
	fo := options.Find().SetSort(bson.D{{"time", -1}}).SetLimit(limit)
	cur, err := coll.Find(context.TODO(), bson.D{{"sourceip", ip}}, fo)
	if err != nil {
		return entries, err
	}
	if err := cur.All(context.TODO(), &entries); err != nil {
		return entries, err
	}
	loc, _ := time.LoadLocation(sarpConfig.Timezone)
	for i := range entries {
		entries[i].Time = entries[i].Time.In(loc)
	}
	return entries, nil
}

func buildTimeline(data gin.H, unreg []unregEntry, updates []scoreEntry) {
	var times []time.Time
	for _, u := range unreg {
		times = append(times, u.FirstSeen, u.LastSeen)
	}
	for _, s := range updates {
		times = append(times, s.Time)
	}
	if len(times) == 0 {
		return
	}
	start, end := times[0], times[0]
	for _, t := range times {
		if t.Before(start) {
			start = t
		}
		if t.After(end) {
			end = t
		}
	}
	if !end.After(start) {
		end = start.Add(time.Minute)
	}
	const N = 48
	span := end.Sub(start)
	bucketDur := span / time.Duration(N)
	if bucketDur <= 0 {
		bucketDur = time.Minute
	}
	idx := func(t time.Time) int {
		i := int(t.Sub(start) / bucketDur)
		if i < 0 {
			i = 0
		}
		if i >= N {
			i = N - 1
		}
		return i
	}
	counts := make([]float64, N)
	totalEvents := 0.0
	for _, s := range updates {
		counts[idx(s.Time)]++
		totalEvents++
	}
	var online time.Duration
	for _, u := range unreg {
		a, b := idx(u.FirstSeen), idx(u.LastSeen)
		if b < a {
			a, b = b, a
		}
		per := float64(u.Count) / float64(b-a+1)
		for i := a; i <= b; i++ {
			counts[i] += per
		}
		totalEvents += float64(u.Count)
		if d := u.LastSeen.Sub(u.FirstSeen); d > 0 {
			online += d
		}
	}
	max := 0.0
	for _, c := range counts {
		if c > max {
			max = c
		}
	}
	colors := []string{"#20242a", "#243b57", "#2f5e97", "#3f86d4", "#5aa6ff"}
	loc, _ := time.LoadLocation(sarpConfig.Timezone)
	buckets := make([]ipBucket, N)
	for i := 0; i < N; i++ {
		lvl := 0
		if max > 0 && counts[i] > 0 {
			lvl = int(counts[i]/max*4.0 + 0.9999)
			if lvl < 1 {
				lvl = 1
			}
			if lvl > 4 {
				lvl = 4
			}
		}
		bStart := start.Add(time.Duration(i) * bucketDur).In(loc)
		buckets[i] = ipBucket{
			Tip:   fmt.Sprintf("%s  -  %.0f event(s)", bStart.Format("01-02 15:04 MST"), counts[i]),
			Color: colors[lvl],
		}
	}
	data["hasTimeline"] = true
	data["buckets"] = buckets
	data["axisStart"] = start.In(loc).Format("2006-01-02 15:04 MST")
	data["axisEnd"] = end.In(loc).Format("2006-01-02 15:04 MST")
	data["bucketDur"] = bucketDur.Round(time.Second).String()
	data["totalEvents"] = int(totalEvents + 0.5)
	data["onlineTotal"] = online.Round(time.Minute).String()
}

func viewMonitorIP(c *gin.Context) {
	ip := c.Param("ip")
	unreg, err := getUnregisteredByIP(ip)
	if err != nil {
		unreg = []unregEntry{}
		fmt.Println("monitor/ip unregistered:", err)
	}
	updates, err := getScoresByIP(ip, 1000)
	if err != nil {
		updates = []scoreEntry{}
		fmt.Println("monitor/ip scores:", err)
	}
	teams := map[string]bool{}
	images := map[string]bool{}
	for _, s := range updates {
		teams[s.Team.ID] = true
		images[s.Image.Name] = true
	}
	data := gin.H{
		"ip": ip, "unregistered": unreg, "updates": updates,
		"updateCount": len(updates), "teamCount": len(teams), "imageCount": len(images),
		"hasTimeline": false,
	}
	buildTimeline(data, unreg, updates)
	c.HTML(http.StatusOK, "monitor_ip.html", pageData(c, "Source IP", data))
}
