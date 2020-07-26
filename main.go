package main

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var sarpConfig = Config{}

func main() {

	// load in config
	readConfig(&sarpConfig)
	checkConfig()

	// Initialize Gin router
	//gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/assets", "./assets")
	initCookies(r)

	// Routes
	routes := r.Group("/")
	{
		routes.GET("/login", func(c *gin.Context) {
			c.HTML(http.StatusOK, "login.html", pageData(c, "login", nil))
		})
		routes.POST("/login", login)
		routes.GET("/logout", logout)
		routes.GET("/", viewScoreboard)
		routes.GET("/team/:team", viewTeam)
		routes.GET("/image/:image", viewImage)
		routes.GET("/team/:team/image/:image", viewTeamImage)
		routes.GET("/status", getStatus)
		routes.POST("/update", scoreUpdate)
		routes.GET("/about", viewAbout)
	}

	authRoutes := routes.Group("/")
	authRoutes.Use(authRequired)
	{
		authRoutes.GET("/settings", viewSettings)
		authRoutes.GET("/export", exportCsv)
		authRoutes.POST("/settings", changeSettings)
	}

	r.Run(":4013")

}

///////////////////
// GET Endpoints //
///////////////////

func viewScoreboard(c *gin.Context) {
	teamScores, err := getScores()
	fmt.Println("teamscoreS", teamScores)
	if err != nil {
		panic(err)
	}
	teamData, err := parseScoresIntoTeams(teamScores)
	if err != nil {
		panic(err)
	}
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard", gin.H{"scores": teamData, "event": sarpConfig.Event}))
}

func viewImage(c *gin.Context) {
	imageName := c.Param("image")
	if !validateString(imageName) {
		errorOut(c, errors.New("Invalid team name"))
	}
	teamScores, err := getScores()
	if err != nil {
		panic(err)
	}
	filteredScores := []scoreEntry{}
	for _, score := range teamScores {
		if score.Image == imageName {
			filteredScores = append(filteredScores, score)
		}
	}
	teamData, err := parseScoresIntoTeams(filteredScores)
	if err != nil {
		panic(err)
	}
	c.HTML(http.StatusOK, "index.html", pageData(c, "Scoreboard for "+imageName, gin.H{"scores": teamData, "imageFilter": imageName, "event": sarpConfig.Event}))
}

func viewTeam(c *gin.Context) {
	teamName := c.Param("team")
	if !validateString(teamName) || !validateTeam(teamName) {
		errorOutGraceful(c, errors.New("Invalid team name"))
		return
	}
	teamScore := getScore(teamName, "")
	fmt.Println("teamscore", teamScore)
	if len(teamScore) <= 0 {
		errorOutGraceful(c, errors.New("Team doesn't have any image data"))
		return
	}
	teamData, err := parseScoresIntoTeam(teamScore)
	if err != nil {
		errorOutGraceful(c, errors.New("Parsing team scores failed"))
		return
	}
	allRecords := getAll(teamName, "")
	c.HTML(http.StatusOK, "detail.html", pageData(c, "Scoreboard for "+teamName, gin.H{"data": teamScore, "team": teamData, "records": allRecords}))
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
	teamData, err := parseScoresIntoTeam(teamScore)
	if err != nil {
		errorOut(c, errors.New("Parsing team scores failed"))
		return
	}
	c.HTML(http.StatusOK, "detail.html", pageData(c, "Scoreboard for "+teamName+" "+imageName, gin.H{"data": teamScore, "team": teamData, "imageFilter": imageName}))
}

func getStatus(c *gin.Context) {
	c.HTML(http.StatusOK, "about.html", pageData(c, "about", nil))
}

func viewAbout(c *gin.Context) {
	c.HTML(http.StatusOK, "about.html", pageData(c, "about", nil))
}

func viewSettings(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", pageData(c, "settings", nil))
}

////////////////////
// POST Endpoints //
////////////////////

func scoreUpdate(c *gin.Context) {
	c.Request.ParseForm()
	cryptUpdate := c.Request.Form.Get("update")
	newScore, err := parseUpdate(cryptUpdate)
	if err != nil {
		errorOut(c, err)
		return
	}
	err = insertScore(newScore)
	if err != nil {
		errorOut(c, err)
		return
	}
	c.HTML(http.StatusOK, "index.html", pageData(c, "lists", nil))
}

func changeSettings(c *gin.Context) {
	c.Request.ParseForm()
	c.HTML(http.StatusOK, "index.html", pageData(c, "lists", nil))
}

//////////////////////
// Helper Functions //
//////////////////////

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
	fmt.Println("plainupdate", plainUpdate)
	mapUpdate, err := validateUpdate(plainUpdate)
	if err != nil {
		return scoreEntry{}, err
	}
	pointValue, err := strconv.Atoi(mapUpdate["score"])
	if err != nil {
		return scoreEntry{}, err
	}
	newEntry := scoreEntry{
		Time:   time.Now(),
		Team:   mapUpdate["team"],
		Image:  mapUpdate["image"],
		Vulns:  parseVulns(mapUpdate["vulns"]),
		Points: pointValue,
	}
	calcPlayTime(&newEntry)
	calcElapsedTime(&newEntry)
	return newEntry, nil
}

func validateString(input string) bool {
	if input == "" {
		return false
	}
	validationString := `^[a-zA-Z0-9-_]+$`
	inputValidation := regexp.MustCompile(validationString)
	return inputValidation.MatchString(input)
}

func pageData(c *gin.Context, title string, ginMap gin.H) gin.H {
	newGinMap := gin.H{}
	newGinMap["title"] = title
	newGinMap["user"] = getUser(c)
	fmt.Println("user is", newGinMap["user"])
	newGinMap["config"] = sarpConfig
	for key, value := range ginMap {
		newGinMap[key] = value
	}
	return newGinMap
}
