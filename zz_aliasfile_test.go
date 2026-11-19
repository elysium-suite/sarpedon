package main

import (
	"os"
	"testing"
	"time"
)

func inTempDir(t *testing.T) {
	t.Helper()
	old, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	saved, savedLoaded := sarpConfig, confTeamsLoaded
	t.Cleanup(func() { os.Chdir(old); sarpConfig, confTeamsLoaded = saved, savedLoaded })
}

// An alias an admin edits in sarpedon.conf shows without restarting the server.
func TestAliasEditedInFileShowsWithoutRestart(t *testing.T) {
	inTempDir(t)
	sarpConfig.Team = []teamData{{ID: "JSaV8BOsvC", Alias: "JSaV8BOsvC"}}
	os.WriteFile("sarpedon.conf", []byte("[[team]]\nID = \"JSaV8BOsvC\"\nAlias = \"coolturtle\"\n"), 0600)
	os.Chtimes("sarpedon.conf", time.Now(), time.Now().Add(time.Second))
	if got := displayAlias(teamData{ID: "JSaV8BOsvC", Alias: "JSaV8BOsvC"}); got != "coolturtle" {
		t.Fatalf("got %q", got)
	}
}

// Creating a team must not write the stale in-memory config over an alias edited in the file.
func TestCreatingTeamKeepsHandEditedAlias(t *testing.T) {
	inTempDir(t)
	sarpConfig.Team = []teamData{{ID: "JSaV8BOsvC", Alias: "JSaV8BOsvC"}} // memory is stale
	os.WriteFile("sarpedon.conf", []byte("[[team]]\nID = \"JSaV8BOsvC\"\nAlias = \"coolturtle\"\n"), 0600)
	createNewTeamId("another")
	data, _ := os.ReadFile("sarpedon.conf")
	var onDisk config
	readConfig(&onDisk)
	if !hasTeamID(onDisk.Team, "another") || onDisk.Team[0].Alias != "coolturtle" {
		t.Fatalf("file now: %s", data)
	}
	if got := displayAlias(teamData{ID: "JSaV8BOsvC", Alias: "JSaV8BOsvC"}); got != "coolturtle" {
		t.Fatalf("got %q", got)
	}
}

func TestWithDisplayAliasFixesEveryScore(t *testing.T) {
	sarpConfig.Team = []teamData{{ID: "u1", Alias: "alice"}}
	defer func() { sarpConfig.Team = nil }()
	got := withDisplayAlias([]scoreEntry{{Team: teamData{ID: "u1", Alias: "u1"}}, {Team: teamData{ID: "tt-0123456789abcdef", Alias: "tt-0123456789abcdef"}}})
	if got[0].Team.Alias != "alice" || got[1].Team.Alias != publicName("tt-0123456789abcdef", "") {
		t.Fatalf("got %+v", got)
	}
}
