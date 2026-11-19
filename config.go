package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// releaseData is one practice's answer-key release time, keyed by the levelsvc comp ID.
// In sarpedon.conf:
//   [[release]]
//   comp = "sprint01-accounts-adv-lin"
//   time = 2026-10-24T19:00:00-07:00
type releaseData struct {
	Comp string
	Time time.Time
}

type config struct {
	Event         string
	Password      string
	PlayTime      string
	Timezone      string
	DiscordHook   string
	Enforce       bool
	Timeout       int
	LevelsvcUrl   string   // base URL of the levelsvc competition engine (default http://127.0.0.1:8099)
	CommunityUrl  string   // base URL of the turtlebot community-image feed (default http://127.0.0.1:8098); the "Community images" page
	LevelsvcToken string   // levelsvc admin token, for the creator web pages
	SprintMS      []string // Weekly Sprint: team aliases in the Middle School division (everyone else is High School); empty = built-in default
	Release       []releaseData // Weekly Sprint: per-practice answer-key release times (by comp ID). Before a practice's release, its Windows/Linux points count x2 (see practice.go applySprintBonus).
	Admin         []adminData
	Image         []imageData
	Team          []teamData
	Alternate     alternateData
	AllowNewTeam  bool
}

func readConfig(conf *config) {
	fileContent, err := ioutil.ReadFile("./sarpedon.conf")
	if err != nil {
		log.Fatalln("Configuration file (./sarpedon.conf) not found:", err)
	}
	if _, err := toml.Decode(string(fileContent), &conf); err != nil {
		log.Fatalln(err)
	}
}

func checkConfig() {
	if sarpConfig.Password == "" {
		log.Fatalln("No password provided!")
	}
	if sarpConfig.Admin == nil {
		log.Fatalln("No admin account(s) provided!")
	}
	if sarpConfig.Image == nil {
		log.Fatalln("No images provided!")
	}
	if sarpConfig.Timezone == "" {
		log.Fatalln("No timezone provided!")
	}
	if sarpConfig.PlayTime != "" {
		if _, err := time.ParseDuration(sarpConfig.PlayTime); err != nil {
			log.Fatalln("Invalid duration for playtime: " + err.Error())
		}
	}
	if sarpConfig.Timeout == 0 {
		// Default timeout (in seconds)
		sarpConfig.Timeout = 15
	} else if sarpConfig.Timeout < 0 {
		// If a negative value, set no timeout
		sarpConfig.Timeout = 0
	}
	if sarpConfig.LevelsvcUrl == "" {
		sarpConfig.LevelsvcUrl = "http://127.0.0.1:8099"
	}
	for _, image := range sarpConfig.Image {
		if image.Name == "" {
			log.Fatalln("Image name is empty:", image)
		}
		matches := 0
		var dupeImage imageData
		for _, imageDupe := range sarpConfig.Image {
			if image.Name == imageDupe.Name {
				dupeImage = imageDupe
				matches++
			}
		}
		if matches > 1 {
			log.Fatalln("Duplicate image details found:", image, dupeImage)
		}
	}

	for i := range sarpConfig.Team {
		team := &sarpConfig.Team[i]
		if team.ID == "" {
			log.Fatalln("Team id is empty:", *team)
		}
		if team.Alias == "" {
			// Don't refuse to start over a missing alias: use the ID so scoring keeps running.
			log.Println("Team alias is empty, using the ID:", team.ID)
			team.Alias = team.ID
		}
		matches := 0
		var dupeTeam teamData
		for _, teamDupe := range sarpConfig.Team {
			if team.ID == teamDupe.ID {
				dupeTeam = teamDupe
				matches++
			}
			if team.Alias == teamDupe.Alias {
				dupeTeam = teamDupe
				matches++
			}
			if matches > 2 {
				log.Fatalln("Duplicate team details found:", team, dupeTeam)
			}
		}
	}

	if alternateCompletion {
		sarpConfig.Alternate.Map = make(map[string]bool)

		for _, ID := range sarpConfig.Alternate.Data {
			sarpConfig.Alternate.Map[ID] = true
		}
	}
}

// updateConfig saves a team that was just created. It never writes the in-memory config over the
// file: an admin may have edited sarpedon.conf (aliases!) while the server runs, and rewriting from
// memory would silently undo that. Instead it re-reads the file, adds any team the file lacks, and
// then adopts the file's team list, so hand-edited aliases take effect without a restart.
func updateConfig() {
	configMu.Lock()
	defer configMu.Unlock()
	var onDisk config
	if _, err := os.Stat("./sarpedon.conf"); err == nil {
		readConfig(&onDisk)
	} else {
		onDisk = sarpConfig
	}
	for _, t := range sarpConfig.Team {
		if !hasTeamID(onDisk.Team, t.ID) {
			onDisk.Team = append(onDisk.Team, t)
		}
	}
	var confBuffer bytes.Buffer
	toml.NewEncoder(&confBuffer).Encode(onDisk)
	ioutil.WriteFile("./sarpedon.conf", confBuffer.Bytes(), 0600)
	sarpConfig.Team = onDisk.Team
	fillEmptyAliases(sarpConfig.Team)
	confTeamsLoaded = fileModTime()
}

func hasTeamID(teams []teamData, id string) bool {
	for _, t := range teams {
		if t.ID == id {
			return true
		}
	}
	return false
}

// A team without an alias uses its ID (see checkConfig).
func fillEmptyAliases(teams []teamData) {
	for i := range teams {
		if teams[i].Alias == "" {
			teams[i].Alias = teams[i].ID
		}
	}
}

var (
	configMu        sync.Mutex
	confTeamsLoaded time.Time
)

func fileModTime() time.Time {
	st, err := os.Stat("./sarpedon.conf")
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

// refreshTeams re-reads the [[team]] list when sarpedon.conf changed on disk, so an alias an admin
// just edited shows up at once. A missing or unreadable file keeps the teams already in memory.
func refreshTeams() {
	mod := fileModTime()
	configMu.Lock()
	defer configMu.Unlock()
	if mod.IsZero() || mod.Equal(confTeamsLoaded) {
		return
	}
	confTeamsLoaded = mod
	data, err := ioutil.ReadFile("./sarpedon.conf")
	if err != nil {
		return
	}
	var onDisk config
	if _, err := toml.Decode(string(data), &onDisk); err != nil {
		log.Println("sarpedon.conf changed but could not be parsed; keeping the teams in memory:", err)
		return
	}
	fillEmptyAliases(onDisk.Team)
	for _, t := range sarpConfig.Team { // teams made since the file was written
		if !hasTeamID(onDisk.Team, t.ID) {
			onDisk.Team = append(onDisk.Team, t)
		}
	}
	sarpConfig.Team = onDisk.Team
}
