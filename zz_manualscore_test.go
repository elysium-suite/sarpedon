package main

import (
	"html/template"
	"testing"
)

func TestSetScoreQuery(t *testing.T) {
	q, bad := setScoreQuery("c1", " team-7 ", "2", "40", "")
	if bad != "" || q.Get("team") != "team-7" || q.Get("level") != "2" || q.Get("scored") != "40" || q.Get("total") != "100" {
		t.Fatalf("good form: %v %q", q, bad)
	}
	for name, a := range map[string][5]string{
		"no comp":    {"", "t", "1", "5", "100"},
		"no team":    {"c", " ", "1", "5", "100"},
		"bad level":  {"c", "t", "x", "5", "100"},
		"neg score":  {"c", "t", "1", "-1", "100"},
		"over total": {"c", "t", "1", "101", "100"},
		"zero total": {"c", "t", "1", "0", "0"},
	} {
		if _, bad := setScoreQuery(a[0], a[1], a[2], a[3], a[4]); bad == "" {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestManageScoreTemplateRenders(t *testing.T) {
	tpl := template.Must(template.New("").Funcs(template.FuncMap{"increment": func(n int) int { return n + 1 }, "local": localTime}).ParseGlob("templates/*"))
	if tpl.Lookup("manage_score.html") == nil {
		t.Fatal("manage_score.html missing")
	}
}
