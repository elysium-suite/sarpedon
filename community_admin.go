package main

// Community images admin + consolidated view (projman #412).
//   - /community            one table: a row per image, a column per player, best score only
//   - /community/detail     one player's submissions on one image
//   - /community/lines      the per-image line-item tables (admin can delete a line)
//   - /manage/community     admin: link Discord user <-> scoreboard alias, remove non-image pictures
// Admin choices live in Sarpedon's Mongo (the bot's feed is never edited): links in
// community_links, removed pictures / deleted lines in community_hidden.

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type communityHidden struct {
	Pics  map[string]bool // thumbnail hash -> removed
	Lines map[string]bool // lineKey -> deleted
}

func lineKey(imageKey, user string) string { return imageKey + "\x00" + user }

func thumbHash(thumb string) string { return strings.TrimPrefix(thumb, "/community/img/") }

func (h communityHidden) picHidden(s communityScore) bool {
	return s.Thumb != "" && h.Pics[thumbHash(s.Thumb)]
}

// applyHidden drops deleted lines and removed pictures. An entry whose best picture was
// removed falls back to its latest picture; if that is removed too the entry is dropped.
func applyHidden(f *communityFeed, h communityHidden) *communityFeed {
	out := &communityFeed{Generated: f.Generated}
	for _, img := range f.Images {
		ni := communityImage{Image: img.Image, Key: img.Key}
		for _, e := range img.Entries {
			if h.Lines[lineKey(img.Key, e.User)] {
				continue
			}
			if h.picHidden(e.Best) {
				if h.picHidden(e.Latest) {
					continue
				}
				e.Best = e.Latest
			}
			if h.picHidden(e.Latest) {
				e.Latest = e.Best
			}
			ni.Entries = append(ni.Entries, e)
		}
		if len(ni.Entries) > 0 {
			out.Images = append(out.Images, ni)
		}
	}
	return out
}

type communityMatrix struct {
	Users []string
	Rows  []communityMatrixRow
}

type communityMatrixRow struct {
	Image string
	Key   string
	Cells []*communityEntry // aligned with Users; nil = no submission
}

// buildCommunityMatrix: a row per image, a column per user (sorted), each cell the entry
// (whose Best is the score shown).
func buildCommunityMatrix(f *communityFeed) communityMatrix {
	seen := map[string]bool{}
	m := communityMatrix{}
	for _, img := range f.Images {
		for _, e := range img.Entries {
			if !seen[e.User] {
				seen[e.User] = true
				m.Users = append(m.Users, e.User)
			}
		}
	}
	sort.Slice(m.Users, func(i, j int) bool { return strings.ToLower(m.Users[i]) < strings.ToLower(m.Users[j]) })
	idx := map[string]int{}
	for i, u := range m.Users {
		idx[u] = i
	}
	for _, img := range f.Images {
		row := communityMatrixRow{Image: img.Image, Key: img.Key, Cells: make([]*communityEntry, len(m.Users))}
		for i := range img.Entries {
			e := img.Entries[i]
			if cur := row.Cells[idx[e.User]]; cur == nil || e.Best.Score > cur.Best.Score {
				row.Cells[idx[e.User]] = &e
			}
		}
		m.Rows = append(m.Rows, row)
	}
	return m
}

// ---- Mongo ----

func loadCommunityHidden() communityHidden {
	h := communityHidden{Pics: map[string]bool{}, Lines: map[string]bool{}}
	initDatabase()
	cur, err := mongoClient.Database(dbName).Collection("community_hidden").Find(context.TODO(), bson.D{})
	if err != nil {
		return h
	}
	var docs []struct {
		Kind  string `bson:"kind"`
		Value string `bson:"value"`
	}
	if cur.All(context.TODO(), &docs) != nil {
		return h
	}
	for _, d := range docs {
		if d.Kind == "pic" {
			h.Pics[d.Value] = true
		} else if d.Kind == "line" {
			h.Lines[d.Value] = true
		}
	}
	return h
}

func addCommunityHidden(kind, value string) error {
	initDatabase()
	_, err := mongoClient.Database(dbName).Collection("community_hidden").UpdateOne(context.TODO(),
		bson.D{{"kind", kind}, {"value", value}}, bson.D{{"$set", bson.D{{"kind", kind}, {"value", value}}}},
		options.Update().SetUpsert(true))
	return err
}

type communityLink struct {
	Discord string `bson:"discord"`
	Team    string `bson:"team"`
}

