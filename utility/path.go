package utility

import (
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands ~ to home directory and converts to absolute path.
// If the path starts with ./ or ../, it resolves relative to the current working directory.
func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(homeDir, path[2:])
	} else if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		// Leave as-is; filepath.Abs will resolve relative to current working directory
		// No transformation needed
	}
	return filepath.Abs(path)
}