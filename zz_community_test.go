package main

import (
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchCommunityAndDuration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/community.json" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"generated":"x","images":[{"image":"Ubuntu 22","key":"ubuntu22","entries":[{"user":"a","team":null,"submissions":2,"rank":1,"best":{"score":72,"max_score":100,"elapsed_seconds":4364,"verdict":"ok","reasons":[],"thumb":"/community/img/abc"},"latest":{"score":72,"verdict":"ok"}}]}]}`))
	}))
	defer srv.Close()
	old := sarpConfig.CommunityUrl
	sarpConfig.CommunityUrl = srv.URL + "/"
	defer func() { sarpConfig.CommunityUrl = old }()
	f, err := fetchCommunity()
	if err != nil || len(f.Images) != 1 {
		t.Fatalf("feed: %v %+v", err, f)
	}
	e := f.Images[0].Entries[0]
	if e.Best.Score != 72 || e.Team != nil || *e.Rank != 1 {
		t.Fatalf("entry %+v", e)
	}
	if d := communityDuration(e.Best.ElapsedSeconds); d != "1:12:44" {
		t.Fatalf("duration %q", d)
	}
	if communityDuration(nil) != "" || communityInt(nil) != "" {
		t.Fatal("nil handling")
	}
}

func TestFetchCommunityUnreachable(t *testing.T) {
	old := sarpConfig.CommunityUrl
	sarpConfig.CommunityUrl = "http://127.0.0.1:1"
	defer func() { sarpConfig.CommunityUrl = old }()
	if _, err := fetchCommunity(); err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("want unreachable, got %v", err)
	}
}

func TestCommunityHashRe(t *testing.T) {
	if communityHashRe.MatchString("../etc/passwd") || !communityHashRe.MatchString(strings.Repeat("a", 64)) {
		t.Fatal("hash regex")
	}
}

func TestCommunityTemplateRenders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images":[{"image":"Ubuntu 22","key":"u","entries":[{"user":"alice","team":"Turtles","submissions":1,"rank":1,"best":{"score":72,"max_score":100,"found_vulns":14,"total_vulns":20,"elapsed_seconds":4364,"verdict":"suspicious","reasons":["too fast"],"thumb":"/community/img/abc"},"latest":{"score":72,"verdict":"ok"}}]}]}`))
	}))
	defer srv.Close()
	old := sarpConfig.CommunityUrl
	sarpConfig.CommunityUrl = srv.URL
	defer func() { sarpConfig.CommunityUrl = old }()
	r := gin.New()
	r.SetFuncMap(template.FuncMap{"increment": func(n int) int { return n + 1 }, "local": localTime})
	r.LoadHTMLGlob("templates/*")
	initCookies(r)
	r.GET("/community", viewCommunity)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/community", nil))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "Ubuntu 22") || !strings.Contains(body, "1:12:44") || !strings.Contains(body, "14 / 20") || !strings.Contains(body, "/community/img/abc") {
		t.Fatalf("code %d body %s", w.Code, body)
	}
}
