package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
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
		if team.ID == teamProp || team.Email == teamProp || team.Alias == teamProp {
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
	if debugEnabled {
		fmt.Println("=======")
		fmt.Println("durnum", durSeconds)
	}
	seconds := durSeconds % 60
	if debugEnabled {
		fmt.Println("seconds", seconds)
	}
	durSeconds -= seconds
	if debugEnabled {
		fmt.Println("durnum", durSeconds)
	}
	minutes := (durSeconds % (60 * 60)) / 60
	if debugEnabled {
		fmt.Println("minutes", minutes)
	}
	durSeconds -= minutes * 60
	if debugEnabled {
		fmt.Println("durnum", durSeconds)
	}
	hours := durSeconds / (60 * 60)
	if debugEnabled {
		fmt.Println("hours", hours)
	}
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func calcPlayTime(newEntry, lastEntry *scoreEntry) error {
	var timeDifference time.Duration
	threshold, _ := time.ParseDuration("5m")
	if lastEntry.Time.IsZero() {
		timeDifference, _ = time.ParseDuration("0s")
	} else {
		timeDifference = newEntry.Time.Sub(lastEntry.Time)
	}
	if timeDifference < threshold {
		newEntry.PlayTime = lastEntry.PlayTime + timeDifference
	} else {
		newEntry.PlayTime = lastEntry.PlayTime
	}
	return nil
}

func calcElapsedTime(newEntry, lastEntry *scoreEntry) error {
	var timeDifference time.Duration
	if lastEntry.Time.IsZero() {
		timeDifference, _ = time.ParseDuration("0s")
	} else {
		timeDifference = newEntry.Time.Sub(lastEntry.Time)
	}
	newEntry.ElapsedTime = lastEntry.ElapsedTime + timeDifference
	return nil
}

func consolidateRecords(allRecords []scoreEntry, images []imageData) ([]imageData, []string) {
	imageRecords := []time.Time{}

	timeStr := "2006-01-02 15:04"
	timeBegin := time.Unix(28800, 0)
	fmt.Println(allRecords)

	if len(allRecords) <= 0 {
		return images, []string{}
	}

	for i, image := range images {
		currentRecord := scoreEntry{}

		for _, record := range allRecords {
			if record.Image.Name == image.Name {
				record.PlayTime = record.PlayTime.Round(time.Minute)

				tempTimeStr := formatTime(record.PlayTime.Round(time.Minute))
				record.PlayTimeStr = tempTimeStr[0 : len(tempTimeStr)-3]
				fmt.Println(record.PlayTimeStr)

				if currentRecord.Time.IsZero() {
					// fmt.Println("======= setting time ======", record.Time)
					currentRecord = record
				}
				// fmt.Println("CHECKING RECORD", record.Time)
				if record.Time.Format(timeStr) != currentRecord.Time.Format(timeStr) {
					// fmt.Println("ADDING new image record, lol:", currentRecord.Time, "versus new", record.Time)
					images[i].Records = append(images[i].Records, currentRecord)
					imageRecords = append(imageRecords, timeBegin.Add(currentRecord.PlayTime))
				}
				currentRecord = record
			}
		}

		if !currentRecord.Time.IsZero() {
			// fmt.Println("ADDING new image record, lol:", currentRecord.Time)
			images[i].Records = append(images[i].Records, currentRecord)
			imageRecords = append(imageRecords, timeBegin.Add(currentRecord.PlayTime))
		}
	}

	sort.SliceStable(imageRecords, func(i, j int) bool {
		return imageRecords[i].Format(timeStr) < imageRecords[j].Format(timeStr)
	})

	if len(imageRecords) <= 0 {
		return images, []string{}
	}

	labels := generateLabels(imageRecords[0], imageRecords[len(imageRecords)-1])
	return images, labels
}

func generateLabels(firstTime, lastTime time.Time) []string {
	timeDiff := lastTime.Sub(firstTime).Round(time.Minute)
	labels := []string{}

	for t, _ := time.ParseDuration("0s"); t <= timeDiff; t += time.Minute {
		timeSince := timeDiff - lastTime.Sub(firstTime.Add(t))

		hours, minutes := "", ""
		if int(timeSince.Hours()) < 10 {
			hours = "0" + strconv.Itoa(int(timeSince.Hours()))
		} else {
			hours = strconv.Itoa(int(timeSince.Hours()))
		}

		if int(timeSince.Minutes())%60 < 10 {
			minutes = "0" + strconv.Itoa(int(timeSince.Minutes())%60)
		} else {
			minutes = strconv.Itoa(int(timeSince.Minutes()) % 60)
		}

		labels = append(labels, hours+":"+minutes)
	}
	return labels
}
