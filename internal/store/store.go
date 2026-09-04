package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"netcatty-center/internal/security"

	_ "modernc.org/sqlite"
)

var (
	ErrAdminExists   = errors.New("Admin already exists")
	ErrLabelRequired = errors.New("label is required")
	ErrHostRequired  = errors.New("hostname is required")
	ErrBadPort       = errors.New("port must be an integer between 1 and 65535")
)

const schema = `
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admins (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  admin_id TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  FOREIGN KEY (admin_id) REFERENCES admins(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS api_keys (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,
  key_prefix TEXT NOT NULL,
  plaintext TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  last_used_at INTEGER,
  revoked_at INTEGER
);

CREATE TABLE IF NOT EXISTS hosts (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  hostname TEXT NOT NULL,
  port INTEGER NOT NULL DEFAULT 22,
  username TEXT NOT NULL DEFAULT '',
  group_path TEXT NOT NULL DEFAULT '',
  tags_json TEXT NOT NULL DEFAULT '[]',
  os TEXT NOT NULL DEFAULT 'linux',
  protocol TEXT NOT NULL DEFAULT 'ssh',
  device_type TEXT NOT NULL DEFAULT 'general',
  notes TEXT NOT NULL DEFAULT '',
  password TEXT NOT NULL DEFAULT '',
  private_key TEXT NOT NULL DEFAULT '',
  passphrase TEXT NOT NULL DEFAULT '',
  startup_command TEXT NOT NULL DEFAULT '',
  startup_command_run_mode TEXT NOT NULL DEFAULT '',
  startup_command_rules_json TEXT NOT NULL DEFAULT '[]',
  visibility TEXT NOT NULL DEFAULT 'all',
  visible_key_ids_json TEXT NOT NULL DEFAULT '[]',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_hosts_group ON hosts(group_path);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
`

type Settings struct {
	CenterID   string `json:"centerId"`
	CenterName string `json:"centerName"`
}

type Admin struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"createdAt"`
}

type AdminRow struct {
	Admin
	PasswordHash string
}

type Session struct {
	AdminID   string
	ExpiresAt int64
}

type APIKey struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	KeyPrefix  string `json:"keyPrefix"`
	Plaintext  string `json:"plaintext"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt *int64 `json:"lastUsedAt"`
	RevokedAt  *int64 `json:"revokedAt"`
}

type StartupCommandRule struct {
	Expect string `json:"expect"`
	Send   string `json:"send"`
}

type Host struct {
	ID                    string               `json:"id"`
	Label                 string               `json:"label"`
	Hostname              string               `json:"hostname"`
	Port                  int                  `json:"port"`
	Username              string               `json:"username"`
	Group                 string               `json:"group"`
	Tags                  []string             `json:"tags"`
	OS                    string               `json:"os"`
	Protocol              string               `json:"protocol"`
	DeviceType            string               `json:"deviceType"`
	Notes                 string               `json:"notes"`
	Password              string               `json:"password"`
	PrivateKey            string               `json:"privateKey"`
	Passphrase            string               `json:"passphrase"`
	StartupCommand        string               `json:"startupCommand"`
	StartupCommandRunMode string               `json:"startupCommandRunMode"`
	StartupCommandRules   []StartupCommandRule `json:"startupCommandRules"`
	Visibility            string               `json:"visibility"`
	VisibleKeyIDs         []string             `json:"visibleKeyIds"`
	CreatedAt             int64                `json:"createdAt"`
	UpdatedAt             int64                `json:"updatedAt"`
}

type HostInput struct {
	Label                 string               `json:"label"`
	Hostname              string               `json:"hostname"`
	Port                  int                  `json:"port"`
	Username              string               `json:"username"`
	Group                 string               `json:"group"`
	Tags                  []string             `json:"tags"`
	OS                    string               `json:"os"`
	Protocol              string               `json:"protocol"`
	DeviceType            string               `json:"deviceType"`
	Notes                 string               `json:"notes"`
	Password              string               `json:"password"`
	PrivateKey            string               `json:"privateKey"`
	Passphrase            string               `json:"passphrase"`
	StartupCommand        string               `json:"startupCommand"`
	StartupCommandRunMode string               `json:"startupCommandRunMode"`
	StartupCommandRules   []StartupCommandRule `json:"startupCommandRules"`
	Visibility            string               `json:"visibility"`
	VisibleKeyIDs         []string             `json:"visibleKeyIds"`
}

