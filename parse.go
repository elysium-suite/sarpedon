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
		Time:   time.Now().UTC(),
		Team:   getTeam(mapUpdate["team"]),
		Image:  getImage(mapUpdate["image"]),
		Vulns:  vulns,
		Points: pointValue,
	}
	// fmt.Println("newenntry", newEntry)
	lastRecord, err := getLastScore(&newEntry)
	if err != nil {
		lastRecord = scoreEntry{}
	}
	calcPlayTime(&newEntry, &lastRecord)
	calcElapsedTime(&newEntry, &lastRecord)
	newEntry.PlayTimeStr = formatTime(newEntry.PlayTime)
	newEntry.ElapsedTimeStr = formatTime(newEntry.ElapsedTime)
	replaceScore(&newEntry)
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
		// fmt.Println("splitvulns", splitVuln, "len", len(splitVuln))
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
		// fmt.Println("BRUH vulnText", vulnText)

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
		// fmt.Println("appending", vulnText, vulnPoints)
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
				Alias:      currentTeam.Alias,
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
		Alias:      scores[len(scores)-1].Team.Alias,
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
