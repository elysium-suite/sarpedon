package main

// Weekly Sprint view. The Cisco practices are the SAME unit practices shown on /practice, just
// regrouped by week for two divisions (High School, Middle School) with different week->unit maps.
// The Windows/Linux hardening practices are shown as their own weekly section (not divided).
// Nothing new is scored here; progress is the same practiceScores() the /practice page uses.

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Middle-School roster (team aliases). Every team not listed here is High School. Overridable from
// sarpedon.conf (`sprintMs = ["alias", ...]`); this built-in default is the initial roster.
var defaultMSTeams = []string{
	"amych", "caryi", "evazh", "aidye", "dhrep", "moupe",
	"gabfi", "lukzh", "ghefi", "brile", "mahka", "andxi", "jiule",
}

func msTeamSet() map[string]bool {
	list := sarpConfig.SprintMS
	if len(list) == 0 {
		list = defaultMSTeams
	}
	m := map[string]bool{}
	for _, a := range list {
		m[strings.ToLower(strings.TrimSpace(a))] = true
	}
	return m
}

func divisionOf(alias string, ms map[string]bool) string {
	if ms[strings.ToLower(alias)] {
		return "ms"
	}
	return "hs"
}

type sprintWeek struct {
	Week  int
	Units []int
}

// High School: the first sprint covers units 1-4, then one unit a week (unit 10 is skipped),
// finishing with two units in the last two weeks.
var hsSprintWeeks = []sprintWeek{
	{1, []int{1, 2, 3, 4}}, {2, []int{5}}, {3, []int{6}}, {4, []int{7}}, {5, []int{8}},
	{6, []int{9}}, {7, []int{11}}, {8, []int{12}}, {9, []int{13, 14}}, {10, []int{15, 16}},
}

// Middle School: one unit per week, over whichever units actually have practices.
func msSprintWeeks(units []int) []sprintWeek {
	out := []sprintWeek{}
	for i, u := range units {
		out = append(out, sprintWeek{Week: i + 1, Units: []int{u}})
	}
	return out
}

// unitsLabel: "Unit 5", "Units 1–4" (consecutive), or "Units 13, 15" (not).
func unitsLabel(units []int) string {
	if len(units) == 0 {
		return ""
	}
	if len(units) == 1 {
		return "Unit " + strconv.Itoa(units[0])
	}
	consecutive := true
	for i := 1; i < len(units); i++ {
		if units[i] != units[i-1]+1 {
			consecutive = false
			break
		}
	}
	if consecutive {
		return "Units " + strconv.Itoa(units[0]) + "–" + strconv.Itoa(units[len(units)-1])
	}
	parts := make([]string, len(units))
	for i, u := range units {
		parts[i] = strconv.Itoa(u)
	}
	return "Units " + strings.Join(parts, ", ")
}

var vmWeekRe = regexp.MustCompile(`^sprint0*(\d+)-`)

