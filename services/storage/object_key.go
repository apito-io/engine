package storage

import (
	"fmt"
	"strings"
)

// BuildObjectKey returns the canonical storage key: {project_id}/{file_type}/{uuid}{ext}.
func BuildObjectKey(projectID, fileType, fileID, ext string) string {
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("%s/%s/%s%s", projectID, fileType, fileID, ext)
}
