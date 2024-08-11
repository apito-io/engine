package utility

import "time"

func GetCurrentTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func AddDaysWithCurrentTime(days int) string {
	return time.Now().UTC().AddDate(0, 0, days).Format(time.RFC3339)
}

func GetCurrentTimeObject() time.Time {
	return time.Now().UTC()
}
