package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	imageStatus = struct {
		sync.RWMutex
		m map[string]map[string]gin.H
	}{m: make(map[string]map[string]gin.H)}
	sarpConfig          = config{}
	debugEnabled        = false
	acceptingScores     = true
	alternateCompletion = false
)

func init() {
	flag.BoolVar(&debugEnabled, "d", false, "Print verbose debug information")
	flag.Parse()
}

func main() {
	readConfig(&sarpConfig)
	checkConfig()

	// Initialize Gin router
	// gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Add Increment Function to Router
	r.SetFuncMap(template.FuncMap{
		"increment": func(num int) int {
			return num + 1
		},
	})

	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./assets")
	initCookies(r)

	// Routes
	routes := r.Group("/")
	{
		routes.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", pageData(c, "login", nil))
		})
		routes.GET("/", viewScoreboard)
		routes.GET("/announcements", viewAnnounce)
		routes.GET("/status/:id/:image", getStatus)
		routes.POST("/login", login)
		routes.POST("/update", scoreUpdate)
		routes.GET("/team/:team", viewTeam)
		routes.GET("/image/:image", viewImage)
		routes.GET("/team/:team/image/:image", viewTeamImage)
	}

	authRoutes := routes.Group("/")
	authRoutes.Use(authRequired)
	{
		authRoutes.GET("/logout", logout)
		authRoutes.GET("/settings", viewSettings)
		authRoutes.POST("/settings", changeSettings)
		authRoutes.GET("/export", exportCsv)
	}

	fmt.Println("Initializing scoreboard data...")
	initScoreboard()

	r.Run(":4013")
}

func viewScoreboard(c *gin.Context) {
	teamScores, err := getTop()
	if err != nil {
		panic(err)
	}
	teamData, err := parseScoresIntoTeams(teamScores)
	if err != nil {
		panic(err)
	}
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard", gin.H{"scores": teamData}))
}

func viewImage(c *gin.Context) {
	imageName := c.Param("image")
	if !validateString(imageName) {
		errorOut(c, errors.New("Invalid image name: "+imageName))
	}
	teamScores, err := getTop()
	if err != nil {
		panic(err)
	}
	filteredScores := []scoreEntry{}
	for _, score := range teamScores {
		if score.Image.Name == imageName {
			filteredScores = append(filteredScores, score)
		}
	}
	teamData, err := parseScoresIntoTeams(filteredScores)
	if err != nil {
		panic(err)
	}
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard for "+imageName, gin.H{"scores": teamData, "imageFilter": getImage(imageName), "event": sarpConfig.Event}))
}

func viewTeam(c *gin.Context) {
	teamName := c.Param("team")
	if !validateString(teamName) || !validateTeam(teamName) {
		errorOutGraceful(c, errors.New("Invalid team name: "+teamName))
		return
	}
	teamScore := getScore(teamName, "")
	if len(teamScore) <= 0 {
		errorOutGraceful(c, errors.New("Team doesn't have any image data"))
		return
	}
	for index, score := range teamScore {
		for _, vuln := range score.Vulns.VulnItems {
			if vuln.VulnPoints < 0 {
				teamScore[index].Penalties++
			}
		}
	}
	teamData, err := parseScoresIntoTeam(teamScore)
	if err != nil {
		errorOutGraceful(c, errors.New("Parsing team scores failed"))
		return
	}
	allRecords := getAll(teamName, "")
	imageCopies := []imageData{}
	for _, image := range sarpConfig.Image {
		imageCopies = append(imageCopies, image)
	}
	images, labels := consolidateRecords(allRecords, imageCopies)
	for index := range images {
		recordIndex := c.Request.URL.Query().Get("record" + strconv.Itoa(index))
		if recordIndex != "" {
			images[index].Index, err = strconv.Atoi(recordIndex)
			if err != nil {
				errorOutGraceful(c, errors.New("Invalid record number given"))
				return
			}
		} else {
			images[index].Index = len(images[index].Records) - 1
		}
	}

	loc, _ := time.LoadLocation(sarpConfig.Timezone)
	for index, score := range teamScore {
		teamScore[index].Time = score.Time.In(loc)
		if teamScore[index].CompletionTime != (time.Time{}) {
			teamScore[index].CompletionTime = score.CompletionTime.In(loc)
		}
	}
	for index := range images {
		for index2, record := range images[index].Records {
			images[index].Records[index2].Time = record.Time.In(loc)
		}
	}

	c.HTML(http.StatusOK, "detail.html", pageData(c, "Scoreboard for "+teamName, gin.H{"data": teamScore, "team": teamData, "labels": labels, "images": images}))
}

func exportCsv(c *gin.Context) {
	c.Data(200, "text/csv", []byte(getCsv()))
}

