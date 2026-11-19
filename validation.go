package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	updateLen = 10
)

// legacyDelimiter is what aeacus 1.8.x puts between the fields of an update (2.x uses `delimiter`).
const legacyDelimiter = "|-S#-|"

// updateDelimiter picks the delimiter a decrypted update or vuln list was built with: the 2.x one
// when it is there, else the 1.8.x one, so images built with either aeacus score.
func updateDelimiter(plain string) string {
	if !strings.Contains(plain, delimiter) && strings.Contains(plain, legacyDelimiter) {
		return legacyDelimiter
	}
	return delimiter
}

func validateUpdate(plainUpdate string) error {
	d := updateDelimiter(plainUpdate)
	splitUpdate := strings.Split(plainUpdate, d)
	if d == legacyDelimiter {
		return validateLegacyUpdate(splitUpdate)
	}
	if len(splitUpdate) < updateLen {
		return errors.New("Malformed update")
	}
	splitUpdate = splitUpdate[:updateLen]

	for _, item := range splitUpdate {
		fmt.Println("item [", item, "]")
		if !validateString(item) {
			return errors.New("String validation failed for " + item)
		}
	}

	if splitUpdate[0] != "team" || !validateTeamByID(splitUpdate[1]) {
		return errors.New("Invalid team specified")
	}

	if splitUpdate[2] != "image" || !validateImage(splitUpdate[3]) {
		return errors.New("Invalid image specified")
	}

	return nil
}

// validateLegacyUpdate checks an aeacus 1.8.x update: team, image, score, challenge, vulns, time
// as key/value pairs. The challenge value is never used, so it is not checked.
func validateLegacyUpdate(split []string) error {
	fields := legacyFields(split)
	for _, k := range []string{"team", "image", "score", "vulns", "time"} {
		v, ok := fields[k]
		if !ok {
			return errors.New("Malformed update")
		}
		if !validateString(v) {
			return errors.New("String validation failed for " + v)
		}
	}
	if !validateTeamByID(fields["team"]) {
		return errors.New("Invalid team specified")
	}
	if !validateImage(fields["image"]) {
		return errors.New("Invalid image specified")
	}
	return nil
}

// legacyFields reads the key/value pairs of an update split on its delimiter.
func legacyFields(split []string) map[string]string {
	fields := make(map[string]string)
	for i := 0; i+1 < len(split); i += 2 {
		fields[split[i]] = split[i+1]
	}
	return fields
}

func validateReq(c *gin.Context) (string, string, error) {
	teamID := c.Param("id")
	if !validateTeam(teamID) {
		err := errors.New("Invalid team id: " + teamID)
		return "", "", err
	}
	imageName := c.Param("image")
	if !validateImage(imageName) {
		err := errors.New("Invalid image name: " + imageName)
		return "", "", err
	}
	return teamID, imageName, nil
}

func validateString(input string) bool {
	if input == "" {
		return false
	}
	validationString := `^[a-zA-Z0-9-_ ]+$`
	inputValidation := regexp.MustCompile(validationString)
	return inputValidation.MatchString(input)
}

func validateTeam(teamName string) bool {
	for _, team := range sarpConfig.Team {
		if team.ID == teamName {
			return true
		}
		if team.Alias == teamName || publicName(team.ID, team.Alias) == teamName {
			return true
		}
	}

	if sarpConfig.AllowNewTeam == true {
		createNewTeamId(teamName)
		return true
	} else {
		return false
	}
}

func validateTeamId(teamId string) bool {
	for _, team := range sarpConfig.Team {
		if team.ID == teamId {
			return true
		}
	}
	return false
}

func validateTeamByID(teamName string) bool {
	for _, team := range sarpConfig.Team {
		if team.ID == teamName {
			return true
		}
	}
	return false
}

func validateImage(imageName string) bool {
	for _, image := range sarpConfig.Image {
		if image.Name == imageName {
			return true
		}
	}
	return false
}

// createNewTeamId registers a team seen for the first time (only reached when allownewteam is set).
func createNewTeamId(teamId string) teamData {
	// Alias defaults to the ID: an empty alias makes the next start fail validation (config.go).
	team := teamData{ID: teamId, Alias: teamId}
	sarpConfig.Team = append(sarpConfig.Team, team)
	updateConfig()
	return team
}
