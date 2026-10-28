package models

import "testing"

func TestRequireRegistrationIdentifier(t *testing.T) {
	t.Run("email mode allows a missing phone", func(t *testing.T) {
		if err := RequireRegistrationIdentifier("email", "ada@example.com", ""); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("email mode requires email", func(t *testing.T) {
		err := RequireRegistrationIdentifier("email", "", "+15551212")
		if err == nil || err.Error() != "email is required" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("unset method is email", func(t *testing.T) {
		err := RequireRegistrationIdentifier(GeneralIdentifierMethod(nil), "", "")
		if err == nil || err.Error() != "email is required" {
			t.Fatalf("got %v", err)
		}
		if err := RequireRegistrationIdentifier(GeneralIdentifierMethod(&Project{}), "ada@example.com", ""); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("phone mode requires phone and allows a missing email", func(t *testing.T) {
		if err := RequireRegistrationIdentifier("phone", "", "+15551212"); err != nil {
			t.Fatal(err)
		}
		err := RequireRegistrationIdentifier("phone", "ada@example.com", "")
		if err == nil || err.Error() != "phone is required" {
			t.Fatalf("got %v", err)
		}
	})
}