func loadCommunityLinks() []communityLink {
	initDatabase()
	links := []communityLink{}
	cur, err := mongoClient.Database(dbName).Collection("community_links").Find(context.TODO(), bson.D{})
	if err == nil {
		cur.All(context.TODO(), &links)
	}
	sort.Slice(links, func(i, j int) bool { return strings.ToLower(links[i].Discord) < strings.ToLower(links[j].Discord) })
	return links
}

func linkMap(links []communityLink) map[string]string {
	m := map[string]string{}
	for _, l := range links {
		m[l.Discord] = l.Team
		if a := aliasFor(l.Team); a != "" { // links saved earlier as an id show as the alias
			m[l.Discord] = teamLabel(l.Team)
		}
	}
	return m
}

func setCommunityLink(discord, team string) error {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("community_links")
	if team == "" {
		_, err := coll.DeleteOne(context.TODO(), bson.D{{"discord", discord}})
		return err
	}
	_, err := coll.UpdateOne(context.TODO(), bson.D{{"discord", discord}},
		bson.D{{"$set", bson.D{{"discord", discord}, {"team", team}}}}, options.Update().SetUpsert(true))
	return err
}

// ---- short image names ----

// loadCommunityNames returns admin-chosen short names keyed by image key.
func loadCommunityNames() map[string]string {
	m := map[string]string{}
	initDatabase()
	cur, err := mongoClient.Database(dbName).Collection("community_names").Find(context.TODO(), bson.D{})
	if err != nil {
		return m
	}
	var docs []struct {
		Key  string `bson:"key"`
		Name string `bson:"name"`
	}
	if cur.All(context.TODO(), &docs) != nil {
		return m
	}
	for _, d := range docs {
		m[d.Key] = d.Name
	}
	return m
}

// setCommunityName saves a short name for an image key; blank removes it.
func setCommunityName(key, name string) error {
	initDatabase()
	coll := mongoClient.Database(dbName).Collection("community_names")
	if name == "" {
		_, err := coll.DeleteOne(context.TODO(), bson.D{{"key", key}})
		return err
	}
	_, err := coll.UpdateOne(context.TODO(), bson.D{{"key", key}},
		bson.D{{"$set", bson.D{{"key", key}, {"name", name}}}}, options.Update().SetUpsert(true))
	return err
}

// applyNames swaps in the short name for every image that has one (keys are unchanged).
func applyNames(f *communityFeed, names map[string]string) *communityFeed {
	out := &communityFeed{Generated: f.Generated, Images: make([]communityImage, len(f.Images))}
	copy(out.Images, f.Images)
	for i := range out.Images {
		if n := names[out.Images[i].Key]; n != "" {
			out.Images[i].Image = n
		}
	}
	return out
}

// ---- views ----

// stubbed in tests (no Mongo there)
var (
	hiddenLoader = loadCommunityHidden
	linksLoader  = loadCommunityLinks
	namesLoader  = loadCommunityNames
)

func communityVisible() (*communityFeed, error) {
	f, err := fetchCommunity()
	if err != nil {
		return nil, err
	}
	return applyNames(applyHidden(f, hiddenLoader()), namesLoader()), nil
}

func communityData(f *communityFeed, err error) gin.H {
	data := gin.H{"dur": communityDuration, "num": communityInt}
	if err != nil {
		data["err"] = err.Error()
	} else {
		data["feed"] = f
	}
	return data
}

func viewCommunity(c *gin.Context) {
	f, err := communityVisible()
	data := communityData(f, err)
	if err == nil {
		data["matrix"] = buildCommunityMatrix(f)
		data["links"] = linkMap(linksLoader())
	}
	c.HTML(http.StatusOK, "community.html", pageData(c, "Community Images", data))
}

func viewCommunityLines(c *gin.Context) {
	f, err := communityVisible()
	data := communityData(f, err)
	data["msg"] = c.Query("msg")
	c.HTML(http.StatusOK, "community_lines.html", pageData(c, "Community Image Lines", data))
}

func viewCommunityDetail(c *gin.Context) {
	f, err := communityVisible()
	data := communityData(f, err)
	key, user := c.Query("key"), c.Query("user")
	if err == nil {
		for _, img := range f.Images {
			if img.Key != key {
				continue
			}
			for i := range img.Entries {
				if img.Entries[i].User == user {
					data["image"], data["entry"] = img.Image, &img.Entries[i]
				}
			}
		}
		data["linked"] = linkMap(linksLoader())[user]
	}
	data["key"], data["who"] = key, user
	c.HTML(http.StatusOK, "community_detail.html", pageData(c, "Community Image Detail", data))
}

