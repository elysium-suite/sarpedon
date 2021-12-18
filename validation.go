package main

import (
	"errors"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	updateLen = 10
)

func validateUpdate(plainUpdate string) error {
	splitUpdate := strings.Split(plainUpdate, delimiter)
	if len(splitUpdate) < updateLen {
		return errors.New("Malformed update")
	}
	splitUpdate = splitUpdate[:updateLen]

	for _, item := range splitUpdate {
		if !validateString(item) {
			return errors.New("String validation failed for " + item)
		}
	}

	if splitUpdate[0] != "team" || !validateTeam(splitUpdate[1]) {
		return errors.New("Invalid team specified")
	}

	if splitUpdate[2] != "image" || !validateImage(splitUpdate[3]) {
		return errors.New("Invalid image specified")
	}

	return nil
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
	validationString := `^[a-zA-Z0-9-_]+$`
	inputValidation := regexp.MustCompile(validationString)
	return inputValidation.MatchString(input)
}

func validateTeam(teamName string) bool {
	for _, team := range sarpConfig.Team {
		if team.ID == teamName {
			return true
		}
		if team.Alias == teamName {
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