// sprintWeekOf reads the week number from a VM practice's id ("sprint02-..." -> 2).
func sprintWeekOf(cp practiceComp) int {
	if m := vmWeekRe.FindStringSubmatch(cp.ID); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// vmWeekTheme pulls the theme from a VM practice name: "Week 1 Advanced: Accounts (Linux)" -> "Accounts".
func vmWeekTheme(name string) string {
	if i := strings.Index(name, ": "); i >= 0 {
		rest := name[i+2:]
		if j := strings.Index(rest, " ("); j >= 0 {
			rest = rest[:j]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

type sprintCell struct {
	Done, Total, Started int
	Prog                 float64
	Class, Summary       string
}

func (c sprintCell) ProgStr() string { return fmtProg(c.Prog) }

type sprintTeamRow struct {
	Team        string
	Division    string
	Cells       []sprintCell
	Done, Total int
	Prog        float64
}

func (r sprintTeamRow) ProgStr() string { return fmtProg(r.Prog) }

type sprintWeekCol struct {
	Week  int
	Label string // "Units 1–4" (Cisco) or "Accounts" (VM)
	Comps []practiceComp
}

type sprintDivision struct {
	Key, Name, About string
	Weeks            []sprintWeekCol
	Rows             []sprintTeamRow
	HasComps         bool
}

// aggregate one team's progress across a set of practices (a week's worth).
func sprintCellFor(comps []practiceComp, alias string, sc map[string]map[string]scoreEntry) sprintCell {
	cell := sprintCell{}
	for _, cp := range comps {
		p := progressFor(cp, alias, sc)
		cell.Done += p.Done
		cell.Total += p.Total
		cell.Prog += p.Prog
		cell.Started += p.Started
	}
	switch {
	case cell.Started == 0:
		cell.Class, cell.Summary = "pc-none", "–"
	case cell.Total > 0 && cell.Done == cell.Total:
		cell.Class = "pc-full"
	case cell.Done > 0:
		cell.Class = "pc-part"
	default:
		cell.Class = "pc-started"
	}
	if cell.Started > 0 {
		cell.Summary = fmtProg(cell.Prog) + "/" + strconv.Itoa(cell.Total)
	}
	return cell
}

func sortSprintRows(rows []sprintTeamRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Prog != rows[j].Prog {
			return rows[i].Prog > rows[j].Prog
		}
		return rows[i].Team < rows[j].Team
	})
}

// buildCiscoDivision regroups the Cisco unit practices into one division's weeks and rows.
func buildCiscoDivision(name, key, about string, weeks []sprintWeek, byUnit map[int][]practiceComp,
	sc map[string]map[string]scoreEntry, ms map[string]bool) sprintDivision {
	d := sprintDivision{Key: key, Name: name, About: about}
	weekComps := make([][]practiceComp, len(weeks))
	for i, w := range weeks {
		var comps []practiceComp
		for _, u := range w.Units {
			comps = append(comps, byUnit[u]...)
		}
		weekComps[i] = comps
		if len(comps) > 0 {
			d.HasComps = true
		}
		d.Weeks = append(d.Weeks, sprintWeekCol{Week: w.Week, Label: unitsLabel(w.Units), Comps: comps})
	}
	teamSet := map[string]bool{}
	for _, comps := range byUnit {
		for _, cp := range comps {
			for _, l := range cp.Levels {
				for alias := range sc[l.Image] {
					if divisionOf(alias, ms) == key {
						teamSet[alias] = true
					}
				}
			}
		}
	}
	for alias := range teamSet {
		r := sprintTeamRow{Team: alias, Division: key}
		for i := range weeks {
			cell := sprintCellFor(weekComps[i], alias, sc)
			r.Cells = append(r.Cells, cell)
			r.Done += cell.Done
			r.Prog += cell.Prog
			r.Total += cell.Total
		}
		d.Rows = append(d.Rows, r)
	}
	sortSprintRows(d.Rows)
	return d
}

// buildVMSection groups the Windows/Linux hardening practices by week (not divided by school level).
func buildVMSection(byWeek map[int][]practiceComp, sc map[string]map[string]scoreEntry, ms map[string]bool) sprintDivision {
	d := sprintDivision{Key: "vm", Name: "Hardening Sprints",
		About: "Windows & Linux practice VMs, one theme per week (both tracks, both OSes). Not split by division."}
	weekNums := make([]int, 0, len(byWeek))
	for w := range byWeek {
		weekNums = append(weekNums, w)
	}
	sort.Ints(weekNums)
	weekComps := make([][]practiceComp, len(weekNums))
	for i, wn := range weekNums {
		comps := byWeek[wn]
		weekComps[i] = comps
		label := "Week " + strconv.Itoa(wn)
		if len(comps) > 0 {
			if t := vmWeekTheme(comps[0].Name); t != "" {
				label = t
			}
			d.HasComps = true
		}
		d.Weeks = append(d.Weeks, sprintWeekCol{Week: wn, Label: label, Comps: comps})
	}
	teamSet := map[string]bool{}
	for _, comps := range byWeek {
		for _, cp := range comps {
			for _, l := range cp.Levels {
				for alias := range sc[l.Image] {
					teamSet[alias] = true
				}
			}
		}
	}
	for alias := range teamSet {
		r := sprintTeamRow{Team: alias, Division: strings.ToUpper(divisionOf(alias, ms))}
		for i := range weekNums {
			cell := sprintCellFor(weekComps[i], alias, sc)
			r.Cells = append(r.Cells, cell)
			r.Done += cell.Done
			r.Prog += cell.Prog
			r.Total += cell.Total
		}
		d.Rows = append(d.Rows, r)
	}
	sortSprintRows(d.Rows)
	return d
}

// viewSprint: the Weekly Sprint page — HS and MS Cisco divisions plus the VM hardening weeks.
func viewSprint(c *gin.Context) {
	all, err := fetchPractices()
	if err != nil {
		c.HTML(http.StatusOK, "sprint.html", pageData(c, "Weekly Sprint", gin.H{"err": err.Error()}))
		return
	}
	sc, err := practiceScores()
	if err != nil {
		panic(err)
	}
	ciscoByUnit := map[int][]practiceComp{}
	vmByWeek := map[int][]practiceComp{}
	unitSet := map[int]bool{}
	for _, cp := range all {
		if cp.Kind == "cisco" {
			if cp.Unit > 0 {
				ciscoByUnit[cp.Unit] = append(ciscoByUnit[cp.Unit], cp)
				unitSet[cp.Unit] = true
			}
			continue
		}
		if w := sprintWeekOf(cp); w > 0 {
			vmByWeek[w] = append(vmByWeek[w], cp)
		}
	}
	units := make([]int, 0, len(unitSet))
	for u := range unitSet {
		units = append(units, u)
	}
	sort.Ints(units)
	ms := msTeamSet()
	divisions := []sprintDivision{
		buildCiscoDivision("High School", "hs",
			"Cisco Packet Tracer practice. Sprint 1 is units 1–4; after that, one unit per week.",
			hsSprintWeeks, ciscoByUnit, sc, ms),
		buildCiscoDivision("Middle School", "ms",
			"Cisco Packet Tracer practice, one unit per week.",
			msSprintWeeks(units), ciscoByUnit, sc, ms),
	}
	vm := buildVMSection(vmByWeek, sc, ms)
	c.HTML(http.StatusOK, "sprint.html", pageData(c, "Weekly Sprint", gin.H{
		"divisions": divisions, "vm": vm}))
}
