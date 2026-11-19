package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// (A typed-in name that is its own alias, e.g. "xinya", is shown as is: see TestDisplayAlias.)
// A team ID is a credential: no name we show may equal it.
func TestNoTeamNameIsAnID(t *testing.T) {
	sarpConfig.Team = []teamData{{ID: "SecretID01", Alias: "SecretID01"}, {ID: "OtherID02", Alias: "ok"}}
	defer func() { sarpConfig.Team = nil }()
	for _, tm := range []teamData{{ID: "SecretID01"}, {ID: "SecretID01", Alias: "secretid01"}, {ID: "Unknown9ab", Alias: "Unknown9ab"}, {ID: "Unknown9ab"}} {
		got := displayAlias(tm)
		if strings.EqualFold(got, tm.ID) || !strings.HasPrefix(got, "Team-") {
			t.Errorf("%+v shows %q", tm, got)
		}
	}
	if got := teamLabel("SecretID01"); strings.Contains(got, "SecretID01") {
		t.Errorf("label %q", got)
	}
	if teamLabel("OtherID02") != "ok" || displayAlias(teamData{ID: "OtherID02", Alias: "OtherID02"}) != "ok" {
		t.Error("configured alias should win")
	}
	if got := aliasFilename("SecretID01_L2_20260101.pka"); strings.Contains(got, "SecretID01") {
		t.Errorf("download name %q leaks the id", got)
	}
	if label := publicName("SecretID01", ""); !validateTeam(label) {
		t.Error("the hashed label must resolve back to its team")
	}
}

// No template may print a team's ID.
func TestTemplatesNeverPrintTeamID(t *testing.T) {
	bad := regexp.MustCompile(`\{\{[^}]*(Team\.ID|\$team\.ID|\.Team\b\s*\}\}[^{]*ID)[^}]*\}\}`)
	files, _ := filepath.Glob("templates/*.html")
	for _, f := range files {
		data, _ := os.ReadFile(f)
		if m := bad.FindString(string(data)); m != "" {
			t.Errorf("%s prints a team id: %s", f, m)
		}
	}
}
