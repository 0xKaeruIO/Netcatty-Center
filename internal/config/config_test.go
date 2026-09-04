package config

import (
	"strings"
	"testing"
)

func TestMatchAdminUsesStartupCredentials(t *testing.T) {
	cfg := Config{AdminUser: "OpsAdmin", AdminPassword: "s3cret-pass"}
	if !cfg.HasBootstrapAdmin() {
		t.Fatal("expected bootstrap admin")
	}
	if !cfg.MatchAdmin("opsadmin", "s3cret-pass") {
		t.Fatal("username should be case-insensitive")
	}
	if cfg.MatchAdmin("opsadmin", "wrong") {
		t.Fatal("password should not match")
	}
	if cfg.MatchAdmin("other", "s3cret-pass") {
		t.Fatal("username should not match")
	}
}

func TestValidateRequiresUserAndPasswordTogether(t *testing.T) {
	if err := (Config{AdminUser: "admin"}).Validate(); err != ErrIncompleteAdminFlags {
		t.Fatalf("err=%v", err)
	}
	if err := (Config{AdminPassword: "x"}).Validate(); err != ErrIncompleteAdminFlags {
		t.Fatalf("err=%v", err)
	}
	if err := (Config{}).Validate(); err != nil {
		t.Fatalf("empty config should be valid: %v", err)
	}
}

func TestLoadReadsAdminFlags(t *testing.T) {
	cfg, err := Load([]string{"-admin-user", "alice", "-admin-password", "pw-123456"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminUser != "alice" || cfg.AdminPassword != "pw-123456" {
		t.Fatalf("cfg=%+v", cfg)
	}
	if _, err := Load([]string{"-admin-user", "alice"}); err == nil || !strings.Contains(err.Error(), "together") {
		t.Fatalf("expected incomplete flags, got %v", err)
	}
}
