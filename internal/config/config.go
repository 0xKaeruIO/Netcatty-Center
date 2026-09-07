package config

import (
	"crypto/subtle"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	flag "github.com/spf13/pflag"
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
	HTTPS         bool
	CookieSecure  bool
	AdminUser     string
	AdminPassword string
}

func FromEnv() Config {
	host := getenv("HOST", "0.0.0.0")
	port := getenvInt("PORT", 4780)
	dataDir := getenv("DATA_DIR", "data")
	https := getenvBool("NCC_HTTPS", true)
	cookieSecure := getenvBool("NCC_COOKIE_SECURE", https)
	return Config{
		Host:          host,
		Port:          port,
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "center.sqlite"),
		HTTPS:         https,
		CookieSecure:  cookieSecure,
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
	fs.BoolVar(&cfg.HTTPS, "https", cfg.HTTPS, "serve HTTPS with a self-signed certificate (default true)")
	plainHTTP := false
	fs.BoolVar(&plainHTTP, "plain-http", false, "disable HTTPS and listen with HTTP")
	fs.StringVar(&cfg.AdminUser, "admin-user", cfg.AdminUser, "admin username (not stored)")
	fs.StringVar(&cfg.AdminPassword, "admin-password", cfg.AdminPassword, "admin password (not stored)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if plainHTTP {
		cfg.HTTPS = false
	}
	cfg.DBPath = filepath.Join(cfg.DataDir, "center.sqlite")
	if os.Getenv("NCC_COOKIE_SECURE") == "" {
		cfg.CookieSecure = cfg.HTTPS
	}
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

func (c Config) PublicURL() string {
	scheme := "http"
	if c.HTTPS {
		scheme = "https"
	}
	host := c.Host
	if host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host + ":" + strconv.Itoa(c.Port)
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

func getenvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return fallback
	}
}
