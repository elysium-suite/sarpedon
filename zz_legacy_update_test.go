package main

import (
	"strings"
	"testing"
)

// legacyUpdate builds a score update the way aeacus 1.8.x does: ASCII delimiter and a challenge field.
func legacyUpdate(team, image, score, vulns string) string {
	d := legacyDelimiter
	return "team" + d + team + d + "image" + d + image + d + "score" + d + score + d +
		"challenge" + d + "a1b2c3d4e5" + d + "vulns" + d + vulns + d + "time" + d + "1790000000" + d
}

func modernUpdate(team, image, score, vulns string) string {
	d := delimiter
	return "team" + d + team + d + "image" + d + image + d + "score" + d + score + d +
		"vulns" + d + vulns + d + "time" + d + "1790000000" + d
}

func withTestConfig(t *testing.T) {
	t.Helper()
	old := sarpConfig
	sarpConfig = config{Password: "global-pw", Team: []teamData{{ID: "TeamOne"}}, Image: []imageData{{Name: "ssh-mini-image", Password: "image-pw"}}}
	t.Cleanup(func() { sarpConfig = old })
}

func TestUpdateDelimiterPicksTheVersionsOwn(t *testing.T) {
	if updateDelimiter(legacyUpdate("a", "b", "1", "c")) != legacyDelimiter {
		t.Error("1.8.x update not recognised")
	}
	if updateDelimiter(modernUpdate("a", "b", "1", "c")) != delimiter {
		t.Error("2.x update not recognised")
	}
	if updateDelimiter("nothing to split on") != delimiter {
		t.Error("an update with neither delimiter should keep the 2.x one")
	}
}

func TestValidateUpdateAcceptsBothVersions(t *testing.T) {
	withTestConfig(t)
	if err := validateUpdate(legacyUpdate("TeamOne", "ssh-mini-image", "10", "abcdef")); err != nil {
		t.Errorf("1.8.x update rejected: %v", err)
	}
	if err := validateUpdate(modernUpdate("TeamOne", "ssh-mini-image", "10", "abcdef")); err != nil {
		t.Errorf("2.x update rejected: %v", err)
	}
}

func TestValidateUpdateStillRejectsBadOnes(t *testing.T) {
	withTestConfig(t)
	for name, u := range map[string]string{
		"legacy unknown team":  legacyUpdate("Nobody", "ssh-mini-image", "10", "abcdef"),
		"legacy unknown image": legacyUpdate("TeamOne", "other", "10", "abcdef"),
		"modern unknown team":  modernUpdate("Nobody", "ssh-mini-image", "10", "abcdef"),
		"too short":            "team" + legacyDelimiter + "TeamOne" + legacyDelimiter,
	} {
		if validateUpdate(u) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLegacyUpdateEndToEndDecryptsWithTheImagePassword(t *testing.T) {
	withTestConfig(t)
	plain := legacyUpdate("TeamOne", "ssh-mini-image", "10", "abcdef")
	crypt, err := hexDecode(hexEncode(encryptString("image-pw", plain)))
	if err != nil {
		t.Fatal(err)
	}
	got, key, err := decryptUpdate(crypt)
	if err != nil || strings.TrimRight(got, " ") != plain || key != "image-pw" { // encryptString pads with spaces
		t.Fatalf("decrypt: %v key=%q same=%v", err, key, strings.TrimRight(got, " ") == plain)
	}
	if err := validateUpdate(got); err != nil {
		t.Fatal(err)
	}
	fields := legacyFields(strings.Split(got, legacyDelimiter))
	if fields["team"] != "TeamOne" || fields["image"] != "ssh-mini-image" || fields["score"] != "10" || fields["vulns"] != "abcdef" {
		t.Errorf("fields: %v", fields)
	}
}

func TestParseVulnsReadsBothDelimiters(t *testing.T) {
	for name, d := range map[string]string{"2.x": delimiter, "1.8.x": legacyDelimiter} {
		plain := "2" + d + "3" + d + "Removed bad user - 5 pts" + d + "Fixed ssh - 5 pts" + d + "Weak password - N2 pts" + d
		enc := hexEncode(encryptString("k", plain))
		w, err := parseVulns(enc, 8, "k")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if w.VulnsScored != 2 || w.VulnsTotal != 3 || len(w.VulnItems) != 3 || w.VulnItems[2].VulnPoints != -2 {
			t.Errorf("%s: parsed %+v", name, w)
		}
	}
}
