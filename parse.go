package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// decryptUpdate tries each image's per-image password first (so one image's leaked key
// can't forge another's score), then the global password. Returns plaintext + key used.
func decryptUpdate(crypt string) (string, string, error) {
	for _, img := range sarpConfig.Image {
		if img.Password == "" {
			continue
		}
		if p, err := decryptString(img.Password, crypt); err == nil {
			return p, img.Password, nil
		}
	}
	p, err := decryptString(sarpConfig.Password, crypt)
	return p, sarpConfig.Password, err
}

// imageForKey returns the image a per-image key belongs to ("" for the global key).
func imageForKey(key string) string {
	if key == sarpConfig.Password {
		return ""
	}
	for _, img := range sarpConfig.Image {
		if img.Password != "" && img.Password == key {
			return img.Name
		}
	}
	return ""
}

func parseUpdate(cryptUpdate, sourceIP string) (scoreEntry, error) {
	if cryptUpdate == "" || !validateString(cryptUpdate) {
		return scoreEntry{}, errors.New("Empty or invalid characters in cryptUpdate.")
	}
	cryptUpdate, err := hexDecode(cryptUpdate)
	if err != nil {
		return scoreEntry{}, errors.New("Error decoding hex input.")
	}
	plainUpdate, key, err := decryptUpdate(cryptUpdate)
	if err != nil {
		return scoreEntry{}, err
	}
	if err := validateUpdate(plainUpdate); err != nil {
		return scoreEntry{}, errors.Wrap(err, "Update validation failed")
	}

	d := updateDelimiter(plainUpdate)
	splitUpdate := strings.Split(plainUpdate, d)
	if d == delimiter {
		splitUpdate = splitUpdate[:updateLen]
	}
	mapUpdate := make(map[string]string)
	for i := 0; i+1 < len(splitUpdate); i += 2 {
		mapUpdate[splitUpdate[i]] = splitUpdate[i+1]
	}
	// a per-image key may only submit for its own image (anti-forgery)
	if ki := imageForKey(key); ki != "" && mapUpdate["image"] != ki {
		return scoreEntry{}, errors.New("image does not match the per-image key")
	}
	pointValue, err := strconv.Atoi(mapUpdate["score"])
	if err != nil {
		return scoreEntry{}, err
	}
	vulns, err := parseVulns(mapUpdate["vulns"], pointValue, key)
	if err != nil {
		return scoreEntry{}, err
	}

	newEntry := scoreEntry{
		Time:     time.Now().UTC(),
		Team:     getTeam(mapUpdate["team"]),
		Image:    getImage(mapUpdate["image"]),
		Vulns:    vulns,
		Points:   pointValue,
		SourceIP: sourceIP,
	}

	lastRecord, err := getLastScore(&newEntry)
	if err != nil {
		lastRecord = scoreEntry{}
	}

	calcPlayTime(&newEntry, &lastRecord)
	calcElapsedTime(&newEntry, &lastRecord)
	calcCompletionTime(&newEntry, &lastRecord)
	newEntry.PlayTimeStr = formatTime(newEntry.PlayTime)
	newEntry.ElapsedTimeStr = formatTime(newEntry.ElapsedTime)
	replaceScore(&newEntry)
	return newEntry, nil
}

func parseVulns(vulnText string, imagePoints int, key string) (vulnWrapper, error) {
	wrapper := vulnWrapper{}
	vulnText, err := hexDecode(vulnText)
	if err != nil {
		return wrapper, errors.New("Error decoding hex input.")
	}

	plainVulns, err := decryptString(key, vulnText)
	if err != nil {
		return wrapper, err
	}

	splitVulns := strings.Split(plainVulns, updateDelimiter(plainVulns))
	scored, err := strconv.Atoi(splitVulns[0])
	if err != nil {
		return wrapper, err
	}
	wrapper.VulnsScored = scored

	total, err := strconv.Atoi(splitVulns[1])
	if err != nil {
		return wrapper, err
	}
	wrapper.VulnsTotal = total

	pointTotal := 0
	splitVulns = splitVulns[2 : len(splitVulns)-1]
	for _, vuln := range splitVulns {
		splitVuln := strings.Split(vuln, "-")
		if debugEnabled {
			fmt.Println("splitvulns", splitVuln, "len", len(splitVuln))
		}
		if len(splitVuln) < 2 {
			return wrapper, errors.New(fmt.Sprintln("Error splitting vuln on delimiter:", splitVuln, "length of", len(splitVuln)))
		}

		splitText := splitVuln[:len(splitVuln)-1]
		vulnText := ""
		for index, subString := range splitText {
			if index != 0 {
				vulnText += "-"
			}
			vulnText += subString
		}

		splitVuln = strings.Split(strings.TrimSpace(splitVuln[len(splitVuln)-1]), " ")
		if len(splitVuln) != 2 {
			return wrapper, errors.New("Error splitting vuln on space")
		}
		vulnPointsText := strings.TrimSpace(splitVuln[0])
		if string(vulnPointsText[0]) == "N" {
			vulnPointsText = "-" + string(vulnPointsText[1:])
		}
		vulnPoints, err := strconv.Atoi(vulnPointsText)
		if err != nil {
			return wrapper, errors.New("Error parsing vuln point value")
		}
		pointTotal += vulnPoints
		if debugEnabled {
			fmt.Println("appending vuln text:", vulnText, "vuln points:", vulnPoints)
		}
		wrapper.VulnItems = append(wrapper.VulnItems, vulnItem{VulnText: vulnText, VulnPoints: vulnPoints})
	}

	if pointTotal != imagePoints {
		fmt.Println("!!! SOMEONE REVERSED THE CRYTPO !!!")
		fmt.Println("!!! POINTS FOR UPDATE DON'T ADD UP !!")
		fmt.Println("!!! Image points is", imagePoints, "vuln point total is", pointTotal)
		fmt.Println("!!! Wrapper:", wrapper)
		return wrapper, errors.New("Vuln points don't add up")

	}
	return wrapper, nil
}

