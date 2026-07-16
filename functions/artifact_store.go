package functions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FilesystemArtifactStore stores immutable artifacts under a root directory.
type FilesystemArtifactStore struct {
	root string
	mu   sync.Mutex
}

// NewFilesystemArtifactStore creates a store rooted at dir (created if missing).
func NewFilesystemArtifactStore(dir string) (*FilesystemArtifactStore, error) {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "apito-function-artifacts")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &FilesystemArtifactStore{root: dir}, nil
}

func (s *FilesystemArtifactStore) Put(ctx context.Context, key string, data []byte, hash string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if hash == "" {
		sum := sha256.Sum256(data)
		hash = hex.EncodeToString(sum[:])
	}
	path := s.pathFor(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	meta := path + ".sha256"
	if err := os.WriteFile(meta, []byte(hash), 0o640); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func (s *FilesystemArtifactStore) Get(ctx context.Context, key string) ([]byte, string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.pathFor(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	hashBytes, _ := os.ReadFile(path + ".sha256")
	hash := string(hashBytes)
	if hash == "" {
		sum := sha256.Sum256(data)
		hash = hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if hash != actual {
		return nil, "", fmt.Errorf("artifact hash mismatch for %s", key)
	}
	return data, hash, nil
}

func (s *FilesystemArtifactStore) Delete(ctx context.Context, key string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.pathFor(key)
	_ = os.Remove(path + ".sha256")
	return os.Remove(path)
}

func (s *FilesystemArtifactStore) pathFor(key string) string {
	clean := filepath.Clean("/" + key)
	return filepath.Join(s.root, clean)
}
