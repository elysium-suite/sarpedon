package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dbName      = "sarpedon"
	dbURI       = "mongodb://localhost:27017"
	mongoClient *mongo.Client
	mongoCtx    context.Context
	timeConn    time.Time
)

type scoreEntry struct {
	Time           time.Time     `json:"time,omitempty"`
	Team           teamData      `json:"team,omitempty"`
	Image          imageData     `json:"image,omitempty"`
	Vulns          vulnWrapper   `json:"vulns,omitempty"`
	Points         int           `json:"points,omitempty"`
	Penalties      int           `json:"penalties,omitempty"`
	PlayTime       time.Duration `json:"playtime,omitempty"`
	PlayTimeStr    string        `json:"playtimestr,omitempty"`
	ElapsedTime    time.Duration `json:"elapsedtime,omitempty"`
	ElapsedTimeStr string        `json:"elapsedtimestr,omitempty"`
	CompletionTime time.Time     `json:"completiontime,omitempty"`
}

type vulnWrapper struct {
	VulnsScored int        `json:"vulnsscored,omitempty"`
	VulnsTotal  int        `json:"vulnstotal,omitempty"`
	VulnItems   []vulnItem `json:"vulnitems,omitempty"`
}

type vulnItem struct {
	VulnText   string `json:"vulntext,omitempty"`
	VulnPoints int    `json:"vulnpoints,omitempty"`
}

type adminData struct {
	Username, Password string
}

type imageData struct {
	Name, Color string
	Records     []scoreEntry
	Index       int
}

type imageShell struct {
	Waiting     bool
	Active      bool
	StdinRead   *io.PipeReader
	StdinWrite  *io.PipeWriter
	StdoutRead  *io.PipeReader
	StdoutWrite *io.PipeWriter
}

type teamData struct {
	ID, Alias, Email  string
	ImageCount, Score int
	Time              string
}

type announcement struct {
	Time  time.Time
	Title string
	Body  string
}

type completion struct {
	ImageName string
	TeamID    string
	Alias     string
}

func initDatabase() {
	refresh := false

	if timeConn.IsZero() {
		refresh = true
	} else {
		err := mongoClient.Ping(context.TODO(), nil)
		if err != nil {
			refresh = true
			mongoClient.Disconnect(mongoCtx)
		}
	}
	timeConn = time.Now()

	if refresh {
		fmt.Println("Refreshing mongodb connection...")
		client, err := mongo.NewClient(options.Client().ApplyURI(dbURI))
		if err != nil {
			log.Fatal(err)
		} else {
			mongoClient = client
		}
		ctx := context.TODO()
		err = client.Connect(ctx)
		if err != nil {
			log.Fatal(err)
		} else {
			mongoCtx = ctx
		}
	}
}

func getAll(teamName, imageName string) []scoreEntry {
	scores := []scoreEntry{}
	coll := mongoClient.Database(dbName).Collection("results")
	teamObj := getTeam(teamName)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"time", 1}})
	mod := mongo.IndexModel{
		Keys: bson.M{
			"time": -1,
		}, Options: nil,
	}
	_, err := coll.Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		panic(err)
	}

	var cursor *mongo.Cursor

	if imageName != "" {
		cursor, err = coll.Find(context.TODO(), bson.D{{"_id", 1}, {"team.id", teamObj.ID}, {"image", getImage(imageName)}}, findOptions)
		if err != nil {
			panic(err)
		}
	} else {
		cursor, err = coll.Find(context.TODO(), bson.D{{"team.id", teamObj.ID}}, findOptions)
		if err != nil {
			panic(err)
		}
	}

	if err := cursor.All(mongoCtx, &scores); err != nil {
		panic(err)
	}
	return scores
}

func initScoreboard() {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("scoreboard")
	err := coll.Drop(mongoCtx)
	if err != nil {
		fmt.Println("error dropping scoreboard:", err)
		os.Exit(1)
	}
	topBoard, err := getScores()
	if err != nil {
		fmt.Println("error fetching scores:", err)
		os.Exit(1)
	}
	if len(topBoard) > 0 {
		topBoardInterface := []interface{}{}
		for _, item := range topBoard {
			topBoardInterface = append(topBoardInterface, item)
		}
		_, err = coll.InsertMany(context.TODO(), topBoardInterface, nil)
		if err != nil {
			fmt.Println("error inserting scores:", err)
			os.Exit(1)
		}
	}
}

