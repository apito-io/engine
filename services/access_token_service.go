package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenSecretBytes = 32
	accessTokenCacheTTL    = 2 * time.Minute
)

var (
	errTokenInvalid      = errors.New("invalid or expired access token")
	errTokenRevoked      = errors.New("access token revoked")
	errTokenFormat       = errors.New("TOKEN_FORMAT_RETIRED: use apt_ access tokens")
	errProjectNotAllowed = errors.New("project not in token grant or not administrable by issuer")
	errDangerRequired    = errors.New("danger acknowledgement required for high-risk capabilities")
)

// AccessTokenStore is the persistence surface used by AccessTokenService.
type AccessTokenStore interface {
	GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error)
	UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error
	FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error)
	CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error)
}

// AccessTokenService mints, validates, revokes, and rotates opaque apt_ tokens.
type AccessTokenService struct {
	db    AccessTokenStore
	cfg   *models.Config
	cache sync.Map // tokenID -> *cachedToken
}

type cachedToken struct {
	record    *models.AccessTokenRecord
	expiresAt time.Time
}

// NewAccessTokenService constructs the service.
func NewAccessTokenService(cfg *models.Config, db AccessTokenStore) *AccessTokenService {
	return &AccessTokenService{cfg: cfg, db: db}
}

// ParseRawToken splits apt_<issuer>_<tokenId>_<secret>.
func ParseRawToken(raw string) (issuerID, tokenID, secret string, err error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, models.AccessTokenPrefix) {
		return "", "", "", errTokenInvalid
	}
	rest := strings.TrimPrefix(raw, models.AccessTokenPrefix)
	parts := strings.SplitN(rest, "_", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", errTokenInvalid
	}
	return parts[0], parts[1], parts[2], nil
}

// IsAccessToken reports apt_ prefix.
func IsAccessToken(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), models.AccessTokenPrefix)
}

// IsRetiredSyncTokenPrefix reports legacy cli-/sdk-/mcp- tokens.
func IsRetiredSyncTokenPrefix(raw string) bool {
	r := strings.TrimSpace(raw)
	return strings.HasPrefix(r, "cli-") || strings.HasPrefix(r, "sdk-") || strings.HasPrefix(r, "mcp-")
}

// IsAdministrableRole returns true for roles that may administer a project for token grants.
func IsAdministrableRole(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	return r == "admin" || r == "owner" || r == "project_admin"
}

