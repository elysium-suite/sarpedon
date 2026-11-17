package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// manage.go adds the test-creator web console. These pages are behind authRequired
// (sarpedon.conf [[admin]] accounts) and drive the levelsvc competition engine over
// localhost, using the levelsvc admin token from sarpedon.conf (levelsvcToken).

func lvDo(method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, sarpConfig.LevelsvcUrl+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Admin-Token", sarpConfig.LevelsvcToken)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return http.DefaultClient.Do(req)
}

func lvGetJSON(path string, out interface{}) error {
	resp, err := lvDo("GET", path, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("levelsvc %s: %s", path, strings.TrimSpace(string(b)))
	}
	return json.Unmarshal(b, out)
}

// parseSubName turns "<team>_L<n>_<timestamp>.pka" into (team, n).
func parseSubName(name string) (string, string) {
	base := strings.TrimSuffix(name, ".pka")
	parts := strings.Split(base, "_")
	if len(parts) >= 3 && strings.HasPrefix(parts[len(parts)-2], "L") {
		return strings.Join(parts[:len(parts)-2], "_"), strings.TrimPrefix(parts[len(parts)-2], "L")
	}
	return "", ""
}

// aliasFor returns a team's alias from sarpedon.conf (by id, case-insensitive), or "".
func aliasFor(id string) string {
	for _, t := range sarpConfig.Team {
		if strings.EqualFold(t.ID, id) {
			return t.Alias
		}
	}
	return ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// safeFilePart keeps only filename-safe characters (others become '-').
func safeFilePart(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' ||
			(r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

// aliasFilename rewrites "<teamid>_L<n>[_<ts>].pka" to use the team's alias instead of the
// id for a friendlier download; falls back to the original name when there's no alias.
func aliasFilename(name string) string {
	base := strings.TrimSuffix(name, ".pka")
	parts := strings.Split(base, "_")
	li := -1
	for i, p := range parts {
		if len(p) >= 2 && p[0] == 'L' && isDigits(p[1:]) {
			li = i
			break
		}
	}
	if li <= 0 {
		return name
	}
	teamID := strings.Join(parts[:li], "_")
	alias := aliasFor(teamID)
	if alias == "" || strings.EqualFold(alias, teamID) {
		return name
	}
	return safeFilePart(alias) + "_" + strings.Join(parts[li:], "_") + ".pka"
}

func viewManage(c *gin.Context) {
	data := gin.H{"msg": c.Query("msg")}
	if sarpConfig.LevelsvcToken == "" {
		data["err"] = "levelsvcToken is not set in sarpedon.conf — add it (the levelsvc admin token)."
		c.HTML(http.StatusOK, "manage.html", pageData(c, "Competitions", data))
		return
	}
	var res struct {
		Competitions []map[string]interface{} `json:"competitions"`
	}
	if err := lvGetJSON("/admin/competitions", &res); err != nil {
		data["err"] = err.Error()
	} else {
		data["comps"] = res.Competitions
	}
	c.HTML(http.StatusOK, "manage.html", pageData(c, "Competitions", data))
}

func manageAction(c *gin.Context) {
	comp := c.PostForm("comp")
	action := c.PostForm("action")
	q := url.Values{"comp": {comp}, "action": {action}}
	msg := action + " " + comp
	resp, err := lvDo("POST", "/admin/competition?"+q.Encode(), nil, "")
	if err != nil {
		msg = "error: " + err.Error()
	} else {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			msg = "error: " + strings.TrimSpace(string(b))
		}
	}
	c.Redirect(http.StatusSeeOther, "/manage?msg="+url.QueryEscape(msg))
}

func viewManageNew(c *gin.Context) {
	c.HTML(http.StatusOK, "manage_new.html", pageData(c, "New Competition", nil))
}

func manageCreate(c *gin.Context) {
	fail := func(m string) {
		c.HTML(http.StatusOK, "manage_new.html", pageData(c, "New Competition", gin.H{"err": m}))
	}
	if err := c.Request.ParseMultipartForm(512 << 20); err != nil {
		fail(err.Error())
		return
	}
	if strings.TrimSpace(c.PostForm("name")) == "" {
		fail("Enter a competition name.")
		return
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("name", c.PostForm("name"))
	if c.PostForm("practice") != "" {
		mw.WriteField("practice", "1")
	}
	mw.WriteField("vis", "1")
	if c.PostForm("public") != "" {
		mw.WriteField("public", "on")
	}
	mw.WriteField("allowedTeams", c.PostForm("allowedTeams"))
	mw.WriteField("aiopt", "1")
	if c.PostForm("aiCheck") != "" {
		mw.WriteField("aiCheck", "1")
	}
	k := 0
	for i := 1; i <= 30; i++ {
		fhs := c.Request.MultipartForm.File[fmt.Sprintf("level%d", i)]
		if len(fhs) == 0 || fhs[0].Filename == "" {
			continue
		}
		k++
		mw.WriteField(fmt.Sprintf("threshold%d", k), c.PostForm(fmt.Sprintf("threshold%d", i)))
		mw.WriteField(fmt.Sprintf("levelname%d", k), c.PostForm(fmt.Sprintf("levelname%d", i)))
		f, err := fhs[0].Open()
		if err != nil {
			continue
		}
		fw, _ := mw.CreateFormFile(fmt.Sprintf("level%d", k), fhs[0].Filename)
		io.Copy(fw, f)
		f.Close()
	}
	mw.Close()
	if k == 0 {
		fail("Add at least one level (choose a .pka file).")
		return
	}
	resp, err := lvDo("POST", "/admin/create", &buf, mw.FormDataContentType())
	if err != nil {
		fail(err.Error())
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fail(strings.TrimSpace(string(b)))
		return
	}
	c.Redirect(http.StatusSeeOther, "/manage?msg="+url.QueryEscape("created competition"))
}

func viewManageEdit(c *gin.Context) {
	comp := c.Query("comp")
	if comp == "" {
		c.Redirect(http.StatusSeeOther, "/manage")
		return
	}
	var ac struct {
		Comp   string                   `json:"comp"`
		Name   string                   `json:"name"`
		Levels []map[string]interface{} `json:"levels"`
	}
	if err := lvGetJSON("/admin/agentconfig?comp="+url.QueryEscape(comp), &ac); err != nil {
		c.HTML(http.StatusOK, "manage.html", pageData(c, "Competitions", gin.H{"err": err.Error()}))
		return
	}
	for i := range ac.Levels {
		if s, _ := ac.Levels[i]["levelName"].(string); s == "" {
			if img, _ := ac.Levels[i]["image"].(string); img != "" {
				ac.Levels[i]["levelName"] = img
			} else {
				ac.Levels[i]["levelName"] = fmt.Sprintf("Level %d", i+1)
			}
		}
	}
	practice := false
	private := false
	aiCheck := false
	allowed := []string{}
	var comps struct {
		Competitions []map[string]interface{} `json:"competitions"`
	}
	if lvGetJSON("/admin/competitions", &comps) == nil {
		for _, cp := range comps.Competitions {
			if id, _ := cp["id"].(string); id == comp {
				if p, ok := cp["practice"].(bool); ok {
					practice = p
				}
				if p, ok := cp["private"].(bool); ok {
					private = p
				}
				if p, ok := cp["aiCheck"].(bool); ok {
					aiCheck = p
				}
				if arr, ok := cp["allowedTeams"].([]interface{}); ok {
					for _, v := range arr {
						if s, ok := v.(string); ok {
							allowed = append(allowed, s)
						}
					}
				}
			}
		}
	}
	c.HTML(http.StatusOK, "manage_edit.html", pageData(c, "Edit Competition", gin.H{
		"comp": comp, "name": ac.Name, "practice": practice,
		"private": private, "aiCheck": aiCheck, "allowedTeams": strings.Join(allowed, "\n"), "levels": ac.Levels}))
}

func manageUpdate(c *gin.Context) {
	comp := c.PostForm("comp")
	fail := func(m string) {
		c.HTML(http.StatusOK, "manage.html", pageData(c, "Competitions", gin.H{"err": m}))
	}
	if err := c.Request.ParseMultipartForm(512 << 20); err != nil {
		fail(err.Error())
		return
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("comp", comp)
	mw.WriteField("name", c.PostForm("name"))
	if c.PostForm("practice") != "" {
		mw.WriteField("practice", "1")
	}
	mw.WriteField("vis", "1")
	if c.PostForm("public") != "" {
		mw.WriteField("public", "on")
	}
	mw.WriteField("allowedTeams", c.PostForm("allowedTeams"))
	mw.WriteField("aiopt", "1")
	if c.PostForm("aiCheck") != "" {
		mw.WriteField("aiCheck", "1")
	}
	k := 0
	for i := 1; i <= 50; i++ {
		fhs := c.Request.MultipartForm.File[fmt.Sprintf("level%d", i)]
		hasFile := len(fhs) > 0 && fhs[0].Filename != ""
		keep := c.PostForm(fmt.Sprintf("keep%d", i))
		if !hasFile && keep == "" {
			continue
		}
		k++
		mw.WriteField(fmt.Sprintf("threshold%d", k), c.PostForm(fmt.Sprintf("threshold%d", i)))
		mw.WriteField(fmt.Sprintf("levelname%d", k), c.PostForm(fmt.Sprintf("levelname%d", i)))
		if hasFile {
			f, err := fhs[0].Open()
			if err != nil {
				continue
			}
			fw, _ := mw.CreateFormFile(fmt.Sprintf("level%d", k), fhs[0].Filename)
			io.Copy(fw, f)
			f.Close()
		} else {
			mw.WriteField(fmt.Sprintf("keep%d", k), keep)
		}
	}
	mw.Close()
	if k == 0 {
		fail("A competition needs at least one level.")
		return
	}
	resp, err := lvDo("POST", "/admin/update", &buf, mw.FormDataContentType())
	if err != nil {
		fail(err.Error())
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fail(strings.TrimSpace(string(b)))
		return
	}
	c.Redirect(http.StatusSeeOther, "/manage?msg="+url.QueryEscape("updated "+comp))
}

func viewReview(c *gin.Context) {
	comp := c.Query("comp")
	if comp == "" {
		c.Redirect(http.StatusSeeOther, "/manage")
		return
	}
	var subs struct {
		Submissions []map[string]interface{} `json:"submissions"`
	}
	var prog struct {
		Progress []map[string]interface{} `json:"progress"`
	}
	lvGetJSON("/admin/submissions?comp="+url.QueryEscape(comp), &subs)
	lvGetJSON("/admin/progress?comp="+url.QueryEscape(comp), &prog)
	for _, s := range subs.Submissions {
		if name, ok := s["name"].(string); ok {
			t, n := parseSubName(name)
			s["team"] = t
			s["level"] = n
			s["teamAlias"] = aliasFor(t)
		}
	}
	for _, p := range prog.Progress {
		if t, ok := p["team"].(string); ok {
			p["teamAlias"] = aliasFor(t)
		}
	}
	// levelsvc reports file times in UTC; show them in the configured timezone like every other page
	for _, rows := range [][]map[string]interface{}{subs.Submissions, prog.Progress} {
		for _, r := range rows {
			if m, ok := r["modified"].(string); ok {
				r["modified"] = localTimestamp(m)
			}
		}
	}
	c.HTML(http.StatusOK, "manage_review.html", pageData(c, "Review", gin.H{
		"comp": comp, "submissions": subs.Submissions, "progress": prog.Progress, "msg": c.Query("msg")}))
}

func viewAgent(c *gin.Context) {
	var latest struct {
		Versions map[string]string `json:"versions"` // platform -> published version
	}
	lvGetJSON("/agent/latest", &latest) // best-effort; empty when nothing published yet
	c.HTML(http.StatusOK, "manage_agent.html", pageData(c, "App Update", gin.H{
		"versions": latest.Versions, "msg": c.Query("msg")}))
}

func manageAgentPublish(c *gin.Context) {
	fail := func(m string) {
		c.HTML(http.StatusOK, "manage_agent.html", pageData(c, "App Update", gin.H{"err": m}))
	}
	if err := c.Request.ParseMultipartForm(256 << 20); err != nil {
		fail(err.Error())
		return
	}
	platform := c.PostForm("platform")
	version := strings.TrimSpace(c.PostForm("version"))
	if platform == "" || version == "" {
		fail("Choose a platform and enter a version.")
		return
	}
	fhs := c.Request.MultipartForm.File["binary"]
	if len(fhs) == 0 || fhs[0].Filename == "" {
		fail("Choose the built agent file to upload.")
		return
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("platform", platform)
	mw.WriteField("version", version)
	f, err := fhs[0].Open()
	if err != nil {
		fail("cannot read the uploaded file")
		return
	}
	fw, _ := mw.CreateFormFile("binary", fhs[0].Filename)
	io.Copy(fw, f)
	f.Close()
	mw.Close()
	resp, err := lvDo("POST", "/admin/agentpublish?platform="+url.QueryEscape(platform)+"&version="+url.QueryEscape(version), &buf, mw.FormDataContentType())
	if err != nil {
		fail(err.Error())
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fail(strings.TrimSpace(string(b)))
		return
	}
	c.Redirect(http.StatusSeeOther, "/manage/agent?msg="+url.QueryEscape("published "+platform+" "+version))
}

func manageDownload(c *gin.Context) {
	comp := c.Query("comp")
	name := c.Query("name")
	path := "/admin/submission"
	if c.Query("kind") == "progress" {
		path = "/admin/progressfile"
	}
	resp, err := lvDo("GET", path+"?comp="+url.QueryEscape(comp)+"&name="+url.QueryEscape(name), nil, "")
	if err != nil {
		errorOut(c, err)
		return
	}
	defer resp.Body.Close()
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", aliasFilename(name)))
	c.DataFromReader(resp.StatusCode, resp.ContentLength, "application/octet-stream", resp.Body, nil)
}

func manageRestore(c *gin.Context) {
	comp := c.PostForm("comp")
	q := url.Values{
		"comp": {comp}, "kind": {c.PostForm("kind")},
		"name": {c.PostForm("name")}, "team": {c.PostForm("team")}, "n": {c.PostForm("n")},
	}
	msg := "restored"
	resp, err := lvDo("POST", "/admin/restore?"+q.Encode(), nil, "")
	if err != nil {
		msg = "error: " + err.Error()
	} else {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			msg = "error: " + strings.TrimSpace(string(b))
		}
	}
	c.Redirect(http.StatusSeeOther, "/manage/review?comp="+url.QueryEscape(comp)+"&msg="+url.QueryEscape(msg))
}