func getScores() ([]scoreEntry, error) {
	initDatabase()
	scores := []scoreEntry{}
	coll := mongoClient.Database(dbName).Collection("results")

	groupStage := bson.D{
		{"$group", bson.D{
			{"_id", bson.D{
				{"image", "$image.name"},
				{"team", "$team.id"},
			}},
			{"time", bson.D{
				{"$max", "$time"},
			}},
			{"team", bson.D{
				{"$last", "$team"},
			}},
			{"image", bson.D{
				{"$last", "$image"},
			}},
			{"points", bson.D{
				{"$last", "$points"},
			}},
			{"playtime", bson.D{
				{"$last", "$playtime"},
			}},
			{"elapsedtime", bson.D{
				{"$last", "$elapsedtime"},
			}},
			{"playtimestr", bson.D{
				{"$last", "$playtimestr"},
			}},
			{"elapsedtimestr", bson.D{
				{"$last", "$elapsedtimestr"},
			}},
			{"completiontime", bson.D{
				{"$last", "$completiontime"},
			}},
			{"vulns", bson.D{
				{"$last", "$vulns"},
			}},
		}},
	}

	projectStage := bson.D{
		{"$project", bson.D{
			{"time", "$time"},
			{"team", "$team"},
			{"image", "$image"},
			{"points", "$points"},
			{"playtime", "$playtime"},
			{"elapsedtime", "$elapsedtime"},
			{"playtimestr", "$playtimestr"},
			{"elapsedtimestr", "$elapsedtimestr"},
			{"completiontime", "$completiontime"},
			{"vulns", "$vulns"},
		}},
	}

	opts := options.Aggregate()

	cursor, err := coll.Aggregate(context.TODO(), mongo.Pipeline{groupStage, projectStage}, opts)
	if err != nil {
		return scores, err
	}

	if err = cursor.All(context.TODO(), &scores); err != nil {
		return scores, err
	}

	return scores, nil
}

func getTop() ([]scoreEntry, error) {
	initDatabase()
	scores := []scoreEntry{}
	coll := mongoClient.Database(dbName).Collection("scoreboard")

	opts := options.Find()
	cursor, err := coll.Find(context.TODO(), bson.D{}, opts)
	if err != nil {
		return scores, err
	}

	if err = cursor.All(context.TODO(), &scores); err != nil {
		return scores, err
	}

	return scores, nil
}

func getCsv() string {
	teamScores, err := getTop()
	if err != nil {
		panic(err)
	}
	csvString := "Email,Alias,Team ID,Image,Score,Play Time,Elapsed Time\n"
	for _, score := range teamScores {
		csvString += score.Team.Email + ","
		csvString += score.Team.Alias + ","
		csvString += score.Team.ID + ","
		csvString += score.Image.Name + ","
		csvString += fmt.Sprintf("%d,", score.Points)
		csvString += formatTime(score.PlayTime) + ","
		csvString += formatTime(score.ElapsedTime) + "\n"
	}
	return csvString
}

func getScore(teamName, imageName string) []scoreEntry {
	scoreResults := []scoreEntry{}
	teamObj := getTeam(teamName)
	teamScores, err := getTop()
	if err != nil {
		panic(err)
	}
	if imageName != "" {
		for _, score := range teamScores {
			if score.Image.Name == imageName && score.Team.ID == teamObj.ID {
				scoreResults = append(scoreResults, score)
			}
		}
	} else {
		for _, image := range sarpConfig.Image {
			for _, score := range teamScores {
				if score.Image.Name == image.Name && score.Team.ID == teamObj.ID {
					scoreResults = append(scoreResults, score)
				}
			}
		}
	}

	return scoreResults
}

func insertScore(newEntry scoreEntry) error {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("results")
	_, err := coll.InsertOne(context.TODO(), newEntry)
	if err != nil {
		return err
	}
	return nil
}

func replaceScore(newEntry *scoreEntry) error {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("scoreboard")
	_, err := coll.DeleteOne(context.TODO(), bson.D{{"image.name", newEntry.Image.Name}, {"team.id", newEntry.Team.ID}})
	if err != nil {
		return err
	}
	_, err = coll.InsertOne(context.TODO(), newEntry)
	if err != nil {
		return err
	}
	return nil
}

func getLastScore(newEntry *scoreEntry) (scoreEntry, error) {
	initDatabase()
	score := scoreEntry{}
	coll := mongoClient.Database(dbName).Collection("scoreboard")
	err := coll.FindOne(context.TODO(), bson.D{{"image.name", newEntry.Image.Name}, {"team.id", newEntry.Team.ID}}).Decode(&score)
	if err != nil {
		fmt.Println("error finding last score:", err)
	}
	return score, err
}

func insertCompletion(completionRecord *completion) error {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("completion")
	_, err := coll.InsertOne(context.TODO(), completionRecord)
	if err != nil {
		return err
	}
	return nil
}

func getCompletion(imageName string) (bool, error) {
	initDatabase()
	var result bson.M

	coll := mongoClient.Database(dbName).Collection("completion")
	err := coll.FindOne(context.TODO(), bson.D{{"imagename", imageName}}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return true, nil
		} else {
			fmt.Println("Error finding finding image completion:", err)
		}
	}

	return false, err
}

func insertAnnouncement(newAnnouncement *announcement) error {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("announcements")
	_, err := coll.InsertOne(context.TODO(), newAnnouncement)
	if err != nil {
		return err
	}
	return nil
}

func getAnnouncements() ([]announcement, error) {
	initDatabase()
	var result []announcement

	coll := mongoClient.Database(dbName).Collection("announcements")
	cur, err := coll.Find(context.TODO(), bson.D{})
	if err != nil {
		return nil, err
	}

	for cur.Next(context.TODO()) {
		var element announcement
		err2 := cur.Decode(&element)
		if err2 != nil {
			return nil, err
		}

		loc, _ := time.LoadLocation(sarpConfig.Timezone)
		element.Time = element.Time.In(loc)
		result = append(result, element)
	}

	return result, err
}
