package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func validateUpdate(plainUpdate string) error {
	splitUpdate := strings.Split(plainUpdate, delimiter)[:12]
	fmt.Println("splitUpdate", splitUpdate, "len", len(splitUpdate))

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
		if team.Id == teamName {
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
