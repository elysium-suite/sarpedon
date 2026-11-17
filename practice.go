package main

// Practice levels: the PT practice competitions run by levelsvc. Each level is a
// sarpedon image tagged `practice = true` in sarpedon.conf. They are kept OUT of the
// main leaderboard (team totals, image count, play time, image filter, team chart) and
// shown on their own Practice pages instead, grouped by the course unit levelsvc stores
// for each practice. Structure (names, units, level labels, thresholds) comes from
// levelsvc; progress comes from the latest scores in the scoreboard collection.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func isPracticeImage(name string) bool {
	for _, im := range sarpConfig.Image {
		if im.Name == name {
			return im.Practice
		}
	}
	return false
}

// splitPractice separates competition scores from practice-level scores.
func splitPractice(scores []scoreEntry) (comp, prac []scoreEntry) {
	for _, s := range scores {
		if isPracticeImage(s.Image.Name) {
			prac = append(prac, s)
		} else {
			comp = append(comp, s)
		}
	}
	return comp, prac
}

type practiceLevel struct {
	Level     int    `json:"level"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	Threshold int    `json:"threshold"`
}

type practiceComp struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Unit     int             `json:"unit"`
	Practice bool            `json:"practice"`
	Hidden   bool            `json:"hidden"`
	Private  bool            `json:"private"`
	Levels   []practiceLevel `json:"levelInfo"`
	Kind     string          `json:"kind"` // category: cisco, windows or linux (levelsvc; empty = cisco)
	Short    string          `json:"-"`    // column label, e.g. "13"
}

// practiceCategory is one tab on /practice. Cisco levels are Packet Tracer activities; Windows and
// Linux levels are scenarios PT Comp sets up inside a student's practice VM.
type practiceCategory struct {
	Key, Name, About, Cell string
}

var practiceCategories = []practiceCategory{
	{"cisco", "Cisco", "Packet Tracer practice, one per course module. A level is cleared when Packet Tracer shows it complete.",
		"that team's Packet Tracer completion"},
	{"windows", "Windows", "Windows 11 / Server practice: PT Comp sets the problems up in the student's Windows practice VM. " +
		"A level is cleared when its problems are fixed.", "the share of that level's problems the team has fixed"},
	{"linux", "Linux", "Linux practice: PT Comp sets the problems up in the student's Linux practice VM. " +
		"A level is cleared when its problems are fixed.", "the share of that level's problems the team has fixed"},
}

// categoryOf returns a practice's category (an unknown kind counts as Cisco).
func categoryOf(cp practiceComp) practiceCategory {
	for _, pc := range practiceCategories {
		if pc.Key == cp.Kind {
			return pc
		}
	}
	return practiceCategories[0]
}

// Category is the practice's category name, for templates ("Windows").
func (cp practiceComp) Category() string { return categoryOf(cp).Name }

func categoryIndex(cp practiceComp) int {
	for i, pc := range practiceCategories {
		if pc.Key == categoryOf(cp).Key {
			return i
		}
	}
	return 0
}

// Practice names: "U4.M13. VLANs" (unit 4, NetAcad module 13), "U16. HSRP · FHRP Concepts"
// (no module number), or the older "13 · VLANs".
var (
	unitMod  = regexp.MustCompile(`^\s*U(\d+)\.M(\d+)\.?`)
	unitOnly = regexp.MustCompile(`^\s*U\d+\.\s*`)
	leadNum  = regexp.MustCompile(`^\s*(\d+)`)
)

// compNum is the module number used to order practices within a unit.
func compNum(name string) int {
	if m := unitMod.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[2])
		return n
	}
	if m := leadNum.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 1 << 30
}

// compShort is the practice's column label on /practice: "M13", or a short title.
func compShort(name string) string {
	if m := unitMod.FindStringSubmatch(name); m != nil {
		return "M" + m[2]
	}
	if m := leadNum.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return strings.TrimSpace(strings.SplitN(unitOnly.ReplaceAllString(name, ""), "·", 2)[0]) // "U16. HSRP · …" -> "HSRP"
}

var (
	practiceMu    sync.Mutex
	practiceCache []practiceComp
	practiceAt    time.Time
)

// fetchPractices returns the released practices (practice, not hidden, not testers-only),
// ordered by unit then module number. Cached for 30s: the pages are public.
func fetchPractices() ([]practiceComp, error) {
	practiceMu.Lock()
	defer practiceMu.Unlock()
	if practiceCache != nil && time.Since(practiceAt) < 30*time.Second {
		return practiceCache, nil
	}
	if sarpConfig.LevelsvcToken == "" {
		return nil, errors.New("levelsvc is not configured (levelsvcToken)")
	}
	var out struct {
		Competitions []practiceComp `json:"competitions"`
	}
	if err := lvGetJSON("/admin/competitions", &out); err != nil {
		return nil, err
	}
	list := []practiceComp{}
	for _, cp := range out.Competitions {
		if !cp.Practice || cp.Hidden || cp.Private || len(cp.Levels) == 0 {
			continue
		}
		cp.Short = compShort(cp.Name)
		cp.Kind = categoryOf(cp).Key
		sort.SliceStable(cp.Levels, func(i, j int) bool { return cp.Levels[i].Level < cp.Levels[j].Level })
		list = append(list, cp)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if ci, cj := categoryIndex(list[i]), categoryIndex(list[j]); ci != cj {
			return ci < cj
		}
		ui, uj := list[i].Unit, list[j].Unit
		if ui == 0 {
			ui = 1 << 30 // no unit -> last
		}
		if uj == 0 {
			uj = 1 << 30
		}
		if ui != uj {
			return ui < uj
		}
		if compNum(list[i].Name) != compNum(list[j].Name) {
			return compNum(list[i].Name) < compNum(list[j].Name)
		}
		return list[i].Name < list[j].Name
	})
	practiceCache, practiceAt = list, time.Now()
	return list, nil
}

// pctOf is a level's completion % as Packet Tracer shows it: items done / items total
// (the PT agent reports item counts); older completion-only reports carry it as points.
func pctOf(e scoreEntry) int {
	if e.Vulns.VulnsTotal > 0 {
		return e.Vulns.VulnsScored * 100 / e.Vulns.VulnsTotal
	}
	if e.Points > 100 {
		return 100
	}
	if e.Points < 0 {
		return 0
	}
	return e.Points
}

type levelCell struct {
	Started, Cleared bool
	Pct              int
	Class            string
}

func cellFor(e scoreEntry, ok bool, threshold int) levelCell {
	if !ok {
		return levelCell{Class: "pc-none"}
	}
	c := levelCell{Started: true, Pct: pctOf(e)}
	if threshold <= 0 {
		threshold = 100
	}
	switch {
	case c.Pct >= threshold:
		c.Cleared, c.Class = true, "pc-full"
	case c.Pct > 0:
		c.Class = "pc-part"
	default:
		c.Class = "pc-started"
	}
	return c
}

// cellFrac is one level's fractional completion toward the leaderboard: a cleared level counts
// as a whole 1.0, and a level still in progress counts its completion percentage (so 75% = 0.75).
// Weekly Sprint VM practices can exceed 100% (pre-release x2, see applySprintBonus, up to 200%);
// that bonus is kept (a full-score-before-release level counts 2.0, shown as 2/1), not clamped to 1.
func cellFrac(c levelCell) float64 {
	p := c.Pct
	if p > 200 {
		p = 200
	}
	if p < 0 {
		p = 0
	}
	if c.Cleared && p < 100 {
		return 1 // cleared at a threshold below 100%
	}
	return float64(p) / 100
}

// fmtProg renders a fractional level count for the board: whole numbers stay plain ("3"), and
// fractions show up to two decimals with trailing zeros trimmed ("0.75", "2.5").
func fmtProg(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		s = "0"
	}
	return s
}

// practiceScores maps image -> team alias -> latest score, for practice images only, with the
// Weekly Sprint "pre-release x2" bonus applied to Windows/Linux practice images (applySprintBonus).
func practiceScores() (map[string]map[string]scoreEntry, error) {
	top, err := getTop()
	if err != nil {
		return nil, err
	}
	m := map[string]map[string]scoreEntry{}
	for _, s := range top {
		if !isPracticeImage(s.Image.Name) {
			continue
		}
		if m[s.Image.Name] == nil {
			m[s.Image.Name] = map[string]scoreEntry{}
		}
		m[s.Image.Name][s.Team.Alias] = s
	}
	applySprintBonus(m)
	return m, nil
}

// applySprintBonus doubles PRE-RELEASE progress on Windows/Linux practice images (the Weekly
// Sprint incentive to finish a VM before its answer key drops). For each team on such an image:
//
//	adjustedScored = before + overall
//
// where `before` is the team's best VulnsScored reached before the practice's release time and
// `overall` its best ever (== 2*before + post-release gains). An image with no release configured,
// or whose release is still in the future, is fully doubled (before == overall). Cisco and non-
// practice images are left untouched. VulnsTotal is unchanged, so pctOf may exceed 100% (up to
// 200%) for VM practices — intended. Releases are configured in sarpedon.conf ([[release]]).
func applySprintBonus(m map[string]map[string]scoreEntry) {
	comps, err := fetchPractices() // cached ~30s; callers have already warmed it
	if err != nil {
		return // without the comp list we can't tell Windows/Linux from Cisco -> leave scores raw
	}
	relByComp := map[string]time.Time{}
	for _, r := range sarpConfig.Release {
		relByComp[r.Comp] = r.Time
	}
	kindByImage := map[string]string{}
	releaseByImage := map[string]time.Time{}
	for _, cp := range comps {
		t, hasRelease := relByComp[cp.ID]
		for _, l := range cp.Levels {
			kindByImage[l.Image] = cp.Kind
			if hasRelease {
				releaseByImage[l.Image] = t
			}
		}
	}
	now := time.Now()
	for image, teams := range m {
		if k := kindByImage[image]; k != "windows" && k != "linux" {
			keepBestScore(image, teams) // Cisco: no bonus, but a clear is never lost to a later 0% report
			continue
		}
		R, released := releaseByImage[image]
		if released && !R.After(now) {
			// released: split each team's pre/post-release progress from the full history
			adj, err := adjustedScoredByImage(image, R)
			if err != nil {
				continue // on a history read error, leave this image's scores raw
			}
			for alias, e := range teams {
				if v, ok := adj[alias]; ok {
					e.Vulns.VulnsScored = v
				} else {
					e.Vulns.VulnsScored *= 2 // no history row matched -> treat as all pre-release
				}
				teams[alias] = e
			}
		} else {
			// unreleased (or release still in the future): everything is pre-release -> doubled
			for alias, e := range teams {
				e.Vulns.VulnsScored *= 2
				teams[alias] = e
			}
		}
	}
}

// keepBestScore raises each team's latest VulnsScored on a practice image to the best it has ever
// reported. The board shows the latest report per (image, team), and the PT agent can report 0
// again after a level was completed (the activity is reopened or reset right after finishing),
// which wiped the clear. Windows/Linux images already use best-ever via adjustedScoredByImage.
func keepBestScore(image string, teams map[string]scoreEntry) {
	best, err := bestScoredByImage(image)
	if err != nil {
		return // history unreadable -> leave the latest scores as reported
	}
	for alias, e := range teams {
		if b := best[alias]; b > e.Vulns.VulnsScored {
			e.Vulns.VulnsScored = b
			if b > 0 && e.Vulns.VulnsTotal < b {
				e.Vulns.VulnsTotal = b
			}
			teams[alias] = e
		}
	}
}

var (
	bestMu    sync.Mutex
	bestCache = map[string]map[string]int{}
	bestAt    = map[string]time.Time{}
)

// bestScoredByImage returns each team alias's best-ever VulnsScored on one image (cached 30s).
func bestScoredByImage(image string) (map[string]int, error) {
	bestMu.Lock()
	defer bestMu.Unlock()
	if m, ok := bestCache[image]; ok && time.Since(bestAt[image]) < 30*time.Second {
		return m, nil
	}
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("scores")
	cur, err := coll.Aggregate(context.TODO(), mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"image.name": image}}},
		{{Key: "$group", Value: bson.M{"_id": "$team.alias", "best": bson.M{"$max": "$vulns.vulnsscored"}}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Alias string `bson:"_id"`
		Best  int    `bson:"best"`
	}
	if err := cur.All(context.TODO(), &rows); err != nil {
		return nil, err
	}
	m := make(map[string]int, len(rows))
	for _, r := range rows {
		m[r.Alias] = r.Best
	}
	bestCache[image], bestAt[image] = m, time.Now()
	return m, nil
}

// adjustedScoredByImage returns each team alias's adjusted VulnsScored for one RELEASED
// Windows/Linux practice image: before + overall, from the full report history (one read for
// all teams), where before = best VulnsScored before `release` and overall = best ever.
func adjustedScoredByImage(image string, release time.Time) (map[string]int, error) {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("scores")
	historyIndexOnce.Do(func() {
		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{Keys: bson.D{{Key: "image.name", Value: 1}, {Key: "time", Value: 1}}})
	})
	opts := options.Find().SetProjection(bson.M{"time": 1, "team.alias": 1, "vulns.vulnsscored": 1})
	cur, err := coll.Find(context.TODO(), bson.M{"image.name": image}, opts)
	if err != nil {
		return nil, err
	}
	var rows []scoreEntry
	if err := cur.All(context.TODO(), &rows); err != nil {
		return nil, err
	}
	before := map[string]int{}
	overall := map[string]int{}
	for _, e := range rows {
		a := e.Team.Alias
		s := e.Vulns.VulnsScored
		if s > overall[a] {
			overall[a] = s
		}
		if e.Time.Before(release) && s > before[a] {
			before[a] = s
		}
	}
	out := make(map[string]int, len(overall))
	for a, ov := range overall {
		out[a] = before[a] + ov
	}
	return out, nil
}

// compProgress summarises one team's progress through one practice. Done counts fully-cleared
// levels (for the badge colour and "module complete"); Prog is the fractional progress shown on
// the board, where a partly-finished level counts its completion (75% done → 0.75).
type compProgress struct {
	Comp           practiceComp
	Cells          []levelCell
	Done, Started  int
	Total          int
	Prog           float64
	Class, Summary string
}

func (p compProgress) ProgStr() string { return fmtProg(p.Prog) }

func progressFor(cp practiceComp, team string, sc map[string]map[string]scoreEntry) compProgress {
	p := compProgress{Comp: cp, Total: len(cp.Levels)}
	for _, l := range cp.Levels {
		e, ok := sc[l.Image][team]
		c := cellFor(e, ok, l.Threshold)
		if c.Started {
			p.Started++
		}
		if c.Cleared {
			p.Done++
		}
		p.Prog += cellFrac(c)
		p.Cells = append(p.Cells, c)
	}
	switch {
	case p.Started == 0:
		p.Class, p.Summary = "pc-none", "–"
	case p.Done == p.Total:
		p.Class = "pc-full"
	case p.Done > 0:
		p.Class = "pc-part"
	default:
		p.Class = "pc-started"
	}
	if p.Started > 0 {
		p.Summary = fmtProg(p.Prog) + "/" + strconv.Itoa(p.Total)
	}
	return p
}

type unitGroup struct {
	Unit  int
	Label string // "Unit 4"; practices without a unit: "Other" (Cisco) or the category name
	Comps []practiceComp
}

func groupByUnit(list []practiceComp) []unitGroup {
	groups := []unitGroup{}
	for _, cp := range list {
		if n := len(groups); n > 0 && groups[n-1].Unit == cp.Unit {
			groups[n-1].Comps = append(groups[n-1].Comps, cp)
			continue
		}
		label := "Unit " + strconv.Itoa(cp.Unit)
		if cp.Unit == 0 {
			label = "Other"
			if cat := categoryOf(cp); cat.Key != "cisco" {
				label = cat.Name
			}
		}
		groups = append(groups, unitGroup{Unit: cp.Unit, Label: label, Comps: []practiceComp{cp}})
	}
	return groups
}

type categoryTab struct {
	practiceCategory
	Count  int
	Active bool
}

// practiceTabs picks the category to show (?cat=, else the first one with practices) and keeps
// only its practices.
func practiceTabs(all []practiceComp, want string) ([]categoryTab, practiceCategory, []practiceComp) {
	count := map[string]int{}
	for _, cp := range all {
		count[cp.Kind]++
	}
	pick := practiceCategories[0]
	found := false
	for _, pc := range practiceCategories {
		if pc.Key == want {
			pick, found = pc, true
		}
	}
	if !found {
		for _, pc := range practiceCategories {
			if count[pc.Key] > 0 {
				pick = pc
				break
			}
		}
	}
	tabs := []categoryTab{}
	for _, pc := range practiceCategories {
		tabs = append(tabs, categoryTab{pc, count[pc.Key], pc.Key == pick.Key})
	}
	list := []practiceComp{}
	for _, cp := range all {
		if cp.Kind == pick.Key {
			list = append(list, cp)
		}
	}
	return tabs, pick, list
}

type practiceRow struct {
	Team        string
	Cols        []compProgress
	Done, Total int
	Prog        float64
	Started     int
}

func (r practiceRow) ProgStr() string { return fmtProg(r.Prog) }

// viewPractice: one category's tab (?cat=): teams x practices (grouped by unit), each cell =
// levels cleared / levels.
func viewPractice(c *gin.Context) {
	all, err := fetchPractices()
	if err != nil {
		c.HTML(http.StatusOK, "practice.html", pageData(c, "Practice", gin.H{"err": err.Error()}))
		return
	}
	tabs, cat, list := practiceTabs(all, c.Query("cat"))
	sc, err := practiceScores()
	if err != nil {
		panic(err)
	}
	teams := map[string]bool{}
	for _, cp := range list {
		for _, l := range cp.Levels {
			for alias := range sc[l.Image] {
				teams[alias] = true
			}
		}
	}
	rows := []practiceRow{}
	levels := 0
	for _, cp := range list {
		levels += len(cp.Levels)
	}
	for alias := range teams {
		r := practiceRow{Team: alias, Total: levels}
		for _, cp := range list {
			p := progressFor(cp, alias, sc)
			r.Cols = append(r.Cols, p)
			r.Done += p.Done
			r.Prog += p.Prog
			r.Started += p.Started
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Prog != rows[j].Prog {
			return rows[i].Prog > rows[j].Prog
		}
		if rows[i].Started != rows[j].Started {
			return rows[i].Started > rows[j].Started
		}
		return rows[i].Team < rows[j].Team
	})
	c.HTML(http.StatusOK, "practice.html", pageData(c, "Practice", gin.H{
		"tabs": tabs, "cat": cat, "units": groupByUnit(list), "comps": list, "rows": rows, "levels": levels}))
}

type practiceCompRow struct {
	Team          string
	Cells         []levelCell
	Done, Started int
	Prog          float64
	PctSum        int
}

func (r practiceCompRow) ProgStr() string { return fmtProg(r.Prog) }

// viewPracticeComp: one practice, teams x levels, each cell = completion %.
func viewPracticeComp(c *gin.Context) {
	id := c.Param("comp")
	list, err := fetchPractices()
	if err != nil {
		c.HTML(http.StatusOK, "practice_comp.html", pageData(c, "Practice", gin.H{"err": err.Error()}))
		return
	}
	var cp *practiceComp
	for i := range list {
		if list[i].ID == id {
			cp = &list[i]
		}
	}
	if cp == nil {
		errorOutGraceful(c, errors.New("No such practice: "+id))
		return
	}
	sc, err := practiceScores()
	if err != nil {
		panic(err)
	}
	teams := map[string]bool{}
	for _, l := range cp.Levels {
		for alias := range sc[l.Image] {
			teams[alias] = true
		}
	}
	rows := []practiceCompRow{}
	for alias := range teams {
		p := progressFor(*cp, alias, sc)
		r := practiceCompRow{Team: alias, Cells: p.Cells, Done: p.Done, Prog: p.Prog, Started: p.Started}
		for _, cell := range p.Cells {
			r.PctSum += cell.Pct
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Prog != rows[j].Prog {
			return rows[i].Prog > rows[j].Prog
		}
		if rows[i].PctSum != rows[j].PctSum {
			return rows[i].PctSum > rows[j].PctSum
		}
		return rows[i].Team < rows[j].Team
	})
	series, nteams, terr := cachedModuleTimeline(*cp)
	if terr != nil {
		series = nil
	}
	teamVulns := map[string][]levelVulnView{}
	if getUser(c) != "" { // admin only: the not-fixed list is an answer key
		teamList := make([]string, 0, len(rows))
		for _, r := range rows {
			teamList = append(teamList, r.Team)
		}
		teamVulns = compTeamVulns(*cp, teamList, sc)
	}
	c.HTML(http.StatusOK, "practice_comp.html", pageData(c, "Practice: "+cp.Name, gin.H{"comp": cp, "rows": rows,
		"cat": categoryOf(*cp), "timeline": series, "timelineTeams": nteams, "teamVulns": teamVulns}))
}

// teamPractice lists the practices a team has started, for the team page.
func teamPractice(alias string) ([]compProgress, string) {
	list, err := fetchPractices()
	if err != nil {
		return nil, err.Error()
	}
	sc, err := practiceScores()
	if err != nil {
		return nil, err.Error()
	}
	out := []compProgress{}
	for _, cp := range list {
		if p := progressFor(cp, alias, sc); p.Started > 0 {
			out = append(out, p)
		}
	}
	return out, ""
}

// ---------------- timelines ----------------
// Every report the PT agent sends is kept in the scores collection, so a practice's whole
// history is there. The agent reports every few seconds while a student works, so the
// series keep only the points where a value changes.

type practicePoint struct {
	Time  time.Time
	Team  string // alias
	Image string
	Pct   int
}

var historyIndexOnce sync.Once

// practiceHistory returns the reports for the given practice images (every team, or one
// team by alias), oldest first, reduced to the fields the timelines need.
func practiceHistory(images []string, teamAlias string) ([]practicePoint, error) {
	if len(images) == 0 {
		return nil, nil
	}
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("scores")
	historyIndexOnce.Do(func() { // lets the image filter use an index instead of a full scan
		coll.Indexes().CreateOne(context.TODO(), mongo.IndexModel{Keys: bson.D{{Key: "image.name", Value: 1}, {Key: "time", Value: 1}}})
	})
	filter := bson.M{"image.name": bson.M{"$in": images}}
	if teamAlias != "" {
		filter["team.alias"] = teamAlias
	}
	opts := options.Find().SetSort(bson.D{{Key: "time", Value: 1}}).SetProjection(bson.M{
		"time": 1, "team.alias": 1, "image.name": 1, "vulns.vulnsscored": 1, "vulns.vulnstotal": 1, "points": 1})
	cur, err := coll.Find(context.TODO(), filter, opts)
	if err != nil {
		return nil, err
	}
	var rows []scoreEntry
	if err := cur.All(context.TODO(), &rows); err != nil {
		return nil, err
	}
	out := make([]practicePoint, 0, len(rows))
	for _, e := range rows {
		out = append(out, practicePoint{Time: e.Time, Team: e.Team.Alias, Image: e.Image.Name, Pct: pctOf(e)})
	}
	return out, nil
}

type timeSeries struct {
	Name string     `json:"name"`
	Data [][2]int64 `json:"data"` // [unix ms, percent]
}

// extendToNow carries each series' last value to the present, so a step line shows where a
// team stands now rather than stopping at its last change.
func extendToNow(list []timeSeries) {
	now := time.Now().UnixMilli()
	for i := range list {
		if d := list[i].Data; len(d) > 0 && d[len(d)-1][0] < now {
			list[i].Data = append(d, [2]int64{now, d[len(d)-1][1]})
		}
	}
}

// moduleTimeline: one series per team = its completion of the whole module over time (the
// mean of its level percentages). At most maxSeries teams: those furthest along, then the
// earliest to get there. Returns the series and the number of teams that have started.
func moduleTimeline(cp practiceComp, hist []practicePoint, maxSeries int) ([]timeSeries, int) {
	n := len(cp.Levels)
	if n == 0 {
		return nil, 0
	}
	idx := map[string]int{}
	for i, l := range cp.Levels {
		idx[l.Image] = i
	}
	type state struct {
		pcts    []int
		last    int64
		reached time.Time
		s       timeSeries
	}
	teams := map[string]*state{}
	for _, p := range hist {
		i, ok := idx[p.Image]
		if !ok {
			continue
		}
		t := teams[p.Team]
		if t == nil {
			t = &state{pcts: make([]int, n), last: -1, s: timeSeries{Name: p.Team}}
			teams[p.Team] = t
		}
		t.pcts[i] = p.Pct
		sum := 0
		for _, v := range t.pcts {
			sum += v
		}
		if v := int64(sum / n); v != t.last {
			t.s.Data = append(t.s.Data, [2]int64{p.Time.UnixMilli(), v})
			t.last, t.reached = v, p.Time
		}
	}
	all := make([]*state, 0, len(teams))
	for _, t := range teams {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].last != all[j].last {
			return all[i].last > all[j].last
		}
		if !all[i].reached.Equal(all[j].reached) {
			return all[i].reached.Before(all[j].reached)
		}
		return all[i].s.Name < all[j].s.Name
	})
	out := []timeSeries{}
	for i, t := range all {
		if i >= maxSeries {
			break
		}
		out = append(out, t.s)
	}
	extendToNow(out)
	return out, len(all)
}

// levelTimeline: one series per level a team has started (hist = that team's reports).
func levelTimeline(cp practiceComp, hist []practicePoint) []timeSeries {
	out := []timeSeries{}
	for _, l := range cp.Levels {
		s := timeSeries{Name: "L" + strconv.Itoa(l.Level) + " " + l.Name}
		last := -1
		for _, p := range hist {
			if p.Image == l.Image && p.Pct != last {
				s.Data = append(s.Data, [2]int64{p.Time.UnixMilli(), int64(p.Pct)})
				last = p.Pct
			}
		}
		if len(s.Data) > 0 {
			out = append(out, s)
		}
	}
	extendToNow(out)
	return out
}

var (
	timelineMu    sync.Mutex
	timelineCache = map[string]struct {
		at     time.Time
		series []timeSeries
		teams  int
	}{}
)

// cachedModuleTimeline: the practice page is public, so its chart is cached for 30s.
func cachedModuleTimeline(cp practiceComp) ([]timeSeries, int, error) {
	timelineMu.Lock()
	defer timelineMu.Unlock()
	if c, ok := timelineCache[cp.ID]; ok && time.Since(c.at) < 30*time.Second {
		return c.series, c.teams, nil
	}
	images := []string{}
	for _, l := range cp.Levels {
		images = append(images, l.Image)
	}
	hist, err := practiceHistory(images, "")
	if err != nil {
		return nil, 0, err
	}
	series, teams := moduleTimeline(cp, hist, 8) // 8 = the validated categorical palette
	timelineCache[cp.ID] = struct {
		at     time.Time
		series []timeSeries
		teams  int
	}{time.Now(), series, teams}
	return series, teams, nil
}

// teamTimelines: practice ID -> JSON of that team's per-level series, for the team page.
func teamTimelines(alias string, prog []compProgress) map[string]string {
	out := map[string]string{}
	images := []string{}
	for _, p := range prog {
		for _, l := range p.Comp.Levels {
			images = append(images, l.Image)
		}
	}
	hist, err := practiceHistory(images, alias)
	if err != nil {
		return out
	}
	for _, p := range prog {
		b, _ := json.Marshal(levelTimeline(p.Comp, hist))
		out[p.Comp.ID] = string(b)
	}
	return out
}

// ---------------- machine-readable feed ----------------

// viewPracticeJSON: practice structure + every team's progress (the same data as the
// public Practice page), for the Discord bot.
func viewPracticeJSON(c *gin.Context) {
	list, err := fetchPractices()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	sc, err := practiceScores()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	practices := []gin.H{}
	teams := map[string]bool{}
	for _, cp := range list {
		levels := []gin.H{}
		for _, l := range cp.Levels {
			levels = append(levels, gin.H{"level": l.Level, "name": l.Name, "threshold": l.Threshold})
			for alias := range sc[l.Image] {
				teams[alias] = true
			}
		}
		practices = append(practices, gin.H{"id": cp.ID, "name": cp.Name, "unit": cp.Unit, "kind": cp.Kind, "levels": levels})
	}
	aliases := make([]string, 0, len(teams))
	for a := range teams {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	out := []gin.H{}
	for _, alias := range aliases {
		progress := []gin.H{}
		for _, cp := range list {
			p := progressFor(cp, alias, sc)
			if p.Started == 0 {
				continue
			}
			cleared, pcts := []int{}, []int{}
			for i, cell := range p.Cells {
				if cell.Cleared {
					cleared = append(cleared, cp.Levels[i].Level)
				}
				if cell.Started {
					pcts = append(pcts, cell.Pct)
				} else {
					pcts = append(pcts, -1)
				}
			}
			progress = append(progress, gin.H{"id": cp.ID, "done": p.Done, "progress": p.Prog, "total": p.Total, "cleared": cleared, "pct": pcts})
		}
		out = append(out, gin.H{"team": alias, "practices": progress})
	}
	c.JSON(http.StatusOK, gin.H{"generated": time.Now().UTC().Format(time.RFC3339), "practices": practices, "teams": out})
}

// ---------------- admin per-vuln view (which practice vulns a team has / hasn't fixed) ----------------
// The agent reports only the FOUND items (and any penalties incurred), so the full item list comes
// from levelsvc (the scenario's checks). Admin-only: the not-fixed list would be an answer key.

type lvVulnDef struct {
	Message string `json:"message"`
	Points  int    `json:"points"`
}
type lvLevelVulns struct {
	Level int         `json:"level"`
	Name  string      `json:"name"`
	Image string      `json:"image"`
	Vulns []lvVulnDef `json:"vulns"`
}

// vulnStat is one scored item with whether this team currently has it. For a penalty, Found means
// the team triggered it (bad).
type vulnStat struct {
	Message string
	Points  int
	Penalty bool
	Found   bool
}
type levelVulnView struct {
	Level        int
	Name         string
	Found, Total int
	Vulns        []vulnStat
}

var (
	vulnDefMu    sync.Mutex
	vulnDefCache = map[string][]lvLevelVulns{}
	vulnDefAt    = map[string]time.Time{}
)

// practiceVulnDefs fetches a competition's scored-item list from levelsvc (cached 30s).
func practiceVulnDefs(comp string) ([]lvLevelVulns, error) {
	vulnDefMu.Lock()
	defer vulnDefMu.Unlock()
	if v, ok := vulnDefCache[comp]; ok && time.Since(vulnDefAt[comp]) < 30*time.Second {
		return v, nil
	}
	if sarpConfig.LevelsvcToken == "" {
		return nil, errors.New("levelsvc is not configured (levelsvcToken)")
	}
	var out struct {
		Levels []lvLevelVulns `json:"levels"`
	}
	if err := lvGetJSON("/admin/vulns?comp="+url.QueryEscape(comp), &out); err != nil {
		return nil, err
	}
	vulnDefCache[comp], vulnDefAt[comp] = out.Levels, time.Now()
	return out.Levels, nil
}

// normVuln matches the agent's vuln-name transform (build_update replaces '-' with '/').
func normVuln(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "-", "/"))
}

// levelVulnsForComp builds one team's fixed/not-fixed view for one competition (its scenario items
// matched against the items in the team's latest score). Levels with no item list (Cisco) are skipped.
func levelVulnsForComp(defs []lvLevelVulns, alias string, sc map[string]map[string]scoreEntry) []levelVulnView {
	views := []levelVulnView{}
	for _, ld := range defs {
		if len(ld.Vulns) == 0 {
			continue
		}
		have := map[string]bool{}
		for _, vi := range sc[ld.Image][alias].Vulns.VulnItems {
			have[normVuln(vi.VulnText)] = true
		}
		v := levelVulnView{Level: ld.Level, Name: ld.Name}
		for _, d := range ld.Vulns {
			st := vulnStat{Message: d.Message, Points: d.Points, Penalty: d.Points < 0, Found: have[normVuln(d.Message)]}
			if !st.Penalty {
				v.Total++
				if st.Found {
					v.Found++
				}
			}
			v.Vulns = append(v.Vulns, st)
		}
		views = append(views, v)
	}
	return views
}

// practiceVulnViews: for the team page, each practice the team started -> its fixed/not-fixed view.
func practiceVulnViews(alias string, prog []compProgress, sc map[string]map[string]scoreEntry) map[string][]levelVulnView {
	out := map[string][]levelVulnView{}
	for _, p := range prog {
		defs, err := practiceVulnDefs(p.Comp.ID)
		if err != nil {
			continue
		}
		if v := levelVulnsForComp(defs, alias, sc); len(v) > 0 {
			out[p.Comp.ID] = v
		}
	}
	return out
}

// compTeamVulns: for one practice's page, each team's fixed/not-fixed view (keyed by team alias).
func compTeamVulns(comp practiceComp, teams []string, sc map[string]map[string]scoreEntry) map[string][]levelVulnView {
	defs, err := practiceVulnDefs(comp.ID)
	if err != nil {
		return map[string][]levelVulnView{}
	}
	out := map[string][]levelVulnView{}
	for _, t := range teams {
		if v := levelVulnsForComp(defs, t, sc); len(v) > 0 {
			out[t] = v
		}
	}
	return out
}
