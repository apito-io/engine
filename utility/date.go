package utility

import (
	"strings"
	"time"
)

// FormatReadableDate turns RFC3339 / date-only timestamps into "2 January 2006".
// Empty or unparseable values return fallback (or the raw string if fallback is empty).
func FormatReadableDate(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return strings.TrimSpace(fallback)
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Format("2 January 2006")
		}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC().Format("2 January 2006")
	}
	return raw
}

// UTC time functions
func GetCurrentTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func AddDaysWithCurrentTime(days int) string {
	return time.Now().UTC().AddDate(0, 0, days).Format(time.RFC3339)
}

func GetCurrentTimeObject() time.Time {
	return time.Now().UTC()
}

// Dhaka time functions
func GetDhakaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		// Fallback to fixed offset if timezone data is unavailable
		loc = time.FixedZone("Asia/Dhaka", 6*60*60) // UTC+6
	}
	return loc
}

func GetCurrentDhakaTimeDateOnly() string {
	return time.Now().In(GetDhakaLocation()).Format("2006-01-02")
}

func GetCurrentDhakaTime() string {
	return time.Now().In(GetDhakaLocation()).Format(time.RFC3339)
}

func AddDaysWithCurrentDhakaTime(days int) string {
	return time.Now().In(GetDhakaLocation()).AddDate(0, 0, days).Format(time.RFC3339)
}

func GetCurrentDhakaTimeObject() time.Time {
	return time.Now().In(GetDhakaLocation())
}