func viewTeamImage(c *gin.Context) {
	teamName := c.Param("team")
	if !validateString(teamName) || !validateTeam(teamName) {
		errorOutGraceful(c, errors.New("Invalid team name"))
		return
	}
	imageName := c.Param("image")
	if !validateString(imageName) || !validateImage(imageName) {
		errorOutGraceful(c, errors.New("Invalid image name"))
		return
	}
	teamScore := getScore(teamName, imageName)
	if len(teamScore) <= 0 {
		errorOutGraceful(c, errors.New("Team doesn't have any image data"))
		return
	}
	for index, score := range teamScore {
		for _, vuln := range score.Vulns.VulnItems {
			if vuln.VulnPoints < 0 {
				teamScore[index].Penalties++
			}
		}
	}
	teamData, err := parseScoresIntoTeam(teamScore)
	if err != nil {
		errorOutGraceful(c, errors.New("Parsing team scores failed"))
		return
	}
	allRecords := getAll(teamName, "")
	images, labels := consolidateRecords(allRecords, []imageData{getImage(imageName)})
	for index := range images {
		recordIndex := c.Request.URL.Query().Get("record" + strconv.Itoa(index))
		if recordIndex != "" {
			images[index].Index, err = strconv.Atoi(recordIndex)
			if err != nil {
				errorOutGraceful(c, errors.New("Invalid record number given"))
				return
			}
		} else {
			images[index].Index = len(images[index].Records) - 1
		}
	}

	c.HTML(http.StatusOK, "detail.html", pageData(c, "Scoreboard for "+teamName, gin.H{"data": teamScore, "team": teamData, "labels": labels, "images": images, "imageFilter": getImage(imageName)}))
}

func getStatus(c *gin.Context) {
	id, image, err := validateReq(c)
	if err != nil {
		errorOut(c, err)
		return
	}

	if sarpConfig.PlayTime != "" {
		playTimeLimit, _ := time.ParseDuration(sarpConfig.PlayTime)
		recentRecord, err := getLastScore(&scoreEntry{
			Team:  getTeam(id),
			Image: getImage(image),
		})
		// Kill them if they're over play time limit
		if err == nil && recentRecord.PlayTime > playTimeLimit && sarpConfig.Enforce {
			c.JSON(200, gin.H{"status": "DIE"})
			return
		}
	}

	if !acceptingScores {
		c.JSON(400, gin.H{"status": "DISABLED"})
		return
	}

	imageStatus.Lock()
	defer imageStatus.Unlock()
	if v, ok := imageStatus.m[id][image]; ok {
		// Delete existing key
		delete(imageStatus.m[id], image)
		c.JSON(200, v)
		return
	}
	c.JSON(200, gin.H{"status": "OK"})
}

func viewSettings(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", pageData(c, "settings", gin.H{"scoring": acceptingScores}))
}

func viewAnnounce(c *gin.Context) {
	allAnnouncements, err := getAnnouncements()
	if err != nil {
		allAnnouncements = []announcement{}
		fmt.Println("Error retrieving announcements", err)
	}
	c.HTML(http.StatusOK, "announce.html", pageData(c, "announcements", gin.H{"announcements": allAnnouncements}))
}

func scoreUpdate(c *gin.Context) {
	if !acceptingScores {
		c.JSON(400, gin.H{"status": "DISABLED"})
		return
	}

	c.Request.ParseForm()
	cryptUpdate := c.Request.Form.Get("update")
	newScore, err := parseUpdate(cryptUpdate)
	if err != nil {
		errorOut(c, err)
		fmt.Println("Error decrypting update-- maybe your password is wrong?")
		return
	}

	err = insertScore(newScore)
	if err != nil {
		errorOut(c, err)
		return
	}

	c.JSON(200, gin.H{"status": "OK"})
}

func changeSettings(c *gin.Context) {
	c.Request.ParseForm()
	settingType := c.Request.Form.Get("settingType")

	var err error
	var msg string
	if settingType == "announcement" {
		announceTitle := c.Request.Form.Get("title")
		announceBody := c.Request.Form.Get("body")
		loc, _ := time.LoadLocation(sarpConfig.Timezone)
		postToDiscord("**" + announceTitle + "**\n" + announceBody)
		insertAnnouncement(&announcement{time.Now().In(loc), announceTitle, announceBody})
		msg = "Successfully announced!"
	} else if settingType == "toggleScoring" {
		acceptingScores = !acceptingScores

	} else if settingType == "wipeDatabase" {
		err = wipeDatabase()
		if err != nil {
			fmt.Println("Error wiping database", err)
		}
		msg = "Successfully wiped database!"

	} else if settingType == "disableTestingID" {
		err = clearTeamScore("testing_id")
		if err != nil {
			fmt.Println("Error clearing testing_id results", err)
		}
		msg = "Cleared data for testing_id."
	}

	c.HTML(http.StatusOK, "settings.html", pageData(c, "settings", gin.H{"scoring": acceptingScores, "msg": msg, "err": err}))
}

func pageData(c *gin.Context, title string, ginMap gin.H) gin.H {
	newGinMap := gin.H{}
	newGinMap["title"] = title
	newGinMap["user"] = getUser(c)
	newGinMap["event"] = sarpConfig.Event
	newGinMap["config"] = sarpConfig
	for key, value := range ginMap {
		newGinMap[key] = value
	}
	return newGinMap
}
