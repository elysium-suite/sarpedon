package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// aiflags.go shows the PT agents' AI-use reports for a competition (levelsvc GET /admin/aiflags).
// Agents report silently while a level runs in a competition with the AI check on; each row is
// one (signal, AI service, evidence) with when it was first/last seen and how often.

type aiFlagEvent struct {
	Source   string `json:"source"`
	Service  string `json:"service"`
	Evidence string `json:"evidence"`
	First    int64  `json:"first"`
	Last     int64  `json:"last"`
	Count    int    `json:"count"`
	Level    int    `json:"level"`
	Full     string `json:"full"` // full captured text (clipboard)
}

func (e aiFlagEvent) FirstAt() time.Time { return time.Unix(e.First, 0) }
func (e aiFlagEvent) LastAt() time.Time  { return time.Unix(e.Last, 0) }

func (e aiFlagEvent) SourceLabel() string {
	switch e.Source {
	case "title":
		return "Window title"
	case "history":
		return "Browser history"
	case "dns":
		return "DNS lookup"
	case "clipboard":
		return "Clipboard copy"
	case "process":
		return "AI app / CLI"
	}
	return e.Source
}

// Seen describes the count in the signal's own terms (window titles are checked every 5 s).
func (e aiFlagEvent) Seen() string {
	plural := func(n int, one string) string {
		if n == 1 {
			return "1 " + one
		}
		return fmt.Sprintf("%d %ss", n, one)
	}
	switch e.Source {
	case "title":
		return fmt.Sprintf("%s (~%s on screen)", plural(e.Count, "check"), (time.Duration(e.Count) * 5 * time.Second).String())
	case "history":
		return plural(e.Count, "visit")
	case "dns":
		return plural(e.Count, "lookup")
	case "clipboard":
		return plural(e.Count, "copy")
	case "process":
		return "running"
	}
	return fmt.Sprint(e.Count)
}

type aiFlagTeam struct {
	Team     string            `json:"team"`
	LastSeen int64             `json:"lastSeen"`
	Since    int64             `json:"since"`
	Version  string            `json:"version"`
	Platform string            `json:"platform"`
	Checks   map[string]string `json:"checks"`
	IP       string            `json:"ip"`
	Events   []aiFlagEvent     `json:"events"`
}

func (t aiFlagTeam) LastSeenAt() time.Time { return time.Unix(t.LastSeen, 0) }

// CheckList is what the agent could check on that computer, e.g. "history: Chrome, Firefox".
func (t aiFlagTeam) CheckList() string {
	names := map[string]string{"titles": "window titles", "history": "browser history", "dns": "DNS", "clipboard": "clipboard", "processes": "AI apps"}
	keys := []string{}
	for k := range t.Checks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		label := names[k]
		if label == "" {
			label = k
		}
		parts = append(parts, label+": "+t.Checks[k])
	}
	return strings.Join(parts, " · ")
}

func viewAIFlags(c *gin.Context) {
	comp := c.Query("comp")
	if comp == "" {
		c.Redirect(http.StatusSeeOther, "/manage")
		return
	}
	data := gin.H{"comp": comp, "msg": c.Query("msg")}
	var res struct {
		Name    string       `json:"name"`
		AICheck bool         `json:"aiCheck"`
		Teams   []aiFlagTeam `json:"teams"`
	}
	if err := lvGetJSON("/admin/aiflags?comp="+url.QueryEscape(comp), &res); err != nil {
		data["err"] = err.Error()
	} else {
		flagged, clean := []aiFlagTeam{}, []aiFlagTeam{}
		for _, t := range res.Teams {
			if len(t.Events) > 0 {
				flagged = append(flagged, t)
			} else {
				clean = append(clean, t)
			}
		}
		data["name"], data["aiCheck"], data["flagged"], data["clean"] = res.Name, res.AICheck, flagged, clean
	}
	c.HTML(http.StatusOK, "manage_ai.html", pageData(c, "AI-use flags", data))
}

func manageAIClear(c *gin.Context) {
	comp, team := c.PostForm("comp"), c.PostForm("team")
	back := func(m string) {
		c.Redirect(http.StatusSeeOther, "/manage/ai?comp="+url.QueryEscape(comp)+"&msg="+url.QueryEscape(m))
	}
	resp, err := lvDo("POST", "/admin/aiflags?comp="+url.QueryEscape(comp)+"&team="+url.QueryEscape(team)+"&action=clear", nil, "")
	if err != nil {
		back("could not clear: " + err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		back("could not clear: " + strings.TrimSpace(string(b)))
		return
	}
	back("cleared " + team)
}
