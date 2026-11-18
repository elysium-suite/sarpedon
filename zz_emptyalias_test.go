package main

import "testing"

// A team with an empty alias must not stop Sarpedon from starting: checkConfig uses the ID.
func TestCheckConfigDefaultsEmptyAlias(t *testing.T) {
	saved := sarpConfig
	defer func() { sarpConfig = saved }()
	sarpConfig.Password = "x"
	sarpConfig.Admin = []adminData{{}}
	sarpConfig.Image = []imageData{{Name: "img"}}
	sarpConfig.Timezone = "UTC"
	sarpConfig.Team = []teamData{{ID: "NewKid"}}
	checkConfig()
	if got := sarpConfig.Team[0].Alias; got != "NewKid" {
		t.Fatalf("alias = %q, want NewKid", got)
	}
}
