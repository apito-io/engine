package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestNewEmailSender_CloudflareEmptyCredsFallsToNoop(t *testing.T) {
	s := NewEmailSender(&models.Config{EmailProvider: "cloudflare"})
	_, ok := s.(noopEmailSender)
	require.True(t, ok)
}

func TestNewEmailSender_ResendEmptyKeyFallsToNoop(t *testing.T) {
	s := NewEmailSender(&models.Config{EmailProvider: "resend"})
	_, ok := s.(noopEmailSender)
	require.True(t, ok)
}

func TestNewEmailSender_ExplicitNoop(t *testing.T) {
	s := NewEmailSender(&models.Config{EmailProvider: "noop", CloudflareAPIToken: "x", CloudflareAccountID: "y"})
	_, ok := s.(noopEmailSender)
	require.True(t, ok)
}

func TestNewEmailSender_Cloudflare(t *testing.T) {
	s := NewEmailSender(&models.Config{
		EmailProvider:       "cloudflare",
		CloudflareAPIToken:  "tok",
		CloudflareAccountID: "acc",
		EmailFrom:           "no-reply@apito.io",
		EmailFromName:       "Apito",
	})
	_, ok := s.(*cloudflareEmailSender)
	require.True(t, ok)
}

func TestCloudflareEmailSender_SendPayload(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"errors":[],"result":{"delivered":["a@b.com"],"permanent_bounces":[],"queued":[]}}`))
	}))
	t.Cleanup(srv.Close)

	sender := &cloudflareEmailSender{
		cfg: &models.Config{
			EmailFrom:     "no-reply@apito.io",
			EmailFromName: "Apito",
		},
		token:     "secret-token",
		accountID: "acct_123",
		client:    srv.Client(),
		endpoint:  srv.URL,
	}
	err := sender.Send(context.Background(), &models.EmailSendRequest{
		Recipients: []string{"a@b.com"},
		Subject:    "Hello",
		HtmlBody:   "<p>Hi</p>",
		TextBody:   "Hi",
	})
	require.NoError(t, err)
	require.Equal(t, "Bearer secret-token", gotAuth)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(gotBody), &payload))
	from, ok := payload["from"].(map[string]interface{})
	require.True(t, ok, "from must be object")
	require.Equal(t, "no-reply@apito.io", from["address"])
	require.Equal(t, "Apito", from["name"])
	require.Nil(t, from["email"])
}

func TestCloudflareEmailSender_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`))
	}))
	t.Cleanup(srv.Close)
	sender := &cloudflareEmailSender{
		cfg:       &models.Config{EmailFrom: "no-reply@apito.io"},
		token:     "bad",
		accountID: "acct",
		client:    srv.Client(),
		endpoint:  srv.URL,
	}
	err := sender.Send(context.Background(), &models.EmailSendRequest{
		Recipients: []string{"a@b.com"},
		Subject:    "x",
		TextBody:   "y",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestCloudflareEmailSender_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":1000,"message":"Sender domain not verified"}]}`))
	}))
	t.Cleanup(srv.Close)
	sender := &cloudflareEmailSender{
		cfg:       &models.Config{EmailFrom: "no-reply@apito.io"},
		token:     "tok",
		accountID: "acct",
		client:    srv.Client(),
		endpoint:  srv.URL,
	}
	err := sender.Send(context.Background(), &models.EmailSendRequest{
		Recipients: []string{"a@b.com"},
		Subject:    "x",
		TextBody:   "y",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "400")
}

func TestComposeTeamInvite_TempPassword(t *testing.T) {
	req := &models.EmailSendRequest{ProjectName: "Fitness", TempPassword: "abc123"}
	ComposeTeamInvite(req)
	require.Equal(t, "You're invited", req.Subject)
	require.Contains(t, req.TextBody, "abc123")
	require.Contains(t, req.TextBody, "Fitness")
	require.Contains(t, req.HtmlBody, "Fitness")
	require.Contains(t, req.HtmlBody, apitoEmailLogoURL)
	require.NotContains(t, req.HtmlBody, "You're invited to Fitness")
}

func TestComposeTeamInvite_AcceptURL(t *testing.T) {
	req := &models.EmailSendRequest{
		ProjectName: "Fitness",
		AcceptURL:   "http://localhost:4000/invite/accept?token=abc",
	}
	ComposeTeamInvite(req)
	require.Contains(t, req.TextBody, "Accept this invite")
	require.Contains(t, req.HtmlBody, "/invite/accept?token=abc")
}

func TestComposeTeamInvite_MultipleProjects(t *testing.T) {
	req := &models.EmailSendRequest{
		ProjectNames: []string{"Apito Website", "Chikly Newspaper"},
	}
	ComposeTeamInvite(req)
	require.Equal(t, "You're invited", req.Subject)
	require.Contains(t, req.TextBody, "- Apito Website")
	require.Contains(t, req.TextBody, "- Chikly Newspaper")
	require.Contains(t, req.HtmlBody, "<li>Apito Website</li>")
	require.Contains(t, req.HtmlBody, "<li>Chikly Newspaper</li>")
	require.Contains(t, req.HtmlBody, apitoEmailLogoURL)
}

