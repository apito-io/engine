package registry

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const defaultMaxUncompressed = 256 << 20 // 256 MiB

// SafeExtractZip extracts a zip into dest. Only root-level regular files.
// Rejects zip-slip, symlinks, absolute paths, nested dirs, and duplicate names.
func SafeExtractZip(zipPath, dest string, expectedBinary string, maxUncompressed int64) error {
	if maxUncompressed <= 0 {
		maxUncompressed = defaultMaxUncompressed
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return coded(CodeUnsafeArchive, "cannot open zip", err)
	}
	defer r.Close()

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("mkdir dest: %w", err)
	}

	seen := map[string]struct{}{}
	var uncompressed int64
	var sawConfig, sawBinary bool

	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		name = strings.TrimPrefix(name, "./")
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
			return coded(CodeUnsafeArchive, "archive path is not a root-level file: "+f.Name, nil)
		}
		if strings.Contains(name, "/") {
			return coded(CodeUnsafeArchive, "archive must contain only root-level files (found "+f.Name+")", nil)
		}
		base := filepath.Base(name)
		if base != name {
			return coded(CodeUnsafeArchive, "unsafe archive name "+f.Name, nil)
		}
		if _, dup := seen[base]; dup {
			return coded(CodeUnsafeArchive, "duplicate archive entry "+base, nil)
		}
		seen[base] = struct{}{}

		if f.Mode()&os.ModeSymlink != 0 || f.Mode()&os.ModeDevice != 0 || f.Mode()&os.ModeNamedPipe != 0 {
			return coded(CodeUnsafeArchive, "archive contains symlink or device file: "+base, nil)
		}
		if !f.Mode().IsRegular() && f.Mode()&os.ModePerm == 0 && f.FileInfo() != nil && !f.FileInfo().Mode().IsRegular() {
			// zip.FileHeader Mode may be 0 on some archives; still reject dirs (already skipped) and check type.
		}

		uncompressed += int64(f.UncompressedSize64)
		if uncompressed > maxUncompressed {
			return coded(CodeUnsafeArchive, "uncompressed archive exceeds size limit", nil)
		}

		rc, err := f.Open()
		if err != nil {
			return coded(CodeUnsafeArchive, "cannot read "+base, err)
		}
		outPath := filepath.Join(dest, base)
		if !strings.HasPrefix(outPath, filepath.Clean(dest)+string(os.PathSeparator)) && outPath != filepath.Clean(dest) {
			rc.Close()
			return coded(CodeUnsafeArchive, "zip-slip rejected for "+base, nil)
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		n, copyErr := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
		out.Close()
		rc.Close()
		if copyErr != nil {
			return coded(CodeUnsafeArchive, "extract "+base, copyErr)
		}
		if uint64(n) != f.UncompressedSize64 {
			return coded(CodeUnsafeArchive, "size mismatch extracting "+base, nil)
		}

		switch base {
		case "config.yml":
			sawConfig = true
		case expectedBinary, expectedBinary + ".exe":
			sawBinary = true
			_ = os.Chmod(outPath, 0o755)
		}
	}

	if !sawConfig {
		return coded(CodeConfigMismatch, "archive missing config.yml", nil)
	}
	if expectedBinary != "" && !sawBinary {
		return coded(CodeConfigMismatch, "archive missing binary "+expectedBinary, nil)
	}
	return nil
}
