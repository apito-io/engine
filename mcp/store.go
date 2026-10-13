package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store is the durable OAuth client, transaction, and credential record.
// It is not the process-local device-login map.
type Store struct {
	path string
	mu   sync.Mutex
	data fileData
}

type fileData struct {
	Clients  map[string]ClientRecord  `json:"clients"`
	Requests map[string]AuthRequest   `json:"requests"`
	Codes    map[string]CodeRecord    `json:"codes"`
	Access   map[string]AccessRecord  `json:"access"`
	Refresh  map[string]RefreshRecord `json:"refresh"`
}

type ClientRecord struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	RedirectURIs []string  `json:"redirect_uris"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuthRequest struct {
	ID            string    `json:"id"`
	ClientID      string    `json:"client_id"`
	RedirectURI   string    `json:"redirect_uri"`
	CodeChallenge string    `json:"code_challenge"`
	Method        string    `json:"method"`
	Resource      string    `json:"resource"`
	State         string    `json:"state"`
	Scope         string    `json:"scope"`
	ExpiresAt     time.Time `json:"expires_at"`
	Status        string    `json:"status"`
}

type CodeRecord struct {
	Hash          string              `json:"hash"`
	ClientID      string              `json:"client_id"`
	RedirectURI   string              `json:"redirect_uri"`
	Challenge     string              `json:"challenge"`
	Method        string              `json:"method"`
	Resource      string              `json:"resource"`
	UserID        string              `json:"user_id"`
	InternalToken string              `json:"internal_token,omitempty"`
	ProjectMode   string              `json:"project_mode"`
	ProjectIDs    []string            `json:"project_ids"`
	TenantMode    string              `json:"tenant_mode"`
	TenantIDs     map[string][]string `json:"tenant_ids"`
	ExpiresAt     time.Time           `json:"expires_at"`
	Used          bool                `json:"used"`
}

type AccessRecord struct {
	Hash          string    `json:"hash"`
	ClientID      string    `json:"client_id"`
	Resource      string    `json:"resource"`
	UserID        string    `json:"user_id"`
	InternalToken string    `json:"internal_token,omitempty"`
	Scope         string    `json:"scope"`
	ExpiresAt     time.Time `json:"expires_at"`
	RefreshHash   string    `json:"refresh_hash,omitempty"`
}

type RefreshRecord struct {
	Hash          string    `json:"hash"`
	ClientID      string    `json:"client_id"`
	Resource      string    `json:"resource"`
	UserID        string    `json:"user_id"`
	InternalToken string    `json:"internal_token,omitempty"`
	Scope         string    `json:"scope"`
	ExpiresAt     time.Time `json:"expires_at"`
	Used          bool      `json:"used"`
}

func OpenStore(path string) (*Store, error) {
	if path == "" {
		return nil, os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	s := &Store{path: path, data: newFileData()}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, s.flushLocked()
		}
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, err
		}
	}
	if s.data.Clients == nil {
		s.data = newFileData()
	}
	return s, nil
}

func newFileData() fileData {
	return fileData{
		Clients:  map[string]ClientRecord{},
		Requests: map[string]AuthRequest{},
		Codes:    map[string]CodeRecord{},
		Access:   map[string]AccessRecord{},
		Refresh:  map[string]RefreshRecord{},
	}
}

func (s *Store) flushLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Store) SaveClient(rec ClientRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Clients[rec.ID] = rec
	return s.flushLocked()
}

func (s *Store) Client(id string) (ClientRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data.Clients[id]
	return rec, ok
}

func (s *Store) SaveRequest(rec AuthRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(time.Now())
	s.data.Requests[rec.ID] = rec
	return s.flushLocked()
}

func (s *Store) Request(id string) (AuthRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data.Requests[id]
	return rec, ok
}

func (s *Store) SaveCode(raw string, rec CodeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.Hash = tokenHash(raw)
	s.data.Codes[rec.Hash] = rec
	return s.flushLocked()
}

func (s *Store) TakeCode(raw string) (CodeRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data.Codes[tokenHash(raw)]
	if !ok || rec.Used || time.Now().After(rec.ExpiresAt) {
		return CodeRecord{}, false
	}
	rec.Used = true
	s.data.Codes[rec.Hash] = rec
	_ = s.flushLocked()
	return rec, true
}

func (s *Store) SaveAccess(raw string, rec AccessRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.Hash = tokenHash(raw)
	s.data.Access[rec.Hash] = rec
	return s.flushLocked()
}

func (s *Store) Access(raw string) (AccessRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data.Access[tokenHash(raw)]
	if !ok || time.Now().After(rec.ExpiresAt) {
		return AccessRecord{}, false
	}
	return rec, true
}

func (s *Store) SaveRefresh(raw string, rec RefreshRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.Hash = tokenHash(raw)
	s.data.Refresh[rec.Hash] = rec
	return s.flushLocked()
}

func (s *Store) TakeRefresh(raw string) (RefreshRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.data.Refresh[tokenHash(raw)]
	if !ok || rec.Used || time.Now().After(rec.ExpiresAt) {
		return RefreshRecord{}, false
	}
	rec.Used = true
	s.data.Refresh[rec.Hash] = rec
	_ = s.flushLocked()
	return rec, true
}

func (s *Store) expireLocked(now time.Time) {
	for id, rec := range s.data.Requests {
		if now.After(rec.ExpiresAt) {
			delete(s.data.Requests, id)
		}
	}
}
