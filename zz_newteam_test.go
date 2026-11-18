package main

import (
	"os"
	"testing"
)

// A team made on first sight must have an alias, or the next start fails validation ("Team alias is empty").
func TestCreateNewTeamIdSetsAlias(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	saved := sarpConfig
	defer func() { sarpConfig = saved }()
	sarpConfig.Team = nil

	team := createNewTeamId("NewKid")
	if team.ID != "NewKid" || team.Alias != "NewKid" {
		t.Fatalf("got %+v", team)
	}
}
