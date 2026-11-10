package main

import (
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// manualscore.go: the admin page to set or overwrite a team's score for a level (projman #339),
// for students whose VM cannot run the scoring agent. Behind authRequired like the rest of
// /manage; it calls levelsvc POST /admin/setscore with the levelsvc admin token from
// sarpedon.conf, so the admin never handles that token.

type scoreComp struct {
	ID     string
	Name   string
	Levels []int
	Info   []scoreLevel
}

// scoreLevel is one level with its image, and (once a team is chosen) that team's current score.
type scoreLevel struct {
	Level   int
	Name    string
	Image   string
	Has     bool
	Scored  int
	Total   int
	Percent int
}

// scoreComps lists the competitions with their level numbers 1..N (levelsvc /admin/competitions).
func scoreComps() ([]scoreComp, error) {
	var res struct {
		Competitions []map[string]interface{} `json:"competitions"`
	}
	if err := lvGetJSON("/admin/competitions", &res); err != nil {
		return nil, err
	}
	out := []scoreComp{}
	for _, m := range res.Competitions {
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		n, _ := m["levels"].(float64)
		sc := scoreComp{ID: id, Name: name}
		for i := 1; i <= int(n); i++ {
			sc.Levels = append(sc.Levels, i)
			sc.Info = append(sc.Info, scoreLevel{Level: i, Name: "Level " + strconv.Itoa(i)})
		}
		if infos, ok := m["levelInfo"].([]interface{}); ok {
			for _, x := range infos {
				li, _ := x.(map[string]interface{})
				lv, _ := li["level"].(float64)
				if i := int(lv) - 1; i >= 0 && i < len(sc.Info) {
					if nm, _ := li["name"].(string); nm != "" {
						sc.Info[i].Name = nm
					}
					sc.Info[i].Image, _ = li["image"].(string)
				}
			}
		}
		if id != "" {
			out = append(out, sc)
		}
	}
	return out, nil
}

// scoreTeams: the teams sarpedon.conf knows, as id and alias, for the form's suggestions.
func scoreTeams() []string {
	out := []string{}
	for _, t := range sarpConfig.Team {
		out = append(out, t.ID)
		if t.Alias != "" && !strings.EqualFold(t.Alias, t.ID) {
			out = append(out, t.Alias)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

// withCurrentScores fills in the team's present score on each level (what the scoreboard holds
// now, from the agent or a manual entry), so the admin sees what they are replacing.
func withCurrentScores(info []scoreLevel, team string) []scoreLevel {
	out := append([]scoreLevel(nil), info...)
	team = strings.TrimSpace(team)
	if team == "" {
		return out
	}
	top, err := getTop()
	if err != nil {
		return out
	}
	for _, e := range top {
		if !strings.EqualFold(e.Team.Alias, team) && !strings.EqualFold(e.Team.ID, team) {
			continue
		}
		for i := range out {
			if out[i].Image != "" && out[i].Image == e.Image.Name {
				out[i].Has, out[i].Scored, out[i].Total, out[i].Percent = true, e.Vulns.VulnsScored, e.Vulns.VulnsTotal, pctOf(e)
			}
		}
	}
	return out
}

func viewSetScore(c *gin.Context) {
	data := gin.H{"msg": c.Query("msg"), "isErr": strings.HasPrefix(c.Query("msg"), "error"), "teams": scoreTeams()}
	if sarpConfig.LevelsvcToken == "" {
		data["err"] = "levelsvcToken is not set in sarpedon.conf — add it (the levelsvc admin token)."
		c.HTML(http.StatusOK, "manage_score.html", pageData(c, "Set score", data))
		return
	}
	comps, err := scoreComps()
	if err != nil {
		data["err"] = err.Error()
	}
	data["comps"] = comps
	data["comp"] = c.Query("comp")
	data["team"] = c.Query("team")
	for _, sc := range comps {
		if sc.ID == c.Query("comp") {
			data["levels"] = sc.Levels
			data["info"] = withCurrentScores(sc.Info, c.Query("team"))
		}
	}
	c.HTML(http.StatusOK, "manage_score.html", pageData(c, "Set score", data))
}

// setScoreQuery checks the form and builds levelsvc's query, or says what is wrong.
func setScoreQuery(comp, team, level, scored, total string) (url.Values, string) {
	team = strings.TrimSpace(team)
	if comp == "" || team == "" {
		return nil, "Choose a competition and enter a team."
	}
	lvl, err := strconv.Atoi(strings.TrimSpace(level))
	if err != nil || lvl < 1 {
		return nil, "Choose a level."
	}
	s, err := strconv.Atoi(strings.TrimSpace(scored))
	if err != nil || s < 0 {
		return nil, "Enter the points scored (0 or more)."
	}
	t := 100
	if strings.TrimSpace(total) != "" {
		if t, err = strconv.Atoi(strings.TrimSpace(total)); err != nil || t < 1 {
			return nil, "Total points must be a number of 1 or more."
		}
	}
	if s > t {
		return nil, "Points scored cannot be more than the total."
	}
	return url.Values{"comp": {comp}, "team": {team}, "level": {strconv.Itoa(lvl)}, "scored": {strconv.Itoa(s)}, "total": {strconv.Itoa(t)}}, ""
}

func manageSetScore(c *gin.Context) {
	comp := c.PostForm("comp")
	back := func(m string) {
		c.Redirect(http.StatusSeeOther, "/manage/score?comp="+url.QueryEscape(comp)+"&team="+url.QueryEscape(c.PostForm("team"))+"&msg="+url.QueryEscape(m))
	}
	q, bad := setScoreQuery(comp, c.PostForm("team"), c.PostForm("level"), c.PostForm("scored"), c.PostForm("total"))
	if bad != "" {
		back("error: " + bad)
		return
	}
	resp, err := lvDo("POST", "/admin/setscore?"+q.Encode(), nil, "")
	if err != nil {
		back("error: " + err.Error())
		return
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		back("error: " + strings.TrimSpace(string(b)))
		return
	}
	back("Saved: " + q.Get("team") + " now has " + q.Get("scored") + "/" + q.Get("total") + " on level " + q.Get("level") + " of " + comp + ".")
}