type CatalogHost struct {
	ID                    string               `json:"id"`
	Label                 string               `json:"label"`
	Hostname              string               `json:"hostname"`
	Port                  int                  `json:"port"`
	Username              string               `json:"username"`
	Group                 string               `json:"group"`
	Tags                  []string             `json:"tags"`
	OS                    string               `json:"os"`
	Protocol              string               `json:"protocol"`
	DeviceType            string               `json:"deviceType"`
	Notes                 string               `json:"notes"`
	Password              string               `json:"password"`
	PrivateKey            string               `json:"privateKey"`
	Passphrase            string               `json:"passphrase"`
	StartupCommand        string               `json:"startupCommand"`
	StartupCommandRunMode string               `json:"startupCommandRunMode"`
	StartupCommandRules   []StartupCommandRule `json:"startupCommandRules"`
	UpdatedAt             int64                `json:"updatedAt"`
}

const hostSelectColumns = `
  id, label, hostname, port, username, group_path, tags_json,
  os, protocol, device_type, notes, password, private_key, passphrase,
  startup_command, startup_command_run_mode, startup_command_rules_json,
  visibility, visible_key_ids_json,
  created_at, updated_at`

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, err
		}
	}
	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if dbPath == ":memory:" {
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	store := &Store{db: db}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.ensureCenterIdentity(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Settings() (Settings, error) {
	centerID, err := s.getSetting("center_id")
	if err != nil {
		return Settings{}, err
	}
	centerName, err := s.getSetting("center_name")
	if err != nil {
		return Settings{}, err
	}
	if centerName == "" {
		centerName = "Netcatty Center"
	}
	return Settings{CenterID: centerID, CenterName: centerName}, nil
}

func (s *Store) SetCenterName(name string) (Settings, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "Netcatty Center"
	}
	if err := s.setSetting("center_name", trimmed); err != nil {
		return Settings{}, err
	}
	return s.Settings()
}

func (s *Store) NeedsSetup() (bool, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM admins").Scan(&n); err != nil {
		return false, err
	}
	return n == 0, nil
}

func (s *Store) CreateFirstAdmin(username, password string) (Admin, error) {
	needs, err := s.NeedsSetup()
	if err != nil {
		return Admin{}, err
	}
	if !needs {
		return Admin{}, ErrAdminExists
	}
	return s.createAdmin(username, password)
}

