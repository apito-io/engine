package projectauthusers

import (
	"errors"
	"strings"

	"github.com/apito-io/engine/database/system/sqlcommon"
)

func mapAuthUserUniqueViolation(err error) error {
	if err == nil || !sqlcommon.IsSQLUniqueViolation(err) {
		return err
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "idx_users_email"), strings.Contains(msg, ".email"), strings.Contains(msg, " users.email"):
		return errors.New("email already exists for this project")
	case strings.Contains(msg, "idx_users_phone"), strings.Contains(msg, ".phone"), strings.Contains(msg, " users.phone"):
		return errors.New("phone already exists for this project")
	case strings.Contains(msg, "idx_users_google_sub"), strings.Contains(msg, ".google_sub"), strings.Contains(msg, " google_sub"):
		return errors.New("google account already linked to another user")
	case strings.Contains(msg, "idx_users_tenant_username"), strings.Contains(msg, ".username"), strings.Contains(msg, "username"):
		return errors.New("username already exists")
	default:
		return errors.New("user identity already exists")
	}
}
