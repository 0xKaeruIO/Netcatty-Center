package config

import (
	"os"
	"path/filepath"
	"strconv"
)

const (
	SessionCookie = "ncc_session"
	SessionTTLMS  = 7 * 24 * 60 * 60 * 1000
)

type Config struct {
	Host         string
	Port         int
	DataDir      string
	DBPath       string
	PublicDir    string
	CookieSecure bool
}

func FromEnv() Config {
	host := getenv("HOST", "0.0.0.0")
	port := getenvInt("PORT", 4780)
	dataDir := getenv("DATA_DIR", "data")
	publicDir := getenv("NCC_PUBLIC_DIR", "public")
	return Config{
		Host:         host,
		Port:         port,
		DataDir:      dataDir,
		DBPath:       filepath.Join(dataDir, "center.sqlite"),
		PublicDir:    publicDir,
		CookieSecure: os.Getenv("NCC_COOKIE_SECURE") == "1",
	}
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