// Mint creates a new access token for issuerUserID. Returns raw token (show once) and public record.
func (s *AccessTokenService) Mint(ctx context.Context, issuerUserID string, req *models.CreateAccessTokenRequest) (raw string, pub *models.AccessTokenPublic, err error) {
	if s == nil || s.db == nil {
		return "", nil, errors.New("access token service unavailable")
	}
	if req == nil {
		return "", nil, errors.New("request required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", nil, errors.New("name is required")
	}

	caps, err := authz.ResolvePreset(req.Preset, req.Capabilities)
	if err != nil {
		return "", nil, err
	}
	if authz.HasDangerCapability(caps) && !req.AcknowledgeDanger {
		return "", nil, errDangerRequired
	}

	projectMode := strings.TrimSpace(req.ProjectGrantMode)
	if projectMode == "" {
		projectMode = models.AccessTokenProjectGrantSelected
	}
	if projectMode != models.AccessTokenProjectGrantSelected && projectMode != models.AccessTokenProjectGrantAllAdmin {
		return "", nil, errors.New("invalid project_grant_mode")
	}

	adminProjects, err := s.listAdministrableProjectIDs(ctx, issuerUserID)
	if err != nil {
		return "", nil, err
	}
	adminSet := toSet(adminProjects)

	var projectIDs []string
	switch projectMode {
	case models.AccessTokenProjectGrantAllAdmin:
		projectIDs = nil // dynamic
	case models.AccessTokenProjectGrantSelected:
		if len(req.ProjectIDs) == 0 {
			return "", nil, errors.New("project_ids required for selected project grant")
		}
		for _, pid := range req.ProjectIDs {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			if _, ok := adminSet[pid]; !ok {
				return "", nil, fmt.Errorf("project %s is not administrable by issuer", pid)
			}
			projectIDs = append(projectIDs, pid)
		}
		if len(projectIDs) == 0 {
			return "", nil, errors.New("project_ids required for selected project grant")
		}
	}

	tenantMode := strings.TrimSpace(req.TenantGrantMode)
	if tenantMode == "" {
		tenantMode = models.AccessTokenTenantGrantNone
	}
	switch tenantMode {
	case models.AccessTokenTenantGrantAll, models.AccessTokenTenantGrantSelected, models.AccessTokenTenantGrantNone:
	default:
		return "", nil, errors.New("invalid tenant_grant_mode")
	}

	expiresAt, err := parseTokenExpiry(req.Duration, req.Preset)
	if err != nil {
		return "", nil, err
	}

	tokenID := utility.NewID()
	secretBytes := make([]byte, accessTokenSecretBytes)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", nil, err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}

	prefix := secret
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	record := &models.AccessTokenRecord{
		ID:               tokenID,
		SecretHash:       string(hash),
		SecretPrefix:     prefix,
		Name:             name,
		Description:      strings.TrimSpace(req.Description),
		IssuerUserID:     issuerUserID,
		Status:           models.AccessTokenStatusActive,
		Preset:           strings.TrimSpace(req.Preset),
		ExpiresAt:        expiresAt,
		CreatedAt:        utility.GetCurrentTime(),
		ProjectGrantMode: projectMode,
		ProjectIDs:       projectIDs,
		TenantGrantMode:  tenantMode,
		TenantIDs:        cloneTenantIDs(req.TenantIDs),
		Capabilities:     caps,
		AllowedCIDRs:     append([]string(nil), req.AllowedCIDRs...),
	}

	user, err := s.db.GetSystemUser(ctx, issuerUserID)
	if err != nil || user == nil {
		return "", nil, errors.New("issuer user not found")
	}
	if !user.IsActive && user.ID != "" {
		// IsActive may be unset on older rows; only reject explicit false if we track it.
	}
	user.AccessTokens = append(user.AccessTokens, record)
	// Clear legacy sync tokens on any mint so we stop carrying raw secrets.
	user.SyncTokens = nil
	if err := s.db.UpdateSystemUser(ctx, user, true); err != nil {
		return "", nil, err
	}

	raw = formatRawToken(issuerUserID, tokenID, secret)
	s.putCache(record)
	return raw, record.ToPublic(), nil
}

// PublicForPrincipal returns AccessTokenPublic for the calling apt_ principal (self only).
// Prefers in-memory cache from ValidateRaw; falls back to issuer inventory by TokenID.
func (s *AccessTokenService) PublicForPrincipal(ctx context.Context, principal *models.AccessPrincipal) (*models.AccessTokenPublic, error) {
	if principal == nil || strings.TrimSpace(principal.TokenID) == "" {
		return nil, errors.New("missing access principal")
	}
	if rec := s.getCache(principal.TokenID); rec != nil {
		return rec.ToPublic(), nil
	}
	user, err := s.db.GetSystemUser(ctx, principal.IssuerUserID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, t := range user.AccessTokens {
		if t == nil || t.ID != principal.TokenID {
			continue
		}
		pub := t.ToPublic()
		if t.Status == models.AccessTokenStatusActive && isExpired(t.ExpiresAt, now) {
			pub.Status = models.AccessTokenStatusExpired
		}
		return pub, nil
	}
	// Fallback projection from principal (no name/expires enrichment).
	return &models.AccessTokenPublic{
		ID:               principal.TokenID,
		IssuerUserID:     principal.IssuerUserID,
		Status:           models.AccessTokenStatusActive,
		ProjectGrantMode: principal.ProjectGrantMode,
		ProjectIDs:       append([]string(nil), principal.ProjectIDs...),
		TenantGrantMode:  principal.TenantGrantMode,
		TenantIDs:        cloneTenantIDs(principal.TenantIDs),
		Capabilities:     append([]string(nil), principal.Capabilities...),
		AllowedCIDRs:     append([]string(nil), principal.AllowedCIDRs...),
		CapabilityCount:  len(principal.Capabilities),
	}, nil
}

// List returns public tokens for issuer.
func (s *AccessTokenService) List(ctx context.Context, issuerUserID string) ([]*models.AccessTokenPublic, error) {
	user, err := s.db.GetSystemUser(ctx, issuerUserID)
	if err != nil {
		return nil, err
	}
	out := make([]*models.AccessTokenPublic, 0, len(user.AccessTokens))
	now := time.Now().UTC()
	for _, t := range user.AccessTokens {
		if t == nil {
			continue
		}
		pub := t.ToPublic()
		if t.Status == models.AccessTokenStatusActive && isExpired(t.ExpiresAt, now) {
			pub.Status = models.AccessTokenStatusExpired
		}
		out = append(out, pub)
	}
	return out, nil
}

// Revoke marks a token revoked by id for the issuer.
func (s *AccessTokenService) Revoke(ctx context.Context, issuerUserID, tokenID, revokedBy string) error {
	user, err := s.db.GetSystemUser(ctx, issuerUserID)
	if err != nil {
		return err
	}
	found := false
	for _, t := range user.AccessTokens {
		if t == nil || t.ID != tokenID {
			continue
		}
		t.Status = models.AccessTokenStatusRevoked
		t.RevokedAt = utility.GetCurrentTime()
		t.RevokedBy = revokedBy
		found = true
		s.evictCache(tokenID)
		break
	}
	if !found {
		return errors.New("token not found")
	}
	return s.db.UpdateSystemUser(ctx, user, true)
}

// RevokeByRaw validates ownership then revokes.
func (s *AccessTokenService) RevokeByRaw(ctx context.Context, issuerUserID, raw string) error {
	rec, err := s.loadAndVerify(ctx, raw)
	if err != nil {
		return err
	}
	if rec.IssuerUserID != issuerUserID {
		return errors.New("token does not belong to current user")
	}
	return s.Revoke(ctx, issuerUserID, rec.ID, issuerUserID)
}

// Rotate revokes the old token and mints a replacement with the same grants.
func (s *AccessTokenService) Rotate(ctx context.Context, issuerUserID, tokenID string) (raw string, pub *models.AccessTokenPublic, err error) {
	user, err := s.db.GetSystemUser(ctx, issuerUserID)
	if err != nil {
		return "", nil, err
	}
	var existing *models.AccessTokenRecord
	for _, t := range user.AccessTokens {
		if t != nil && t.ID == tokenID {
			existing = t
			break
		}
	}
	if existing == nil {
		return "", nil, errors.New("token not found")
	}
	if existing.Status != models.AccessTokenStatusActive {
		return "", nil, errors.New("only active tokens can be rotated")
	}
	if err := s.Revoke(ctx, issuerUserID, tokenID, issuerUserID); err != nil {
		return "", nil, err
	}
	req := &models.CreateAccessTokenRequest{
		Name:              existing.Name,
		Description:       existing.Description,
		Duration:          existing.ExpiresAt,
		Preset:            existing.Preset,
		ProjectGrantMode:  existing.ProjectGrantMode,
		ProjectIDs:        existing.ProjectIDs,
		TenantGrantMode:   existing.TenantGrantMode,
		TenantIDs:         existing.TenantIDs,
		Capabilities:      existing.Capabilities,
		AllowedCIDRs:      existing.AllowedCIDRs,
		AcknowledgeDanger: true,
	}
	// Preserve absolute expiry date string if already ISO/date.
	return s.Mint(ctx, issuerUserID, req)
}

// ValidateRaw verifies apt_ token and returns TokenClaims + AccessPrincipal.
func (s *AccessTokenService) ValidateRaw(ctx context.Context, raw string, clientIP, userAgent string) (*models.TokenClaims, *models.AccessPrincipal, error) {
	if IsRetiredSyncTokenPrefix(raw) {
		return nil, nil, errTokenFormat
	}
	rec, err := s.loadAndVerify(ctx, raw)
	if err != nil {
		return nil, nil, err
	}
	if err := checkCIDRAllow(rec.AllowedCIDRs, clientIP); err != nil {
		return nil, nil, err
	}

	issuer, err := s.db.GetSystemUser(ctx, rec.IssuerUserID)
	if err != nil || issuer == nil {
		return nil, nil, errTokenInvalid
	}

	_ = s.touchLastUsed(ctx, rec, clientIP, userAgent)

	principal := &models.AccessPrincipal{
		TokenID:          rec.ID,
		IssuerUserID:     rec.IssuerUserID,
		ProjectGrantMode: rec.ProjectGrantMode,
		ProjectIDs:       append([]string(nil), rec.ProjectIDs...),
		TenantGrantMode:  rec.TenantGrantMode,
		TenantIDs:        cloneTenantIDs(rec.TenantIDs),
		Capabilities:     append([]string(nil), rec.Capabilities...),
		AllowedCIDRs:     append([]string(nil), rec.AllowedCIDRs...),
		TokenType:        "access_token",
	}

	claims := &models.TokenClaims{
		UserID:        rec.IssuerUserID,
		Email:         issuer.Email,
		// Role "admin" is historical synthetic context for apt_ tokens so BuildSystemParam
		// can resolve a system role. System GraphQL ACL for project data MUST use
		// AccessPrincipal capabilities (RequireCapability), NOT Role.IsAdmin from this claim.
		// Narrowing this to "" / "access_token" is deferred — it would change BuildSystemParam
		// IsAdmin behavior for many resolvers still assuming admin today.
		Role:          "admin",
		TokenType:     "access_token",
		TokenUniqueID: rec.ID,
		ProjectIDs:    append([]string(nil), rec.ProjectIDs...),
		Scopes:        append([]string(nil), rec.Capabilities...),
		ExpireAt:      expiryUnix(rec.ExpiresAt),
	}
	return claims, principal, nil
}

// AuthorizeProject checks project grant ∩ issuer administrable role for requested project.
func (s *AccessTokenService) AuthorizeProject(ctx context.Context, principal *models.AccessPrincipal, projectID string) error {
	if principal == nil {
		return errors.New("missing access principal")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil // no project context yet
	}
	ok, err := s.issuerCanAdminister(ctx, principal.IssuerUserID, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return errProjectNotAllowed
	}
	switch principal.ProjectGrantMode {
	case models.AccessTokenProjectGrantAllAdmin:
		return nil
	case models.AccessTokenProjectGrantSelected:
		for _, id := range principal.ProjectIDs {
			if id == projectID {
				return nil
			}
		}
		return errProjectNotAllowed
	default:
		return errProjectNotAllowed
	}
}

// AuthorizeTenant checks optional tenant grant for project.
func (s *AccessTokenService) AuthorizeTenant(principal *models.AccessPrincipal, projectID, tenantID string) error {
	if principal == nil {
		return errors.New("missing access principal")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil
	}
	switch principal.TenantGrantMode {
	case models.AccessTokenTenantGrantNone, "":
		return errors.New("token has no tenant grant")
	case models.AccessTokenTenantGrantAll:
		return nil
	case models.AccessTokenTenantGrantSelected:
		ids := principal.TenantIDs[projectID]
		for _, id := range ids {
			if id == tenantID {
				return nil
			}
		}
		return errors.New("tenant not in token grant")
	default:
		return errors.New("invalid tenant grant mode")
	}
}

func (s *AccessTokenService) loadAndVerify(ctx context.Context, raw string) (*models.AccessTokenRecord, error) {
	issuerID, tokenID, secret, err := ParseRawToken(raw)
	if err != nil {
		return nil, err
	}
	if rec := s.getCache(tokenID); rec != nil {
		if rec.IssuerUserID != issuerID {
			return nil, errTokenInvalid
		}
		if err := verifyRecord(rec, secret); err != nil {
			return nil, err
		}
		return rec, nil
	}
	user, err := s.db.GetSystemUser(ctx, issuerID)
	if err != nil || user == nil {
		return nil, errTokenInvalid
	}
	var found *models.AccessTokenRecord
	for _, t := range user.AccessTokens {
		if t != nil && t.ID == tokenID {
			found = t
			break
		}
	}
	if found == nil {
		return nil, errTokenInvalid
	}
	if err := verifyRecord(found, secret); err != nil {
		return nil, err
	}
	s.putCache(found)
	return found, nil
}

func verifyRecord(rec *models.AccessTokenRecord, secret string) error {
	if rec.Status == models.AccessTokenStatusRevoked {
		return errTokenRevoked
	}
	if isExpired(rec.ExpiresAt, time.Now().UTC()) {
		return errTokenInvalid
	}
	if err := bcrypt.CompareHashAndPassword([]byte(rec.SecretHash), []byte(secret)); err != nil {
		return errTokenInvalid
	}
	return nil
}

func (s *AccessTokenService) touchLastUsed(ctx context.Context, rec *models.AccessTokenRecord, ip, ua string) error {
	user, err := s.db.GetSystemUser(ctx, rec.IssuerUserID)
	if err != nil {
		return err
	}
	now := utility.GetCurrentTime()
	for _, t := range user.AccessTokens {
		if t != nil && t.ID == rec.ID {
			t.LastUsedAt = now
			t.LastUsedIP = ip
			t.LastUsedUA = ua
			rec.LastUsedAt = now
			rec.LastUsedIP = ip
			rec.LastUsedUA = ua
			break
		}
	}
	s.putCache(rec)
	// Best-effort; do not fail the request on last-used write errors.
	_ = s.db.UpdateSystemUser(ctx, user, true)
	return nil
}

func (s *AccessTokenService) listAdministrableProjectIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.FindUserProjectsWithRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, row := range rows {
		if row == nil || row.Project == nil {
			continue
		}
		if IsAdministrableRole(row.Role) {
			ids = append(ids, row.Project.ID)
		}
	}
	return ids, nil
}

func (s *AccessTokenService) issuerCanAdminister(ctx context.Context, userID, projectID string) (bool, error) {
	pwr, err := s.db.CheckProjectWithRoles(ctx, userID, projectID)
	if err != nil || pwr == nil {
		return false, nil
	}
	return IsAdministrableRole(pwr.Role), nil
}

func (s *AccessTokenService) putCache(rec *models.AccessTokenRecord) {
	if rec == nil {
		return
	}
	cp := *rec
	s.cache.Store(rec.ID, &cachedToken{record: &cp, expiresAt: time.Now().Add(accessTokenCacheTTL)})
}

func (s *AccessTokenService) getCache(tokenID string) *models.AccessTokenRecord {
	v, ok := s.cache.Load(tokenID)
	if !ok {
		return nil
	}
	ct := v.(*cachedToken)
	if time.Now().After(ct.expiresAt) {
		s.cache.Delete(tokenID)
		return nil
	}
	if ct.record.Status == models.AccessTokenStatusRevoked {
		s.cache.Delete(tokenID)
		return nil
	}
	cp := *ct.record
	return &cp
}

func (s *AccessTokenService) evictCache(tokenID string) {
	s.cache.Delete(tokenID)
}

func formatRawToken(issuerID, tokenID, secret string) string {
	return models.AccessTokenPrefix + issuerID + "_" + tokenID + "_" + secret
}

func parseTokenExpiry(duration, preset string) (string, error) {
	duration = strings.TrimSpace(duration)
	if duration == "" {
		days := 90
		for _, p := range authz.Presets() {
			if p.ID == strings.ToLower(strings.TrimSpace(preset)) && p.DefaultDays > 0 {
				days = p.DefaultDays
				break
			}
		}
		return time.Now().UTC().AddDate(0, 0, days).Format("2006-01-02"), nil
	}
	if duration == "never" {
		return time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC).Format(time.RFC3339), nil
	}
	if t, err := time.Parse("2006-01-02", duration); err == nil {
		return t.UTC().Format("2006-01-02"), nil
	}
	if t, err := time.Parse(time.RFC3339, duration); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", errors.New("invalid duration: use YYYY-MM-DD, RFC3339, never, or empty for default")
}

