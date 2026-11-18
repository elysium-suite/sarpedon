package main

import "testing"

func TestDisplayAlias(t *testing.T) {
	old := sarpConfig.Team
	defer func() { sarpConfig.Team = old }()
	sarpConfig.Team = []teamData{{ID: "JSaV8BOsvC", Alias: "coolturtle"}, {ID: "plain", Alias: "plain"}}
	cases := []struct {
		in   teamData
		want string
	}{
		{teamData{ID: "JSaV8BOsvC", Alias: "JSaV8BOsvC"}, "coolturtle"},
		{teamData{ID: "JSaV8BOsvC"}, "coolturtle"},
		{teamData{ID: "JSaV8BOsvC", Alias: "stored"}, "stored"},
		{teamData{ID: "plain", Alias: "plain"}, "plain"},
		{teamData{ID: "unknown", Alias: "unknown"}, "unknown"},
	}
	for _, c := range cases {
		if got := displayAlias(c.in); got != c.want {
			t.Errorf("%+v: got %q want %q", c.in, got, c.want)
		}
	}
}