func parseScoresIntoTeam(scores []scoreEntry) (teamData, error) {
	data, err := parseScoresIntoTeams(scores)
	if err != nil || len(data) <= 0 {
		return teamData{}, err
	}
	return data[0], nil
}

func parseScoresIntoTeams(scores []scoreEntry) ([]teamData, error) {
	td := []teamData{}
	if len(scores) <= 0 {
		return td, nil
	}

	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].Team.ID < scores[j].Team.ID
	})

	imageCount := 0
	totalScore := 0
	playTime, _ := time.ParseDuration("0s")
	currentTeam := scores[0].Team

	for _, score := range scores {
		if currentTeam.ID != score.Team.ID {
			td = append(td, teamData{
				ID:         currentTeam.ID,
				Alias:      displayAlias(currentTeam),
				Email:      currentTeam.Email,
				ImageCount: imageCount,
				Score:      totalScore,
				Time:       formatTime(playTime),
			})
			imageCount = 0
			totalScore = 0
			playTime, _ = time.ParseDuration("0s")
			currentTeam = score.Team
		}
		imageCount++
		totalScore += score.Points
		playTime += score.PlayTime
	}

	td = append(td, teamData{
		ID:         scores[len(scores)-1].Team.ID,
		Alias:      displayAlias(scores[len(scores)-1].Team),
		Email:      scores[len(scores)-1].Team.Email,
		ImageCount: imageCount,
		Score:      totalScore,
		Time:       formatTime(playTime),
	})

	sort.SliceStable(td, func(i, j int) bool {
		var result bool
		if td[i].Score == td[j].Score {
			result = td[i].Time < td[j].Time
		} else {
			result = td[i].Score > td[j].Score
		}
		return result
	})

	return td, nil
}

// publicName is the only name of a team that may be shown to anyone: its alias, or, when it has no
// real alias (empty or equal to the ID), a short label derived from a hash of the ID. A team ID is a
// credential, so it must never appear on a page, in an export, a link we render or an announcement.
func publicName(id, alias string) string {
	if alias != "" && !strings.EqualFold(alias, id) {
		return alias
	}
	if alias != "" && !looksLikeCredential(id) {
		// A team created on the fly from a typed-in name has that name as both ID and alias; it is
		// a name, not a credential, so show it rather than an unexplained "Team-xxxxxx".
		return alias
	}
	sum := sha256.Sum256([]byte(id))
	return "Team-" + hex.EncodeToString(sum[:])[:6]
}

// displayAlias is the name to show for a team on the scoreboard. A score stores the team as it
// was when the score came in; teams auto-created on their first score got their ID as alias, so
// when sarpedon.conf now holds a real alias for that ID, it wins over a stored empty/ID alias.
// It never returns the ID itself (see publicName).
func displayAlias(t teamData) string {
	refreshTeams()
	if t.Alias != "" && !strings.EqualFold(t.Alias, t.ID) {
		return t.Alias
	}
	for _, c := range sarpConfig.Team {
		if c.ID == t.ID && c.Alias != "" && !strings.EqualFold(c.Alias, c.ID) {
			return c.Alias
		}
	}
	return publicName(t.ID, t.Alias)
}

// teamLabel is publicName for a team id found in sarpedon.conf (by id, case-insensitive); an id
// the config doesn't know still gets a hashed label, never itself.
func teamLabel(id string) string {
	refreshTeams()
	for _, t := range sarpConfig.Team {
		if strings.EqualFold(t.ID, id) {
			return publicName(t.ID, t.Alias)
		}
	}
	return publicName(id, "")
}

// withDisplayAlias gives every score's team the name to show for it (see displayAlias). Scores come
// from the database with the team as it was at scoring time, so every place that loads scores calls
// this once; nothing downstream (leaderboard, team pages, practice, sprint) sees a stale ID alias.
func withDisplayAlias(scores []scoreEntry) []scoreEntry {
	for i := range scores {
		scores[i].Team.Alias = displayAlias(scores[i].Team)
	}
	return scores
}

var (
	mintedID = regexp.MustCompile(`^tt-[0-9a-f]{16}$`)
	randomID = regexp.MustCompile(`^[A-Za-z0-9]{10}$`)
	hasDigit = regexp.MustCompile(`[0-9]`)
	hasUpper = regexp.MustCompile(`[A-Z]`)
	hasLower = regexp.MustCompile(`[a-z]`)
)

// looksLikeCredential reports whether id has the shape of a generated team ID (levelsvc's
// tt-<16 hex>, or sarpedon's 10-char mixed-case alphanumeric with a digit) as opposed to a
// human-typed name such as "xinya" or "Team Rocket".
func looksLikeCredential(id string) bool {
	if mintedID.MatchString(id) {
		return true
	}
	return randomID.MatchString(id) && hasDigit.MatchString(id) && hasUpper.MatchString(id) && hasLower.MatchString(id)
}
