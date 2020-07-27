package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

var sarpConfig = Config{}

func main() {

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
		routes.GET("/", viewScoreboard)
		routes.POST("/login", login)
		routes.GET("/status", getStatus)
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

	r.Run(":4013")

}

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
	imageCopies := []ImageData{}
	for _, image := range sarpConfig.Image {
		imageCopies = append(imageCopies, image)
	}
	images, labels := consolidateRecords(allRecords, imageCopies)
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
	teamData, err := parseScoresIntoTeam(teamScore)
	if err != nil {
		errorOut(c, errors.New("Parsing team scores failed"))
		return
	}
	images, labels := consolidateRecords(getAll(teamName, imageName), []ImageData{getImage(imageName)})
	c.HTML(http.StatusOK, "detail.html", pageData(c, "Scoreboard for "+teamName+" "+imageName, gin.H{"data": teamScore, "team": teamData, "imageFilter": getImage(imageName), "labels": labels, "images": images}))
}

func getStatus(c *gin.Context) {
	c.JSON(200, gin.H{"status": "OK"})
}

func viewSettings(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", pageData(c, "settings", nil))
}

func scoreUpdate(c *gin.Context) {
	c.Request.ParseForm()
	cryptUpdate := c.Request.Form.Get("update")
	newScore, err := parseUpdate(cryptUpdate)
	if err != nil {
		errorOut(c, err)
		return
	}
	fmt.Println("newscore is", newScore)
	err = insertScore(newScore)
	if err != nil {
		errorOut(c, err)
		return
	}
	c.JSON(200, gin.H{"status": "OK"})
}

func changeSettings(c *gin.Context) {
	c.Request.ParseForm()
	c.HTML(http.StatusOK, "index.html", pageData(c, "lists", nil))
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
