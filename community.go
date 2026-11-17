package main

// Community images: CyberPatriot-style score reports that students post as screenshots in the
// Discord #homework-submission channel. The turtlebot Discord bot reads them (vision model),
// judges whether the time is plausible, and serves them as JSON at <communityUrl>/community.json
// plus thumbnails at <communityUrl>/community/img/<sha256>. This page only displays that feed;
// nothing here scores or punishes anyone.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type communityScore struct {
	Score          int      `json:"score"`
	MaxScore       *int     `json:"max_score"`
	FoundVulns     *int     `json:"found_vulns"`
	TotalVulns     *int     `json:"total_vulns"`
	Penalties      *int     `json:"penalties"`
	ElapsedSeconds *int     `json:"elapsed_seconds"`
	Verdict        string   `json:"verdict"`
	Reasons        []string `json:"reasons"`
	Thumb          string   `json:"thumb"`
	Submitted      string   `json:"submitted"`
}

type communityEntry struct {
	User        string         `json:"user"`
	Team        *string        `json:"team"`
	Submissions int            `json:"submissions"`
	Rank        *int           `json:"rank"`
	Best        communityScore `json:"best"`
	Latest      communityScore `json:"latest"`
}

type communityImage struct {
	Image   string           `json:"image"`
	Key     string           `json:"key"`
	Entries []communityEntry `json:"entries"`
}

type communityFeed struct {
	Generated string           `json:"generated"`
	Images    []communityImage `json:"images"`
}

func communityBase() string {
	b := strings.TrimRight(sarpConfig.CommunityUrl, "/")
	if b == "" {
		b = "http://127.0.0.1:8098"
	}
	return b
}

var communityHTTP = &http.Client{Timeout: 8 * time.Second}

func fetchCommunity() (*communityFeed, error) {
	resp, err := communityHTTP.Get(communityBase() + "/community.json")
	if err != nil {
		return nil, errors.New("the community feed is unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("the community feed returned %d", resp.StatusCode)
	}
	var f communityFeed
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&f); err != nil {
		return nil, errors.New("the community feed is not valid JSON")
	}
	return &f, nil
}

// communityDuration formats seconds as h:mm:ss ("" when unknown).
func communityDuration(s *int) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%d:%02d:%02d", *s/3600, *s%3600/60, *s%60)
}

func communityInt(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

var communityHashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func viewCommunity(c *gin.Context) {
	f, err := fetchCommunity()
	data := gin.H{}
	if err != nil {
		data["err"] = err.Error()
	} else {
		data["feed"] = f
	}
	data["dur"] = communityDuration
	data["num"] = communityInt
	c.HTML(http.StatusOK, "community.html", pageData(c, "Community Images", data))
}

// viewCommunityImg proxies a screenshot thumbnail from the bot (which only listens on localhost).
func viewCommunityImg(c *gin.Context) {
	h := c.Param("hash")
	if !communityHashRe.MatchString(h) {
		c.Status(http.StatusNotFound)
		return
	}
	resp, err := communityHTTP.Get(communityBase() + "/community/img/" + h)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		c.Status(http.StatusNotFound)
		return
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if ct != "image/png" && ct != "image/jpeg" && ct != "image/webp" {
		ct = "application/octet-stream"
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.DataFromReader(http.StatusOK, resp.ContentLength, ct, io.LimitReader(resp.Body, 16<<20), nil)
}
