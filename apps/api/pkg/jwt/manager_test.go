package jwt

import (
	"testing"
	"time"
)

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	mgr := NewManager("test-secret", time.Hour)

	token, jti, expiresAt, err := mgr.Issue("user-123", "user", []string{"ADMIN"})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token == "" || jti == "" {
		t.Fatal("Issue() returned an empty token or jti")
	}
	if !expiresAt.After(time.Now()) {
		t.Error("Issue() expiresAt should be in the future")
	}

	claims, err := mgr.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("claims.Subject = %q, want %q", claims.Subject, "user-123")
	}
	if claims.SubjectType != "user" {
		t.Errorf("claims.SubjectType = %q, want %q", claims.SubjectType, "user")
	}
	if claims.ID != jti {
		t.Errorf("claims.ID = %q, want %q (the jti Issue returned)", claims.ID, jti)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "ADMIN" {
		t.Errorf("claims.Roles = %v, want [ADMIN]", claims.Roles)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	issuer := NewManager("secret-a", time.Hour)
	verifier := NewManager("secret-b", time.Hour)

	token, _, _, err := issuer.Issue("user-123", "user", nil)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := verifier.Verify(token); err == nil {
		t.Error("Verify() with the wrong secret should fail, got nil error")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	mgr := NewManager("test-secret", -time.Minute) // already expired

	token, _, _, err := mgr.Issue("user-123", "customer", nil)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := mgr.Verify(token); err == nil {
		t.Error("Verify() on an expired token should fail, got nil error")
	}
}

func TestIssueGeneratesUniqueJTIs(t *testing.T) {
	mgr := NewManager("test-secret", time.Hour)

	_, jti1, _, _ := mgr.Issue("user-1", "user", nil)
	_, jti2, _, _ := mgr.Issue("user-1", "user", nil)

	if jti1 == jti2 {
		t.Error("two calls to Issue() should not produce the same jti — rotation/revocation relies on uniqueness")
	}
}
