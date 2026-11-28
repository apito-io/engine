package database

import (
	"context"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestIsStubDriverCredentials(t *testing.T) {
	require.True(t, IsStubDriverCredentials(&models.DriverCredentials{Engine: _const.ApitoDB, Schema: "p_x"}))
	require.True(t, IsStubDriverCredentials(&models.DriverCredentials{}))
	require.False(t, IsStubDriverCredentials(&models.DriverCredentials{Engine: _const.ApitoDB, Host: "h"}))
	require.False(t, IsStubDriverCredentials(&models.DriverCredentials{Engine: _const.SQLiteDriver}))
	require.False(t, IsStubDriverCredentials(&models.DriverCredentials{Database: "libsql://db.turso.io"}))
}

func TestResolveStubCredentialsUsesHook(t *testing.T) {
	called := ""
	cm := &ConnectionManager{cfg: &models.Config{
		ResolveProjectCredentialsHook: func(_ context.Context, pid string, stub *models.DriverCredentials) (*models.DriverCredentials, error) {
			called = pid
			return &models.DriverCredentials{Engine: _const.PostgreSQLDriver, Host: "pg", Schema: stub.Schema}, nil
		},
	}}
	got, err := cm.resolveStubCredentials(context.Background(), "p1", &models.DriverCredentials{Engine: _const.ApitoDB, Schema: "p_p1"})
	require.NoError(t, err)
	require.Equal(t, "p1", called)
	require.Equal(t, "pg", got.Host)
	require.Equal(t, "p_p1", got.Schema)
	require.Equal(t, "p1", got.ProjectID)

	got, err = cm.resolveStubCredentials(context.Background(), "p2", &models.DriverCredentials{Engine: _const.SQLiteDriver, File: "a.sqlite"})
	require.NoError(t, err)
	require.Nil(t, got, "non-stub credentials are not resolved")
}