func (s *Store) FindAdminByUsername(username string) (*AdminRow, error) {
	row := s.db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM admins WHERE username = ?",
		strings.TrimSpace(username),
	)
	var admin AdminRow
	if err := row.Scan(&admin.ID, &admin.Username, &admin.PasswordHash, &admin.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

func (s *Store) EnsureAdminAnchor(id string) error {
	existing, err := s.GetAdmin(id)
	if err != nil || existing != nil {
		return err
	}
	_, err = s.db.Exec(
		"INSERT INTO admins (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)",
		id, "ncc-bootstrap-anchor", "!", time.Now().UnixMilli(),
	)
	return err
}

func (s *Store) GetAdmin(id string) (*Admin, error) {
	row := s.db.QueryRow("SELECT id, username, created_at FROM admins WHERE id = ?", id)
	var admin Admin
	if err := row.Scan(&admin.ID, &admin.Username, &admin.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

func (s *Store) CreateSession(adminID, tokenHash string, expiresAt int64) error {
	_, err := s.db.Exec(
		"INSERT INTO sessions (token_hash, admin_id, expires_at) VALUES (?, ?, ?)",
		tokenHash, adminID, expiresAt,
	)
	return err
}

func (s *Store) FindSession(tokenHash string) (*Session, error) {
	row := s.db.QueryRow("SELECT admin_id, expires_at FROM sessions WHERE token_hash = ?", tokenHash)
	var session Session
	if err := row.Scan(&session.AdminID, &session.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}

func (s *Store) PurgeExpiredSessions() {
	_, _ = s.db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().UnixMilli())
}

func (s *Store) ListHosts() ([]Host, error) {
	rows, err := s.db.Query("SELECT" + hostSelectColumns + " FROM hosts ORDER BY group_path ASC, label ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hosts := make([]Host, 0)
	for rows.Next() {
		host, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, rows.Err()
}

func (s *Store) GetHost(id string) (*Host, error) {
	row := s.db.QueryRow("SELECT"+hostSelectColumns+" FROM hosts WHERE id = ?", id)
	host, err := scanHost(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &host, nil
}

func (s *Store) CreateHost(input HostInput) (Host, error) {
	now := time.Now().UnixMilli()
	host, err := hostFromInput(security.RandomID(), input, now, now)
	if err != nil {
		return Host{}, err
	}
	_, err = s.db.Exec(`
      INSERT INTO hosts (
        id, label, hostname, port, username, group_path, tags_json,
        os, protocol, device_type, notes, password, private_key, passphrase,
        startup_command, startup_command_run_mode, startup_command_rules_json,
        visibility, visible_key_ids_json,
        created_at, updated_at
      ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		host.ID, host.Label, host.Hostname, host.Port, host.Username, host.Group,
		mustJSON(host.Tags), host.OS, host.Protocol, host.DeviceType, host.Notes,
		host.Password, host.PrivateKey, host.Passphrase,
		host.StartupCommand, host.StartupCommandRunMode, mustJSON(host.StartupCommandRules),
		host.Visibility, mustJSON(host.VisibleKeyIDs),
		host.CreatedAt, host.UpdatedAt,
	)
	return host, err
}

func (s *Store) UpdateHost(id string, input HostInput) (*Host, error) {
	existing, err := s.GetHost(id)
	if err != nil || existing == nil {
		return existing, err
	}
	host, err := hostFromInput(id, input, existing.CreatedAt, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`
      UPDATE hosts SET
        label = ?, hostname = ?, port = ?, username = ?, group_path = ?,
        tags_json = ?, os = ?, protocol = ?, device_type = ?, notes = ?,
        password = ?, private_key = ?, passphrase = ?,
        startup_command = ?, startup_command_run_mode = ?, startup_command_rules_json = ?,
        visibility = ?, visible_key_ids_json = ?,
        updated_at = ?
      WHERE id = ?`,
		host.Label, host.Hostname, host.Port, host.Username, host.Group,
		mustJSON(host.Tags), host.OS, host.Protocol, host.DeviceType, host.Notes,
		host.Password, host.PrivateKey, host.Passphrase,
		host.StartupCommand, host.StartupCommandRunMode, mustJSON(host.StartupCommandRules),
		host.Visibility, mustJSON(host.VisibleKeyIDs),
		host.UpdatedAt, id,
	)
	if err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *Store) DeleteHost(id string) (bool, error) {
	result, err := s.db.Exec("DELETE FROM hosts WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (s *Store) CreateAPIKey(name, keyHash, keyPrefix, plaintext string) (APIKey, error) {
	now := time.Now().UnixMilli()
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "client"
	}
	key := APIKey{
		ID:        security.RandomID(),
		Name:      trimmed,
		KeyPrefix: keyPrefix,
		Plaintext: plaintext,
		CreatedAt: now,
	}
	_, err := s.db.Exec(`
      INSERT INTO api_keys (id, name, key_hash, key_prefix, plaintext, created_at, last_used_at, revoked_at)
      VALUES (?, ?, ?, ?, ?, ?, NULL, NULL)`,
		key.ID, key.Name, keyHash, keyPrefix, plaintext, now,
	)
	return key, err
}

func (s *Store) ListAPIKeys() ([]APIKey, error) {
	rows, err := s.db.Query(`
      SELECT id, name, key_prefix, plaintext, created_at, last_used_at, revoked_at
      FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		var lastUsed, revoked sql.NullInt64
		if err := rows.Scan(&key.ID, &key.Name, &key.KeyPrefix, &key.Plaintext, &key.CreatedAt, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		key.LastUsedAt = nullInt(lastUsed)
		key.RevokedAt = nullInt(revoked)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) FindActiveAPIKeyByHash(keyHash string) (id string, name string, ok bool, err error) {
	row := s.db.QueryRow(`
      SELECT id, name FROM api_keys WHERE key_hash = ? AND revoked_at IS NULL`, keyHash)
	if err := row.Scan(&id, &name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return id, name, true, nil
}

func (s *Store) TouchAPIKey(id string) error {
	_, err := s.db.Exec("UPDATE api_keys SET last_used_at = ? WHERE id = ?", time.Now().UnixMilli(), id)
	return err
}

func (s *Store) RevokeAPIKey(id string) (bool, error) {
	result, err := s.db.Exec(
		"UPDATE api_keys SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL",
		time.Now().UnixMilli(), id,
	)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (s *Store) RestoreAPIKey(id string) (bool, error) {
	result, err := s.db.Exec(
		"UPDATE api_keys SET revoked_at = NULL WHERE id = ? AND revoked_at IS NOT NULL",
		id,
	)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (s *Store) DeleteAPIKey(id string) (bool, error) {
	result, err := s.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return false, nil
	}
	return true, s.removeAPIKeyFromHostVisibility(id)
}

func (s *Store) removeAPIKeyFromHostVisibility(keyID string) error {
	hosts, err := s.ListHosts()
	if err != nil {
		return err
	}
	for _, host := range hosts {
		next := make([]string, 0, len(host.VisibleKeyIDs))
		changed := false
		for _, id := range host.VisibleKeyIDs {
			if id == keyID {
				changed = true
				continue
			}
			next = append(next, id)
		}
		if !changed {
			continue
		}
		if _, err := s.db.Exec(
			"UPDATE hosts SET visible_key_ids_json = ? WHERE id = ?",
			mustJSON(next), host.ID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) createAdmin(username, password string) (Admin, error) {
	hash, err := security.HashPassword(password)
	if err != nil {
		return Admin{}, err
	}
	admin := Admin{
		ID:        security.RandomID(),
		Username:  strings.TrimSpace(username),
		CreatedAt: time.Now().UnixMilli(),
	}
	_, err = s.db.Exec(
		"INSERT INTO admins (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)",
		admin.ID, admin.Username, hash, admin.CreatedAt,
	)
	return admin, err
}

func (s *Store) migrate() error {
	if err := s.ensureColumn("api_keys", "plaintext", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn("hosts", "password", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn("hosts", "private_key", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn("hosts", "passphrase", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn("hosts", "startup_command", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn("hosts", "startup_command_run_mode", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn("hosts", "startup_command_rules_json", `TEXT NOT NULL DEFAULT '[]'`); err != nil {
		return err
	}
	if err := s.ensureColumn("hosts", "visibility", `TEXT NOT NULL DEFAULT 'all'`); err != nil {
		return err
	}
	return s.ensureColumn("hosts", "visible_key_ids_json", `TEXT NOT NULL DEFAULT '[]'`)
}

func (s *Store) ensureColumn(table, name, ddl string) error {
	var count int
	query := "SELECT COUNT(*) FROM pragma_table_info('" + table + "') WHERE name = ?"
	if err := s.db.QueryRow(query, name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + name + " " + ddl)
	return err
}

func (s *Store) ensureCenterIdentity() error {
	if value, err := s.getSetting("center_id"); err != nil {
		return err
	} else if value == "" {
		if err := s.setSetting("center_id", security.RandomID()); err != nil {
			return err
		}
	}
	if value, err := s.getSetting("center_name"); err != nil {
		return err
	} else if value == "" {
		return s.setSetting("center_name", "Netcatty Center")
	}
	return nil
}

func (s *Store) getSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *Store) setSetting(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

func ToCatalogHost(host Host) CatalogHost {
	command := host.StartupCommand
	if host.StartupCommandRunMode == "rules" {
		command = ""
	}
	rules := host.StartupCommandRules
	if rules == nil {
		rules = []StartupCommandRule{}
	}
	return CatalogHost{
		ID:                    host.ID,
		Label:                 host.Label,
		Hostname:              host.Hostname,
		Port:                  host.Port,
		Username:              host.Username,
		Group:                 host.Group,
		Tags:                  host.Tags,
		OS:                    host.OS,
		Protocol:              host.Protocol,
		DeviceType:            host.DeviceType,
		Notes:                 host.Notes,
		Password:              host.Password,
		PrivateKey:            host.PrivateKey,
		Passphrase:            host.Passphrase,
		StartupCommand:        command,
		StartupCommandRunMode: host.StartupCommandRunMode,
		StartupCommandRules:   rules,
		UpdatedAt:             host.UpdatedAt,
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHost(row rowScanner) (Host, error) {
	var host Host
	var tagsJSON string
	var rulesJSON string
	var visibleKeysJSON string
	err := row.Scan(
		&host.ID, &host.Label, &host.Hostname, &host.Port, &host.Username, &host.Group,
		&tagsJSON, &host.OS, &host.Protocol, &host.DeviceType, &host.Notes,
		&host.Password, &host.PrivateKey, &host.Passphrase,
		&host.StartupCommand, &host.StartupCommandRunMode, &rulesJSON,
		&host.Visibility, &visibleKeysJSON,
		&host.CreatedAt, &host.UpdatedAt,
	)
	if err != nil {
		return Host{}, err
	}
	host.Tags = parseStoredTags(tagsJSON)
	if host.Tags == nil {
		host.Tags = []string{}
	}
	host.StartupCommandRules = parseStoredStartupRules(rulesJSON)
	host.Visibility = asVisibility(host.Visibility)
	host.VisibleKeyIDs = parseStoredIDs(visibleKeysJSON)
	return host, nil
}

func hostFromInput(id string, input HostInput, createdAt, updatedAt int64) (Host, error) {
	label := strings.TrimSpace(input.Label)
	hostname := strings.TrimSpace(input.Hostname)
	if label == "" {
		return Host{}, ErrLabelRequired
	}
	if hostname == "" {
		return Host{}, ErrHostRequired
	}
	port := input.Port
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		return Host{}, ErrBadPort
	}
	runMode := asStartupRunMode(input.StartupCommandRunMode)
	command := input.StartupCommand
	if runMode == "rules" {
		command = ""
	}
	return Host{
		ID:                    id,
		Label:                 label,
		Hostname:              hostname,
		Port:                  port,
		Username:              strings.TrimSpace(input.Username),
		Group:                 strings.TrimSpace(input.Group),
		Tags:                  normalizeTags(input.Tags),
		OS:                    asOS(input.OS),
		Protocol:              asProtocol(input.Protocol),
		DeviceType:            asDeviceType(input.DeviceType),
		Notes:                 strings.TrimSpace(input.Notes),
		Password:              input.Password,
		PrivateKey:            strings.TrimSpace(input.PrivateKey),
		Passphrase:            input.Passphrase,
		StartupCommand:        command,
		StartupCommandRunMode: runMode,
		StartupCommandRules:   normalizeStartupRules(input.StartupCommandRules),
		Visibility:            asVisibility(input.Visibility),
		VisibleKeyIDs:         normalizeIDs(input.VisibleKeyIDs),
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}, nil
}

func (h Host) VisibleToKey(keyID string) bool {
	if asVisibility(h.Visibility) != "keys" {
		return true
	}
	for _, id := range h.VisibleKeyIDs {
		if id == keyID {
			return true
		}
	}
	return false
}

func parseStoredTags(raw string) []string {
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return []string{}
	}
	return normalizeTags(tags)
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func asOS(value string) string {
	switch value {
	case "windows", "macos", "linux":
		return value
	default:
		return "linux"
	}
}

func asProtocol(value string) string {
	if value == "telnet" {
		return "telnet"
	}
	return "ssh"
}

func asDeviceType(value string) string {
	if value == "network" {
		return "network"
	}
	return "general"
}

func asVisibility(value string) string {
	if strings.TrimSpace(value) == "keys" {
		return "keys"
	}
	return "all"
}

func parseStoredIDs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return []string{}
	}
	return normalizeIDs(ids)
}

func normalizeIDs(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func asStartupRunMode(value string) string {
	switch strings.TrimSpace(value) {
	case "paste", "lineDelay", "rules":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func parseStoredStartupRules(raw string) []StartupCommandRule {
	if strings.TrimSpace(raw) == "" {
		return []StartupCommandRule{}
	}
	var rules []StartupCommandRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return []StartupCommandRule{}
	}
	return normalizeStartupRules(rules)
}

func normalizeStartupRules(rules []StartupCommandRule) []StartupCommandRule {
	out := make([]StartupCommandRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, StartupCommandRule{
			Expect: rule.Expect,
			Send:   rule.Send,
		})
	}
	return out
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func nullInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}
