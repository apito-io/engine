package utility

import "os"

func GetEnv(key, defaults string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaults
}
