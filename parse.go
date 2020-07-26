package main

import (
	"fmt"
    "sort"
	"strconv"
	"strings"
	"time"

    "github.com/pkg/errors"
)

func parseUpdate(cryptUpdate string) (scoreEntry, error) {

	if cryptUpdate == "" || !validateString(cryptUpdate) {
		return scoreEntry{}, errors.New("Empty or invalid characters in cryptUpdate.")
	}
	cryptUpdate, err := hexDecode(cryptUpdate)
	if err != nil {
		return scoreEntry{}, errors.New("Error decoding hex input.")
	}
	plainUpdate, err := decryptString(sarpConfig.Password, cryptUpdate)
	if err != nil {
		return scoreEntry{}, err
	}
	if err := validateUpdate(plainUpdate); err != nil {
		return scoreEntry{}, errors.Wrap(err, "Update validation failed")
	}

	mapUpdate := make(map[string]string)
	splitUpdate := strings.Split(plainUpdate, delimiter)[:13]
	for i := 0; i < len(splitUpdate)-2; i += 2 {
		mapUpdate[splitUpdate[i]] = splitUpdate[i+1]
	}
	pointValue, err := strconv.Atoi(mapUpdate["score"])
	if err != nil {
		return scoreEntry{}, err
	}
	vulns, err := parseVulns(mapUpdate["vulns"], pointValue)
	if err != nil {
		return scoreEntry{}, err
	}
	newEntry := scoreEntry{
		Time:   time.Now(),
		Team:   mapUpdate["team"],
		Image:  mapUpdate["image"],
		Vulns:  vulns,
		Points: pointValue,
	}
    fmt.Println("newenntry", newEntry)
	calcPlayTime(&newEntry)
	calcElapsedTime(&newEntry)
	return newEntry, nil
}

func parseVulns(vulnText string, imagePoints int) (vulnWrapper, error) {
	wrapper := vulnWrapper{}
	vulnText, err := hexDecode(vulnText)

	if err != nil {
		return wrapper, errors.New("Error decoding hex input.")
	}

	plainVulns, err := decryptString(sarpConfig.Password, vulnText)
	if err != nil {
		return wrapper, err
	}

	splitVulns := strings.Split(plainVulns, delimiter)
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
		if len(splitVuln) != 2 {
			return wrapper, errors.New("Error splitting vuln on delimiter")
		}
		vulnText := strings.TrimSpace(splitVuln[0])
		splitVuln = strings.Split(strings.TrimSpace(splitVuln[1]), " ")
		if len(splitVuln) != 2 {
			return wrapper, errors.New("Error splitting vuln on space")
		}
		vulnPoints, err := strconv.Atoi(splitVuln[0])
		if err != nil {
			return wrapper, errors.New("Error parsing vuln point value")
		}
		pointTotal += vulnPoints
		fmt.Println("appending", vulnText, vulnPoints)
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

	imageCount := 0
	totalScore := 0
	playTime, _ := time.ParseDuration("0s")
	currentTeam := scores[0].Team

	for _, score := range scores {
		if currentTeam != score.Team {
			td = append(td, teamData{
				Team:       getTeam(currentTeam),
				ImageCount: imageCount,
				Score:      totalScore,
				Time:       formatTime(playTime),
			})
			imageCount = 0
			totalScore = 0
			playTime, _ = time.ParseDuration("0s")
			currentTeam = score.Team
		}
		imageCount += 1
		totalScore += score.Points
		playTime += score.PlayTime
	}

	td = append(td, teamData{
		Team:       getTeam(scores[len(scores)-1].Team),
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
