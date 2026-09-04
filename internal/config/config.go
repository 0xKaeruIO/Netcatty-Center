package config

import (
	"crypto/subtle"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	SessionCookie    = "ncc_session"
	SessionTTLMS     = 7 * 24 * 60 * 60 * 1000
	BootstrapAdminID = "ncc-bootstrap-admin"
)

var ErrIncompleteAdminFlags = errors.New("admin-user and admin-password must be set together")

type Config struct {
	Host          string
	Port          int
	DataDir       string
	DBPath        string
	PublicDir     string
	CookieSecure  bool
	AdminUser     string
	AdminPassword string
}

func FromEnv() Config {
	host := getenv("HOST", "0.0.0.0")
	port := getenvInt("PORT", 4780)
	dataDir := getenv("DATA_DIR", "data")
	publicDir := getenv("NCC_PUBLIC_DIR", "public")
	return Config{
		Host:          host,
		Port:          port,
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "center.sqlite"),
		PublicDir:     publicDir,
		CookieSecure:  os.Getenv("NCC_COOKIE_SECURE") == "1",
		AdminUser:     os.Getenv("NCC_ADMIN_USER"),
		AdminPassword: os.Getenv("NCC_ADMIN_PASSWORD"),
	}
}

func Load(args []string) (Config, error) {
	cfg := FromEnv()
	fs := flag.NewFlagSet("center", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.Host, "host", cfg.Host, "listen host")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "listen port")
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "SQLite data directory")
	fs.StringVar(&cfg.PublicDir, "public-dir", cfg.PublicDir, "admin UI directory")
	fs.StringVar(&cfg.AdminUser, "admin-user", cfg.AdminUser, "admin username (not stored)")
	fs.StringVar(&cfg.AdminPassword, "admin-password", cfg.AdminPassword, "admin password (not stored)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	cfg.DBPath = filepath.Join(cfg.DataDir, "center.sqlite")
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	user := strings.TrimSpace(c.AdminUser)
	if (user == "") != (c.AdminPassword == "") {
		return ErrIncompleteAdminFlags
	}
	return nil
}

func (c Config) HasBootstrapAdmin() bool {
	return strings.TrimSpace(c.AdminUser) != "" && c.AdminPassword != ""
}

func (c Config) MatchAdmin(username, password string) bool {
	if !c.HasBootstrapAdmin() {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(username), strings.TrimSpace(c.AdminUser)) {
		return false
	}
	expected := []byte(c.AdminPassword)
	actual := []byte(password)
	if len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (c Config) Addr() string {
	return c.Host + ":" + strconv.Itoa(c.Port)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