func TestComposePasswordReset(t *testing.T) {
	req := &models.EmailSendRequest{VerificationCode: "123456", AppURL: "http://localhost:4000"}
	ComposePasswordReset(req)
	require.Contains(t, req.Subject, "Reset")
	require.Contains(t, req.TextBody, "123456")
	require.Contains(t, req.HtmlBody, "localhost:4000")
}

type recordingMailer struct {
	last *models.EmailSendRequest
	err  error
}

func (r *recordingMailer) Send(_ context.Context, req *models.EmailSendRequest) error {
	r.last = req
	return r.err
}

type memKV struct {
	data map[string]string
}

func (m *memKV) SetValue(_ context.Context, key string, value string, _ time.Duration) error {
	if m.data == nil {
		m.data = map[string]string{}
	}
	m.data[key] = value
	return nil
}
func (m *memKV) GetValue(_ context.Context, key string) (string, error) {
	v, ok := m.data[key]
	if !ok {
		return "", errKVMissing
	}
	return v, nil
}
func (m *memKV) DelValue(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

var errKVMissing = errString("key not found")

type errString string

func (e errString) Error() string { return string(e) }

func (m *memKV) AddToSortedSets(context.Context, string, string, time.Duration) error {
	return nil
}
func (m *memKV) GetFromSortedSets(context.Context, string, string) (float64, error) { return 0, nil }
func (m *memKV) SetToHashMap(context.Context, string, string, string) error         { return nil }
func (m *memKV) GetFromHashMap(context.Context, string, string) (string, error)     { return "", nil }
func (m *memKV) CheckKeyHashMap(context.Context, string, string) bool               { return false }
func (m *memKV) SetJSONObject(context.Context, string, interface{}, time.Duration) error {
	return nil
}
func (m *memKV) GetJSONObject(context.Context, string) (interface{}, error) { return nil, nil }
func (m *memKV) CheckRedisKey(context.Context, ...string) (bool, error)     { return false, nil }
func (m *memKV) AddToSets(context.Context, string, string) error            { return nil }
func (m *memKV) RemoveSets(context.Context, string, string) error           { return nil }

type memUsers struct {
	byEmail map[string]*models.SystemUser
}

func (m *memUsers) GetSystemUserByEmail(_ context.Context, email string) (*models.SystemUser, error) {
	u, ok := m.byEmail[strings.ToLower(email)]
	if !ok {
		return nil, errString("not found")
	}
	return u, nil
}
func (m *memUsers) UpdateSystemUser(_ context.Context, user *models.SystemUser, _ bool) error {
	m.byEmail[strings.ToLower(user.Email)] = user
	return nil
}

func TestForgetPasswordRequest_UnknownEmailSilent(t *testing.T) {
	mail := &recordingMailer{}
	kv := &memKV{data: map[string]string{}}
	svc, err := NewLocalAuthService(&models.Config{CORSOrigin: "http://localhost:4000"}, nil, kv, mail)
	require.NoError(t, err)
	svc.SetSystemDriver(&memUsers{byEmail: map[string]*models.SystemUser{}})
	err = svc.ForgetPasswordRequest(context.Background(), &models.RegisterRequest{
		User: &models.SystemUser{Email: "nobody@apito.io"},
	})
	require.NoError(t, err)
	require.Nil(t, mail.last)
}

func TestForgetPasswordConfirm_RoundTrip(t *testing.T) {
	mail := &recordingMailer{}
	kv := &memKV{data: map[string]string{}}
	user := &models.SystemUser{ID: "u1", Email: "me@apito.io", Secret: "oldhash"}
	svc, err := NewLocalAuthService(&models.Config{CORSOrigin: "http://localhost:4000"}, nil, kv, mail)
	require.NoError(t, err)
	svc.SetSystemDriver(&memUsers{byEmail: map[string]*models.SystemUser{"me@apito.io": user}})

	err = svc.ForgetPasswordRequest(context.Background(), &models.RegisterRequest{
		User: &models.SystemUser{Email: "me@apito.io"},
	})
	require.NoError(t, err)
	require.NotNil(t, mail.last)
	require.Len(t, mail.last.VerificationCode, 6)

	err = svc.ConfirmForgetPassword(context.Background(), &models.RegisterRequest{
		User:             &models.SystemUser{Email: "me@apito.io", Secret: "newpass1"},
		VerificationCode: "000000",
	})
	require.Error(t, err)

	err = svc.ConfirmForgetPassword(context.Background(), &models.RegisterRequest{
		User:             &models.SystemUser{Email: "me@apito.io", Secret: "newpass1"},
		VerificationCode: mail.last.VerificationCode,
	})
	require.NoError(t, err)
	require.NotEqual(t, "oldhash", user.Secret)
	_, kvErr := kv.GetValue(context.Background(), passwordResetKVKey("me@apito.io"))
	require.Error(t, kvErr)
}