func viewManageCommunity(c *gin.Context) {
	f, err := fetchCommunity()
	data := communityData(f, err)
	h := loadCommunityHidden()
	data["msg"] = c.Query("msg")
	data["links"] = loadCommunityLinks()
	names := loadCommunityNames()
	type imgName struct{ Key, Full, Short string }
	imgs := []imgName{}
	if err == nil {
		for _, img := range f.Images {
			imgs = append(imgs, imgName{img.Key, img.Image, names[img.Key]})
		}
	}
	data["imgs"] = imgs
	// pictures still showing (deduped by hash) and Discord users seen in the feed
	type pic struct{ Hash, Image, User string }
	pics, users, hashes := []pic{}, []string{}, map[string]bool{}
	seenU := map[string]bool{}
	if err == nil {
		for _, img := range f.Images {
			for _, e := range img.Entries {
				if !seenU[e.User] {
					seenU[e.User], users = true, append(users, e.User)
				}
				for _, s := range []communityScore{e.Best, e.Latest} {
					hs := thumbHash(s.Thumb)
					if s.Thumb != "" && !h.Pics[hs] && !hashes[hs] {
						hashes[hs] = true
						pics = append(pics, pic{hs, img.Image, e.User})
					}
				}
			}
		}
	}
	sort.Strings(users)
	data["teams"] = knownAliases()
	data["pics"], data["users"] = pics, users
	c.HTML(http.StatusOK, "manage_community.html", pageData(c, "Community admin", data))
}

func communityBack(c *gin.Context, dest, msg string) {
	c.Redirect(http.StatusSeeOther, dest+"?msg="+url.QueryEscape(msg))
}

// knownAliases: team aliases (from sarpedon.conf and the live scores), sorted; used to pick the scoreboard side of a link.
func knownAliases() []string {
	seen, out := map[string]bool{}, []string{}
	add := func(a string) {
		if a != "" && !seen[strings.ToLower(a)] {
			seen[strings.ToLower(a)] = true
			out = append(out, a)
		}
	}
	for _, t := range sarpConfig.Team {
		add(publicName(t.ID, t.Alias))
	}
	if scores, e := getTop(); e == nil {
		for _, s := range scores {
			add(s.Team.Alias)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

// resolveAlias maps what the admin typed to a canonical alias: an alias (any case) is kept, a team id is
// converted to its alias. ok is false when it matches neither.
func resolveAlias(in string, known []string) (string, bool) {
	for _, a := range known {
		if strings.EqualFold(a, in) {
			return a, true
		}
	}
	if a := aliasFor(in); a != "" {
		return teamLabel(in), true
	}
	return "", false
}

func manageCommunityLink(c *gin.Context) {
	d, t := strings.TrimSpace(c.PostForm("discord")), strings.TrimSpace(c.PostForm("team"))
	if d == "" {
		communityBack(c, "/manage/community", "Discord user is required")
		return
	}
	if t != "" {
		a, ok := resolveAlias(t, knownAliases())
		if !ok {
			communityBack(c, "/manage/community", "unknown scoreboard alias: "+t)
			return
		}
		t = a
	}
	if err := setCommunityLink(d, t); err != nil {
		communityBack(c, "/manage/community", "could not save: "+err.Error())
		return
	}
	communityBack(c, "/manage/community", "saved link for "+d)
}

func manageCommunityRemovePic(c *gin.Context) {
	h := c.PostForm("hash")
	if !communityHashRe.MatchString(h) {
		communityBack(c, "/manage/community", "bad picture id")
		return
	}
	if err := addCommunityHidden("pic", h); err != nil {
		communityBack(c, "/manage/community", "could not remove: "+err.Error())
		return
	}
	communityBack(c, "/manage/community", "picture removed")
}

func manageCommunityDeleteLine(c *gin.Context) {
	key, user := c.PostForm("key"), c.PostForm("user")
	if key == "" || user == "" {
		communityBack(c, "/community/lines", "missing line")
		return
	}
	if err := addCommunityHidden("line", lineKey(key, user)); err != nil {
		communityBack(c, "/community/lines", "could not delete: "+err.Error())
		return
	}
	communityBack(c, "/community/lines", "deleted "+user)
}

func manageCommunityName(c *gin.Context) {
	key, name := strings.TrimSpace(c.PostForm("key")), strings.TrimSpace(c.PostForm("name"))
	if key == "" {
		communityBack(c, "/manage/community", "missing image")
		return
	}
	if err := setCommunityName(key, name); err != nil {
		communityBack(c, "/manage/community", "could not save: "+err.Error())
		return
	}
	communityBack(c, "/manage/community", "saved short name")
}