func isExpired(expiresAt string, now time.Time) bool {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return false
	}
	if t, err := time.Parse("2006-01-02", expiresAt); err == nil {
		// end of day UTC
		end := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
		return now.After(end)
	}
	if t, err := time.Parse(time.RFC3339, expiresAt); err == nil {
		return now.After(t.UTC())
	}
	return false
}

func expiryUnix(expiresAt string) int64 {
	expiresAt = strings.TrimSpace(expiresAt)
	if t, err := time.Parse("2006-01-02", expiresAt); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC).Unix()
	}
	if t, err := time.Parse(time.RFC3339, expiresAt); err == nil {
		return t.Unix()
	}
	return time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC).Unix()
}

func toSet(ids []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

func cloneTenantIDs(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func checkCIDRAllow(cidrs []string, ipStr string) error {
	if len(cidrs) == 0 {
		return nil
	}
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return errors.New("client IP required for CIDR-restricted token")
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		// echo may give "ip:port"
		host, _, err := net.SplitHostPort(ipStr)
		if err == nil {
			ip = net.ParseIP(host)
		}
	}
	if ip == nil {
		return errors.New("invalid client IP")
	}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if strings.Contains(c, "/") {
			_, network, err := net.ParseCIDR(c)
			if err == nil && network.Contains(ip) {
				return nil
			}
			continue
		}
		if pip := net.ParseIP(c); pip != nil && pip.Equal(ip) {
			return nil
		}
	}
	return errors.New("client IP not allowed by token CIDR policy")
}
