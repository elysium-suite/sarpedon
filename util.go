package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

func errorOut(c *gin.Context, err error) {
	fmt.Println("ERROR:", err)
	c.JSON(400, gin.H{"error": "Invalid request."})
	c.Abort()
}

func errorOutGraceful(c *gin.Context, err error) {
	fmt.Println("ERROR:", err)
	c.Redirect(http.StatusSeeOther, "/")
	c.Abort()
}

func getTeam(teamProp string) teamData {
	for _, team := range sarpConfig.Team {
		if team.Id == teamProp || team.Email == teamProp || team.Alias == teamProp {
			return team
		}
	}
	return teamData{}
}

func getImage(imageName string) imageData {
	for _, image := range sarpConfig.Image {
		if image.Name == imageName {
			return image
		}
	}
	return imageData{}
}

func formatTime(dur time.Duration) string {
	durSeconds := dur.Microseconds() / 1000000
	fmt.Println("=======")
	fmt.Println("durnum", durSeconds)
	seconds := durSeconds % 60
	fmt.Println("seconds", seconds)
	durSeconds -= seconds
	fmt.Println("durnum", durSeconds)
	minutes := (durSeconds % (60 * 60)) / 60
	fmt.Println("minutes", minutes)
	durSeconds -= minutes * 60
	fmt.Println("durnum", durSeconds)
	hours := durSeconds / (60 * 60)
	fmt.Println("hours", hours)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func calcPlayTime(newEntry *scoreEntry) error {
	threshhold, _ := time.ParseDuration("5m")
	recentRecord, err := getLastScore(newEntry)
	var timeDifference time.Duration
	if err != nil {
		fmt.Println("playtime: no previous record! time is 0")
		timeDifference, _ = time.ParseDuration("0s")
	} else {
		timeDifference = newEntry.Time.Sub(recentRecord.Time)
		fmt.Println("playtime: time diff is", timeDifference)
	}
	if timeDifference < threshhold {
		fmt.Println("Adding timediff for playtime", timeDifference)
		newEntry.PlayTime = recentRecord.PlayTime + timeDifference
	} else {
		newEntry.PlayTime = recentRecord.PlayTime
	}
	return nil
}

func calcElapsedTime(newEntry *scoreEntry) error {
	recentRecord, err := getLastScore(newEntry)
	var timeDifference time.Duration
	if err != nil {
		fmt.Println("elaptime: no previous record! time is 0")
		timeDifference, _ = time.ParseDuration("0s")
	} else {
		timeDifference = newEntry.Time.Sub(recentRecord.Time)
		fmt.Println("elaptime: time diff is", timeDifference)
	}
	fmt.Println("Adding timediff for elaptime", timeDifference)
	newEntry.ElapsedTime = recentRecord.ElapsedTime + timeDifference
	fmt.Println("Elaptime is now", newEntry.ElapsedTime)
	return nil
}

func consolidateRecords(allRecords []scoreEntry, images []imageData) ([]imageData, []string) {
	imageRecords := []time.Time{}

	timeStr := "2006-01-02 15:04"
	if len(allRecords) <= 0 {
		return images, []string{}
	}

	fmt.Println("UHH, LIKE IMAGES ARE TOTALLY", images)
	for i, image := range images {
		fmt.Println("PROCESSING IMAGE", image)
		currentRecord := scoreEntry{}

		for _, record := range allRecords {
			if record.Image.Name == image.Name {
				record.Time = record.Time.Round(time.Minute)
				if currentRecord.Time.IsZero() {
					fmt.Println("======= setting time ======", record.Time)
					currentRecord = record
				}
				fmt.Println("CHECKING RECORD", record.Time)
				if record.Time.Format(timeStr) != currentRecord.Time.Format(timeStr) {
					fmt.Println("ADDING new image record, lol:", currentRecord.Time, "versus new", record.Time)
					images[i].Records = append(images[i].Records, currentRecord)
					imageRecords = append(imageRecords, currentRecord.Time)
				}
				currentRecord = record
			}
		}

		if !currentRecord.Time.IsZero() {
			fmt.Println("ADDING new image record, lol:", currentRecord.Time)
			images[i].Records = append(images[i].Records, currentRecord)
			imageRecords = append(imageRecords, currentRecord.Time)
		}
	}

	sort.SliceStable(imageRecords, func(i, j int) bool {
		return imageRecords[i].Format(timeStr) < imageRecords[j].Format(timeStr)
	})

	if len(imageRecords) <= 0 {
		return images, []string{}
	}

	labels := generateLabels(imageRecords[0], imageRecords[len(imageRecords)-1])
	fmt.Println("final labels", labels)
	return images, labels
}

func generateLabels(firstTime, lastTime time.Time) []string {
	timeStr := "2006-01-02 15:04"
	timeDiff := lastTime.Sub(firstTime).Round(time.Minute)
	labels := []string{}
	fmt.Println("FIRSTITME", firstTime, "LASTTIME", lastTime)
	for t, _ := time.ParseDuration("0s"); t <= timeDiff; t += time.Minute {
		labels = append(labels, firstTime.Add(t).Format(timeStr))
	}
	return labels
}
