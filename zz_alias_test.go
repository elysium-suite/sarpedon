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
		{teamData{ID: "plain", Alias: "plain"}, "plain"}, // a typed-in name is its own alias
		{teamData{ID: "xinya", Alias: "xinya"}, "xinya"},
		{teamData{ID: "SecretID01", Alias: "SecretID01"}, publicName("SecretID01", "")}, // credential-shaped: hashed label
		{teamData{ID: "tt-0123456789abcdef", Alias: "tt-0123456789abcdef"}, publicName("tt-0123456789abcdef", "")},
	}
	for _, c := range cases {
		if got := displayAlias(c.in); got != c.want {
			t.Errorf("%+v: got %q want %q", c.in, got, c.want)
		}
	}
}
