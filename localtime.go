package main

import (
	"sync"
	"time"
)

var (
	zoneOnce sync.Once
	zone     *time.Location
)

// localZone is the timezone from sarpedon.conf ("timezone"); UTC if it can't be loaded.
func localZone() *time.Location {
	zoneOnce.Do(func() {
		zone = time.UTC
		if l, err := time.LoadLocation(sarpConfig.Timezone); err == nil {
			zone = l
		}
	})
	return zone
}

// localTime converts a timestamp to the configured timezone for display. Zero times are left
// alone so "not completed yet" checks still work.
func localTime(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(localZone())
}

// localTimestamp formats an RFC 3339 string (as levelsvc sends them, in UTC) in the configured
// timezone, e.g. "2026-10-07 20:43:38 PDT". Anything unparseable is returned unchanged.
func localTimestamp(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return localTime(t).Format("2006-01-02 15:04:05 MST")
}
