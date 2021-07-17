package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	sarpConfig   = config{}
	sarpShells   = make(map[string]map[string]*imageShell)
	debugEnabled = false
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
		routes.GET("/shell/:id/:image/clientInput", shellClientInput)
		routes.GET("/shell/:id/:image/clientOutput", shellClientOutput)
	}

	authRoutes := routes.Group("/")
	authRoutes.Use(authRequired)
	{
		authRoutes.GET("/logout", logout)
		authRoutes.GET("/settings", viewSettings)
		authRoutes.POST("/settings", changeSettings)
		authRoutes.GET("/export", exportCsv)
		authRoutes.GET("/shell/:id/:image", getShell)
		authRoutes.GET("/shell/:id/:image/serverInput", shellServerInput)
		authRoutes.GET("/shell/:id/:image/serverOutput", shellServerOutput)
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
	image, err := initShell(c)
	if err != nil {
		errorOut(c, err)
		return
	}
	if image.Waiting {
		c.JSON(200, gin.H{"status": "GIMMESHELL"})
		return
	}
	imageName := c.Param("image")
	teamID := c.Param("id")
	// Already checked for being a valid time on startup.
	if sarpConfig.PlayTime != "" {

		playTimeLimit, _ := time.ParseDuration(sarpConfig.PlayTime)
		recentRecord, err := getLastScore(&scoreEntry{
			Image: getImage(imageName),
			Team:  getTeam(teamID),
		})
		if err == nil && recentRecord.PlayTime > playTimeLimit {
			c.JSON(200, gin.H{"status": "DIE"})
			return
		}
	}
	c.JSON(200, gin.H{"status": "OK"})
}

func getShell(c *gin.Context) {
	image, err := initShell(c)
	if err != nil {
		errorOutGraceful(c, err)
		return
	}
	teamID := c.Param("id")
	imageName := c.Param("image")
	if image.Active == true {
		c.HTML(http.StatusOK, "shell.html", pageData(c, "shell", gin.H{"team": getTeam(teamID), "image": getImage(imageName), "error": "Shell is currently in use!"}))
		return
	}
	refreshShell(teamID, imageName, image)
	fmt.Println("*****************")
	fmt.Println("IMAGE ACTIVE FOR", image, "is", image.Active)
	fmt.Println("*****************")
	c.HTML(http.StatusOK, "shell.html", pageData(c, "shell", gin.H{"team": getTeam(teamID), "image": getImage(imageName)}))
}

func viewSettings(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", pageData(c, "settings", nil))
}

func viewAnnounce(c *gin.Context) {
	allAnnoucements, err := getAnnoucements()
	if err != nil {
		allAnnoucements = []announcement{}
		fmt.Println("Error retrieving annoucements", err)
	}
	c.HTML(http.StatusOK, "announce.html", pageData(c, "announcements", gin.H{"announcements": allAnnoucements}))
}

func scoreUpdate(c *gin.Context) {
	c.Request.ParseForm()
	cryptUpdate := c.Request.Form.Get("update")
	newScore, err := parseUpdate(cryptUpdate)
	if err != nil {
		errorOut(c, err)
		return
	}
	// fmt.Println("newscore is", newScore)
	err = insertScore(newScore)
	if err != nil {
		errorOut(c, err)
		return
	}
	c.JSON(200, gin.H{"status": "OK"})
}

func changeSettings(c *gin.Context) {
	c.Request.ParseForm()
	announceTitle := c.Request.Form.Get("title")
	announceBody := c.Request.Form.Get("body")
	loc, _ := time.LoadLocation(sarpConfig.Timezone)
	insertAnnoucement(&announcement{time.Now().In(loc), announceTitle, announceBody})
	c.HTML(http.StatusOK, "settings.html", pageData(c, "settings", nil))
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
