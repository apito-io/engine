package models

import (
	"strings"
	"testing"
)

func TestRegistrationDefaultRole(t *testing.T) {
	t.Run("unset returns none", func(t *testing.T) {
		if got := RegistrationDefaultRole(&Project{}); got != "none" {
			t.Fatalf("got %q want none", got)
		}
	})
	t.Run("configured role", func(t *testing.T) {
		p := &Project{
			AuthenticationSettings: &AuthenticationSettings{
				DefaultRegistrationRole: "customer",
			},
		}
		if got := RegistrationDefaultRole(p); got != "customer" {
			t.Fatalf("got %q want customer", got)
		}
	})
}

func TestValidateRegistrationDefaultRole(t *testing.T) {
	project := &Project{
		Roles: map[string]*Role{
			"admin":    {IsAdmin: true},
			"customer": {},
		},
	}

	if err := ValidateRegistrationDefaultRole(project, ""); err != nil {
		t.Fatalf("empty role: %v", err)
	}
	if err := ValidateRegistrationDefaultRole(project, "customer"); err != nil {
		t.Fatalf("valid role: %v", err)
	}
	if err := ValidateRegistrationDefaultRole(project, "missing"); err == nil {
		t.Fatal("expected error for missing role")
	}
	if err := ValidateRegistrationDefaultRole(project, "admin"); err == nil {
		t.Fatal("expected error for admin role")
	}
	if err := ValidateRegistrationDefaultRole(nil, "customer"); err == nil {
		t.Fatal("expected error when project is nil")
	}
}

func TestApplyUpdateProjectAuthenticationInput_DefaultRegistrationRole(t *testing.T) {
	project := &Project{
		Roles: map[string]*Role{
			"customer": {},
		},
		AuthenticationSettings: &AuthenticationSettings{
			DefaultRegistrationRole: "customer",
		},
	}

	next, err := ApplyUpdateProjectAuthenticationInput(project, map[string]interface{}{
		"default_registration_role": nil,
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if next.DefaultRegistrationRole != "" {
		t.Fatalf("got %q want empty", next.DefaultRegistrationRole)
	}

	next, err = ApplyUpdateProjectAuthenticationInput(project, map[string]interface{}{
		"default_registration_role": "customer",
	})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if next.DefaultRegistrationRole != "customer" {
		t.Fatalf("got %q want customer", next.DefaultRegistrationRole)
	}

	adminProject := &Project{
		Roles: map[string]*Role{
			"admin": {IsAdmin: true},
		},
	}
	_, err = ApplyUpdateProjectAuthenticationInput(adminProject, map[string]interface{}{
		"default_registration_role": "admin",
	})
	if err == nil || !strings.Contains(err.Error(), "admin") {
		t.Fatalf("expected admin rejection, got %v", err)
	}
}
