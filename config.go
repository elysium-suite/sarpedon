package main

import (
	"io/ioutil"
	"log"
	"time"

	"github.com/BurntSushi/toml"
)

type config struct {
	Event    string
	Password string
	PlayTime string
	Admin    []adminData
	Image    []imageData
	Team     []teamData
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
	if sarpConfig.PlayTime != "" {
		if _, err := time.ParseDuration(sarpConfig.PlayTime); err != nil {
			log.Fatalln("Invalid duration for playtime: " + err.Error())
		}
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

	for _, team := range sarpConfig.Team {
		if team.Id == "" {
			log.Fatalln("Team id is empty:", team)
		}
		if team.Alias == "" {
			log.Fatalln("Team alias is empty:", team)
		}
		matches := 0
		var dupeTeam teamData
		for _, teamDupe := range sarpConfig.Team {
			if team.Id == teamDupe.Id {
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
}
