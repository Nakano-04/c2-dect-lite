package db

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

// Roles recognised by the mini multi-tenant RBAC.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// DefaultTenantID is the tenant every fresh install starts with.
const DefaultTenantID = 1

var ErrNotFound = errors.New("not found")

type Database struct {
	conn *sql.DB
}

// ---------------------------------------------------------------- models ---

type Operator struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	Role      string `json:"role"`
	TenantID  int64  `json:"tenant_id"`
	CreatedAt string `json:"created_at"`
}

type Tenant struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	AllowImpactModules bool   `json:"allow_impact_modules"`
	CreatedAt          string `json:"created_at"`
}

type Session struct {
	ID           string `json:"id"`
	UUID         string `json:"uuid"`
	Hostname     string `json:"hostname"`
	Username     string `json:"username"`
	InternalIP   string `json:"internal_ip"`
	ExternalIP   string `json:"external_ip"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	PID          int    `json:"pid"`
	Process      string `json:"process"`
	Status       string `json:"status"` // active, sleeping, dead
	LastCheckIn  string `json:"last_checkin"`
	FirstCheckIn string `json:"first_checkin"`
	SleepSec     int    `json:"sleep_sec"`
	Jitter       int    `json:"jitter"`
	Tags         string `json:"tags"`
	TenantID     int64  `json:"tenant_id"`
	AgentVersion string `json:"agent_version"`
	KeyEpoch     int    `json:"key_epoch"`
	ExpiryUnix   int64  `json:"expiry_unix"`
}

type Task struct {
	ID          int64  `json:"id"`
	SessionID   string `json:"session_id"`
	Command     string `json:"command"`
	Args        string `json:"args"`
	Status      string `json:"status"` // pending, sent, completed, error
	Result      string `json:"result"`
	Error       string `json:"error"`
	OperatorID  int64  `json:"operator_id"`
	TenantID    int64  `json:"tenant_id"`
	ChunkTotal  int    `json:"chunk_total"`
	ChunkIndex  int    `json:"chunk_index"`
	CreatedAt   string `json:"created_at"`
	SentAt      string `json:"sent_at"`
	CompletedAt string `json:"completed_at"`
}

type Loot struct {
	ID        int64  `json:"id"`
	SessionID string `json:"session_id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Data      []byte `json:"data"`
	Path      string `json:"path"`
	Hash      string `json:"hash"`
	TenantID  int64  `json:"tenant_id"`
	CreatedAt string `json:"created_at"`
}

type AuditEntry struct {
	ID         int64  `json:"id"`
	TS         string `json:"ts"`
	TenantID   int64  `json:"tenant_id"`
	OperatorID int64  `json:"operator_id"`
	Username   string `json:"username"`
	Category   string `json:"category"`
	Action     string `json:"action"`
	Severity   string `json:"severity"`
	Details    string `json:"details"`
	PrevHash   string `json:"prev_hash"`
	Hash       string `json:"hash"`
}

type AgentToken struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	TenantID  int64  `json:"tenant_id"`
	ExpiresAt string `json:"expires_at"`
	Revoked   bool   `json:"revoked"`
	CreatedAt string `json:"created_at"`
}

type RefreshToken struct {
	JTI        string
	OperatorID int64
	TenantID   int64
	ExpiresAt  time.Time
	Revoked    bool
}

type MarketplaceInstall struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenant_id"`
	PackageID       string `json:"package_id"`
	PackageVersion  string `json:"package_version"`
	Checksum        string `json:"checksum"`
	InstalledAt     string `json:"installed_at"`
	InstalledBy     int64  `json:"installed_by"`
	InstalledByName string `json:"installed_by_name"`
}

// ------------------------------------------------------------ construction ---

func New(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)

	db := &Database{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *Database) Close() error { return db.conn.Close() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ------------------------------------------------------------------ schema ---

func (db *Database) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS tenants (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL,
	description TEXT DEFAULT '',
	allow_impact_modules INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS operators (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT UNIQUE NOT NULL,
	password TEXT NOT NULL,
	role TEXT DEFAULT 'operator',
	tenant_id INTEGER DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	failed_attempts INTEGER DEFAULT 0,
	locked_until DATETIME DEFAULT ''
);

CREATE TABLE IF NOT EXISTS agent_tokens (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	token_hash TEXT UNIQUE NOT NULL,
	label TEXT DEFAULT '',
	tenant_id INTEGER NOT NULL DEFAULT 1,
	expires_at DATETIME,
	revoked INTEGER DEFAULT 0,
	created_by INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	uuid TEXT UNIQUE NOT NULL,
	hostname TEXT,
	username TEXT,
	internal_ip TEXT,
	external_ip TEXT,
	os TEXT,
	arch TEXT,
	pid INTEGER,
	process TEXT,
	status TEXT DEFAULT 'active',
	last_checkin DATETIME,
	first_checkin DATETIME,
	sleep_sec INTEGER DEFAULT 10,
	jitter INTEGER DEFAULT 30,
	tags TEXT DEFAULT '',
	tenant_id INTEGER NOT NULL DEFAULT 1,
	agent_public_key BLOB,
	key_epoch INTEGER DEFAULT 0,
	agent_version TEXT DEFAULT '',
	expiry_unix INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	command TEXT NOT NULL,
	args TEXT DEFAULT '',
	status TEXT DEFAULT 'pending',
	result TEXT DEFAULT '',
	error TEXT DEFAULT '',
	operator_id INTEGER,
	tenant_id INTEGER NOT NULL DEFAULT 1,
	chunk_total INTEGER DEFAULT 1,
	chunk_index INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	sent_at DATETIME,
	completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS loot (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	type TEXT NOT NULL,
	name TEXT NOT NULL,
	data BLOB,
	path TEXT DEFAULT '',
	hash TEXT DEFAULT '',
	tenant_id INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts DATETIME DEFAULT CURRENT_TIMESTAMP,
	tenant_id INTEGER NOT NULL DEFAULT 0,
	operator_id INTEGER DEFAULT 0,
	username TEXT DEFAULT '',
	category TEXT DEFAULT 'system',
	action TEXT NOT NULL,
	severity TEXT DEFAULT 'info',
	details TEXT DEFAULT '',
	prev_hash TEXT NOT NULL,
	hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
	jti TEXT PRIMARY KEY,
	operator_id INTEGER NOT NULL,
	tenant_id INTEGER NOT NULL DEFAULT 1,
	expires_at DATETIME NOT NULL,
	revoked INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS marketplace_installs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	tenant_id INTEGER NOT NULL,
	package_id TEXT NOT NULL,
	package_version TEXT DEFAULT '',
	checksum TEXT DEFAULT '',
	installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	installed_by INTEGER DEFAULT 0,
	UNIQUE(tenant_id, package_id)
);

CREATE TABLE IF NOT EXISTS marketplace_keys (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kid TEXT UNIQUE NOT NULL,
	seed BLOB NOT NULL,
	active INTEGER DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS marketplace_packages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	package_id TEXT UNIQUE NOT NULL,
	name TEXT NOT NULL,
	version TEXT DEFAULT '',
	source TEXT NOT NULL,
	scope TEXT DEFAULT 'lab-only',
	sig TEXT DEFAULT '',
	kid TEXT DEFAULT '',
	script_id TEXT DEFAULT '',
	checksum TEXT DEFAULT '',
	tenant_id INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_sessions_checkin ON sessions(last_checkin);
CREATE INDEX IF NOT EXISTS idx_tasks_session ON tasks(session_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant ON tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_loot_tenant ON loot(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_tenant ON audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_refresh_op ON refresh_tokens(operator_id);
`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}

	// Columns added in 2.0 on databases created by 1.x.
	for _, m := range []struct{ table, column, decl string }{
		{"operators", "tenant_id", "INTEGER DEFAULT 1"},
		{"operators", "failed_attempts", "INTEGER DEFAULT 0"},
		{"operators", "locked_until", "DATETIME DEFAULT ''"},
		{"sessions", "tenant_id", "INTEGER DEFAULT 1"},
		{"sessions", "jitter", "INTEGER DEFAULT 30"},
		{"sessions", "agent_public_key", "BLOB DEFAULT NULL"},
		{"sessions", "key_epoch", "INTEGER DEFAULT 0"},
		{"sessions", "agent_version", "TEXT DEFAULT ''"},
		{"sessions", "expiry_unix", "INTEGER DEFAULT 0"},
		{"tasks", "tenant_id", "INTEGER DEFAULT 1"},
		{"tasks", "chunk_total", "INTEGER DEFAULT 1"},
		{"tasks", "chunk_index", "INTEGER DEFAULT 0"},
		{"loot", "tenant_id", "INTEGER DEFAULT 1"},
	} {
		if err := db.ensureColumn(m.table, m.column, m.decl); err != nil {
			return err
		}
	}

	return db.seed()
}

// ensureColumn adds a column when it does not exist yet (1.x -> 2.0 migration).
func (db *Database) ensureColumn(table, column, decl string) error {
	rows, err := db.conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = db.conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, decl))
	return err
}

func (db *Database) seed() error {
	var tenants int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM tenants").Scan(&tenants); err != nil {
		return err
	}
	if tenants == 0 {
		if _, err := db.conn.Exec(
			"INSERT INTO tenants (id, name, description, created_at) VALUES (?, ?, ?, ?)",
			DefaultTenantID, "default", "Primary tenant", now()); err != nil {
			return err
		}
	}

	// The default administrator is only seeded when the operator explicitly
	// asks for it, or when no operator exists at all and a bootstrap password
	// was provided through the environment. No hard coded credentials.
	var operators int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM operators").Scan(&operators); err != nil {
		return err
	}
	if operators > 0 {
		return nil
	}
	username := strings.TrimSpace(os.Getenv("C2_BOOTSTRAP_USER"))
	password := os.Getenv("C2_BOOTSTRAP_PASSWORD")
	if username == "" || password == "" {
		return nil
	}
	return db.CreateOperator(username, password, RoleAdmin, DefaultTenantID)
}

// ----------------------------------------------------------------- tenants ---

func (db *Database) CreateTenant(name, description string, allowImpact bool) (*Tenant, error) {
	t := &Tenant{Name: name, Description: description, AllowImpactModules: allowImpact, CreatedAt: now()}
	res, err := db.conn.Exec(
		"INSERT INTO tenants (name, description, allow_impact_modules, created_at) VALUES (?, ?, ?, ?)",
		t.Name, t.Description, boolInt(t.AllowImpactModules), t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.ID, _ = res.LastInsertId()
	return t, nil
}

func (db *Database) ListTenants() ([]Tenant, error) {
	rows, err := db.conn.Query(
		"SELECT id, name, description, allow_impact_modules, created_at FROM tenants ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tenant
	for rows.Next() {
		var t Tenant
		var allow int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &allow, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.AllowImpactModules = allow == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

func (db *Database) GetTenant(id int64) (*Tenant, error) {
	var t Tenant
	var allow int
	err := db.conn.QueryRow(
		"SELECT id, name, description, allow_impact_modules, created_at FROM tenants WHERE id = ?", id).
		Scan(&t.ID, &t.Name, &t.Description, &allow, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.AllowImpactModules = allow == 1
	return &t, nil
}

func (db *Database) DeleteTenant(id int64) error {
	if id == DefaultTenantID {
		return errors.New("the default tenant cannot be deleted")
	}
	if _, err := db.conn.Exec("DELETE FROM sessions WHERE tenant_id = ?", id); err != nil {
		return err
	}
	if _, err := db.conn.Exec("DELETE FROM tasks WHERE tenant_id = ?", id); err != nil {
		return err
	}
	if _, err := db.conn.Exec("DELETE FROM loot WHERE tenant_id = ?", id); err != nil {
		return err
	}
	if _, err := db.conn.Exec("DELETE FROM operators WHERE tenant_id = ?", id); err != nil {
		return err
	}
	if _, err := db.conn.Exec("DELETE FROM agent_tokens WHERE tenant_id = ?", id); err != nil {
		return err
	}
	_, err := db.conn.Exec("DELETE FROM tenants WHERE id = ?", id)
	return err
}

// --------------------------------------------------------------- operators ---

func (db *Database) CreateOperator(username, password, role string, tenantID int64) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if tenantID <= 0 {
		tenantID = DefaultTenantID
	}
	_, err = db.conn.Exec(
		"INSERT INTO operators (username, password, role, tenant_id, created_at) VALUES (?, ?, ?, ?, ?)",
		username, string(hash), role, tenantID, now())
	return err
}

func (db *Database) AuthenticateOperator(username, password string) (*Operator, error) {
	op := &Operator{}
	var tenantID int
	err := db.conn.QueryRow(
		"SELECT id, username, password, role, tenant_id, created_at FROM operators WHERE username = ?",
		username).Scan(&op.ID, &op.Username, &op.Password, &op.Role, &tenantID, &op.CreatedAt)
	if err != nil {
		return nil, ErrNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(op.Password), []byte(password)); err != nil {
		return nil, ErrNotFound
	}
	op.TenantID = int64(tenantID)
	return op, nil
}

func (db *Database) GetOperatorByID(id int64) (*Operator, error) {
	op := &Operator{}
	var tenantID int
	err := db.conn.QueryRow(
		"SELECT id, username, role, tenant_id, created_at FROM operators WHERE id = ?", id).
		Scan(&op.ID, &op.Username, &op.Role, &tenantID, &op.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	op.TenantID = int64(tenantID)
	return op, nil
}

func (db *Database) ListOperators(tenantID int64) ([]Operator, error) {
	rows, err := db.conn.Query(
		"SELECT id, username, role, tenant_id, created_at FROM operators WHERE tenant_id = ? ORDER BY id", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Operator
	for rows.Next() {
		var op Operator
		var tenant int
		if err := rows.Scan(&op.ID, &op.Username, &op.Role, &tenant, &op.CreatedAt); err != nil {
			return nil, err
		}
		op.TenantID = int64(tenant)
		out = append(out, op)
	}
	return out, rows.Err()
}

// RecordLoginFailure increments the failure counter and locks the account
// after too many consecutive failures.
func (db *Database) RecordLoginFailure(username string, maxAttempts int, lockFor time.Duration) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	locked := ""
	_, _ = db.conn.Exec("UPDATE operators SET failed_attempts = failed_attempts + 1 WHERE username = ?", username)
	var failed int
	if err := db.conn.QueryRow("SELECT failed_attempts FROM operators WHERE username = ?", username).Scan(&failed); err == nil {
		if failed >= maxAttempts {
			locked = time.Now().UTC().Add(lockFor).Format(time.RFC3339)
			_, _ = db.conn.Exec("UPDATE operators SET failed_attempts = 0, locked_until = ? WHERE username = ?", locked, username)
		}
	}
}

func (db *Database) ClearLoginFailures(username string) {
	_, _ = db.conn.Exec("UPDATE operators SET failed_attempts = 0, locked_until = '' WHERE username = ?", username)
}

// LockedUntil returns the lock timestamp for a username, empty when unlocked.
func (db *Database) LockedUntil(username string) string {
	var locked string
	_ = db.conn.QueryRow("SELECT locked_until FROM operators WHERE username = ?", username).Scan(&locked)
	return locked
}

// ------------------------------------------------------------ agent tokens ---

// CreateAgentToken stores the hash of a bootstrap token bound to a tenant.
func (db *Database) CreateAgentToken(label string, tenantID int64, tokenHash string, expiresAt time.Time, createdBy int64) error {
	var exp interface{}
	if !expiresAt.IsZero() {
		exp = expiresAt.UTC().Format(time.RFC3339)
	}
	_, err := db.conn.Exec(
		"INSERT INTO agent_tokens (token_hash, label, tenant_id, expires_at, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		tokenHash, label, tenantID, exp, createdBy, now())
	return err
}

// ResolveAgentToken maps a bootstrap token to its tenant. Expired or revoked
// tokens are rejected.
func (db *Database) ResolveAgentToken(tokenHash string) (int64, error) {
	var tenantID int64
	var expires sql.NullString
	var revoked int
	err := db.conn.QueryRow(
		"SELECT tenant_id, expires_at, revoked FROM agent_tokens WHERE token_hash = ?", tokenHash).
		Scan(&tenantID, &expires, &revoked)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if revoked == 1 {
		return 0, ErrNotFound
	}
	if expires.Valid && expires.String != "" {
		if ts, err := time.Parse(time.RFC3339, expires.String); err == nil && time.Now().UTC().After(ts) {
			return 0, ErrNotFound
		}
	}
	return tenantID, nil
}

func (db *Database) ListAgentTokens(tenantID int64) ([]AgentToken, error) {
	rows, err := db.conn.Query(
		"SELECT id, label, tenant_id, expires_at, revoked, created_at FROM agent_tokens WHERE tenant_id = ? ORDER BY id DESC",
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentToken
	for rows.Next() {
		var t AgentToken
		var expires sql.NullString
		var revoked int
		if err := rows.Scan(&t.ID, &t.Label, &t.TenantID, &expires, &revoked, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.ExpiresAt = expires.String
		t.Revoked = revoked == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

func (db *Database) RevokeAgentToken(id, tenantID int64) error {
	_, err := db.conn.Exec("UPDATE agent_tokens SET revoked = 1 WHERE id = ? AND tenant_id = ?", id, tenantID)
	return err
}

// ---------------------------------------------------------------- sessions ---

func (db *Database) RegisterSession(s *Session) error {
	if s.UUID == "" {
		s.UUID = uuid.New().String()
	}
	nowStr := now()
	s.FirstCheckIn = nowStr
	s.LastCheckIn = nowStr
	if s.Status == "" {
		s.Status = "active"
	}
	if s.SleepSec == 0 {
		s.SleepSec = 10
	}
	if s.TenantID <= 0 {
		s.TenantID = DefaultTenantID
	}
	_, err := db.conn.Exec(`
		INSERT INTO sessions (id, uuid, hostname, username, internal_ip, external_ip, os, arch, pid, process,
			status, last_checkin, first_checkin, sleep_sec, jitter, tags, tenant_id, agent_public_key, agent_version, expiry_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_checkin = excluded.last_checkin,
			status = 'active',
			pid = excluded.pid,
			process = excluded.process,
			hostname = excluded.hostname,
			username = excluded.username,
			internal_ip = excluded.internal_ip,
			external_ip = excluded.external_ip,
			os = excluded.os,
			arch = excluded.arch,
			sleep_sec = excluded.sleep_sec,
			jitter = excluded.jitter,
			agent_version = excluded.agent_version,
			expiry_unix = excluded.expiry_unix
	`, s.ID, s.UUID, s.Hostname, s.Username, s.InternalIP, s.ExternalIP,
		s.OS, s.Arch, s.PID, s.Process, s.Status, s.LastCheckIn, s.FirstCheckIn,
		s.SleepSec, s.Jitter, s.Tags, s.TenantID, nil, s.AgentVersion, s.ExpiryUnix)
	return err
}

func (db *Database) UpdateSessionCheckIn(sessionID, externalIP string) error {
	_, err := db.conn.Exec(
		"UPDATE sessions SET last_checkin = ?, status = 'active', external_ip = ? WHERE id = ?",
		now(), externalIP, sessionID)
	return err
}

func (db *Database) SetSessionStatus(sessionID, status string) error {
	_, err := db.conn.Exec("UPDATE sessions SET status = ? WHERE id = ?", status, sessionID)
	return err
}

const sessionColumns = `id, uuid, hostname, username, internal_ip, external_ip, os, arch, pid, process,
	status, last_checkin, first_checkin, sleep_sec, jitter, tags, tenant_id, agent_version, key_epoch, expiry_unix`

func scanSession(scan func(dest ...any) error) (*Session, error) {
	s := &Session{}
	var lastCheckIn, firstCheckIn sql.NullString
	err := scan(&s.ID, &s.UUID, &s.Hostname, &s.Username, &s.InternalIP, &s.ExternalIP,
		&s.OS, &s.Arch, &s.PID, &s.Process, &s.Status, &lastCheckIn, &firstCheckIn,
		&s.SleepSec, &s.Jitter, &s.Tags, &s.TenantID, &s.AgentVersion, &s.KeyEpoch, &s.ExpiryUnix)
	if err != nil {
		return nil, err
	}
	s.LastCheckIn = lastCheckIn.String
	s.FirstCheckIn = firstCheckIn.String
	return s, nil
}

func (db *Database) GetSession(sessionID string) (*Session, error) {
	s, err := scanSession(db.conn.QueryRow(
		"SELECT "+sessionColumns+" FROM sessions WHERE id = ?", sessionID).Scan)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return s, err
}

// ListSessions returns the sessions of one tenant. tenantID <= 0 lists all.
func (db *Database) ListSessions(tenantID int64, status string) ([]Session, error) {
	query := "SELECT " + sessionColumns + " FROM sessions"
	var args []any
	if tenantID > 0 {
		query += " WHERE tenant_id = ?"
		args = append(args, tenantID)
		if status != "" {
			query += " AND status = ?"
			args = append(args, status)
		}
	} else if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY last_checkin DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		s, err := scanSession(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (db *Database) TagSession(sessionID, tags string) error {
	_, err := db.conn.Exec("UPDATE sessions SET tags = ? WHERE id = ?", tags, sessionID)
	return err
}

func (db *Database) UpdateSessionSleep(sessionID string, sleepSec int) error {
	_, err := db.conn.Exec("UPDATE sessions SET sleep_sec = ? WHERE id = ?", sleepSec, sessionID)
	return err
}

// SetSessionAgentKey stores the agent public key and bumps the key epoch.
func (db *Database) SetSessionAgentKey(sessionID string, pub []byte, epoch int) error {
	_, err := db.conn.Exec(
		"UPDATE sessions SET agent_public_key = ?, key_epoch = ? WHERE id = ?", pub, epoch, sessionID)
	return err
}

func (db *Database) GetSessionAgentKey(sessionID string) ([]byte, int, error) {
	var pub []byte
	var epoch int
	err := db.conn.QueryRow(
		"SELECT agent_public_key, key_epoch FROM sessions WHERE id = ?", sessionID).Scan(&pub, &epoch)
	if err == sql.ErrNoRows {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	return pub, epoch, nil
}

func (db *Database) DeleteSession(sessionID string) error {
	db.conn.Exec("DELETE FROM tasks WHERE session_id = ?", sessionID)
	_, err := db.conn.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

// CleanupStaleSessions removes sessions that stopped checking in.
func (db *Database) CleanupStaleSessions(tenantID int64, maxAgeMinutes int) (int, error) {
	query := "DELETE FROM sessions WHERE last_checkin < datetime('now', ?)"
	args := []any{fmt.Sprintf("-%d minutes", maxAgeMinutes)}
	if tenantID > 0 {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (db *Database) DeleteAllSessions(tenantID int64) error {
	if tenantID > 0 {
		db.conn.Exec("DELETE FROM tasks WHERE tenant_id = ?", tenantID)
		_, err := db.conn.Exec("DELETE FROM sessions WHERE tenant_id = ?", tenantID)
		return err
	}
	db.conn.Exec("DELETE FROM tasks")
	_, err := db.conn.Exec("DELETE FROM sessions")
	return err
}

func (db *Database) SessionCount(tenantID int64) int {
	var count int
	if tenantID > 0 {
		db.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE tenant_id = ?", tenantID).Scan(&count)
	} else {
		db.conn.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	}
	return count
}

func (db *Database) ActiveSessionCount(tenantID int64) int {
	var count int
	if tenantID > 0 {
		db.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE tenant_id = ? AND status = 'active'", tenantID).Scan(&count)
	} else {
		db.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE status = 'active'").Scan(&count)
	}
	return count
}

// ------------------------------------------------------------------- tasks ---

func (db *Database) CreateTask(t *Task) error {
	t.CreatedAt = now()
	t.Status = "pending"
	if t.TenantID <= 0 {
		t.TenantID = DefaultTenantID
	}
	if t.ChunkTotal <= 0 {
		t.ChunkTotal = 1
	}
	result, err := db.conn.Exec(`
		INSERT INTO tasks (session_id, command, args, status, operator_id, tenant_id, chunk_total, chunk_index, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.SessionID, t.Command, t.Args, t.Status, t.OperatorID, t.TenantID, t.ChunkTotal, t.ChunkIndex, t.CreatedAt)
	if err != nil {
		return err
	}
	t.ID, _ = result.LastInsertId()
	return nil
}

func (db *Database) GetPendingTasks(sessionID string) ([]Task, error) {
	rows, err := db.conn.Query(`
		SELECT id, session_id, command, args, status, tenant_id, created_at
		FROM tasks WHERE session_id = ? AND status = 'pending' ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Command, &t.Args, &t.Status, &t.TenantID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetNextTask hands the oldest pending task to an agent.
func (db *Database) GetNextTask(sessionID string) (*Task, error) {
	t := &Task{}
	err := db.conn.QueryRow(`
		SELECT id, session_id, command, args, status, tenant_id, created_at
		FROM tasks WHERE session_id = ? AND status = 'pending' ORDER BY created_at ASC LIMIT 1
	`, sessionID).Scan(&t.ID, &t.SessionID, &t.Command, &t.Args, &t.Status, &t.TenantID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	db.conn.Exec("UPDATE tasks SET status = 'sent', sent_at = ? WHERE id = ?", now(), t.ID)
	t.Status = "sent"
	return t, nil
}

// CompleteTask stores the outcome of a task in one shot.
func (db *Database) CompleteTask(taskID int64, result, errMsg string) error {
	status := "completed"
	if errMsg != "" {
		status = "error"
	}
	_, err := db.conn.Exec(
		"UPDATE tasks SET status = ?, result = ?, error = ?, completed_at = ? WHERE id = ?",
		status, result, errMsg, now(), taskID)
	return err
}

// FinishTask closes a task that was filled chunk by chunk. The accumulated
// result is preserved, only the status and the error are updated.
func (db *Database) FinishTask(taskID int64, errMsg string) error {
	status := "completed"
	if errMsg != "" {
		status = "error"
	}
	_, err := db.conn.Exec(
		"UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?",
		status, errMsg, now(), taskID)
	return err
}

// AppendTaskChunk stores one chunk of a multi part result.
func (db *Database) AppendTaskChunk(taskID int64, chunk string, index, total int) error {
	_, err := db.conn.Exec(
		"UPDATE tasks SET result = result || ?, chunk_index = ?, chunk_total = ? WHERE id = ?",
		chunk, index, total, taskID)
	return err
}

func (db *Database) GetTasksBySession(tenantID int64, sessionID string, limit int) ([]Task, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, session_id, command, args, status, result, error, operator_id, tenant_id,
		chunk_total, chunk_index, created_at, sent_at, completed_at FROM tasks WHERE session_id = ?`
	args := []any{sessionID}
	if tenantID > 0 {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		var sentAt, completedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Command, &t.Args, &t.Status, &t.Result, &t.Error,
			&t.OperatorID, &t.TenantID, &t.ChunkTotal, &t.ChunkIndex, &t.CreatedAt,
			&sentAt, &completedAt); err != nil {
			return nil, err
		}
		t.SentAt = sentAt.String
		t.CompletedAt = completedAt.String
		out = append(out, t)
	}
	return out, rows.Err()
}

func (db *Database) GetTask(tenantID int64, taskID int64) (*Task, error) {
	query := `SELECT id, session_id, command, args, status, result, error, operator_id, tenant_id,
		chunk_total, chunk_index, created_at, sent_at, completed_at FROM tasks WHERE id = ?`
	args := []any{taskID}
	if tenantID > 0 {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	t := &Task{}
	var sentAt, completedAt sql.NullString
	err := db.conn.QueryRow(query, args...).Scan(&t.ID, &t.SessionID, &t.Command, &t.Args, &t.Status,
		&t.Result, &t.Error, &t.OperatorID, &t.TenantID, &t.ChunkTotal, &t.ChunkIndex,
		&t.CreatedAt, &sentAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.SentAt = sentAt.String
	t.CompletedAt = completedAt.String
	return t, nil
}

// ------------------------------------------------------------------- loot ---

func (db *Database) SaveLoot(l *Loot) error {
	l.CreatedAt = now()
	if l.TenantID <= 0 {
		l.TenantID = DefaultTenantID
	}
	_, err := db.conn.Exec(`
		INSERT INTO loot (session_id, type, name, data, path, hash, tenant_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, l.SessionID, l.Type, l.Name, l.Data, l.Path, l.Hash, l.TenantID, l.CreatedAt)
	return err
}

func (db *Database) ListLoot(tenantID int64, sessionID string) ([]Loot, error) {
	query := "SELECT id, session_id, type, name, path, hash, tenant_id, created_at FROM loot"
	var args []any
	if tenantID > 0 {
		query += " WHERE tenant_id = ?"
		args = append(args, tenantID)
		if sessionID != "" {
			query += " AND session_id = ?"
			args = append(args, sessionID)
		}
	} else if sessionID != "" {
		query += " WHERE session_id = ?"
		args = append(args, sessionID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Loot
	for rows.Next() {
		var l Loot
		if err := rows.Scan(&l.ID, &l.SessionID, &l.Type, &l.Name, &l.Path, &l.Hash,
			&l.TenantID, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ------------------------------------------------------------------ audit ---

const auditGenesis = "0000000000000000000000000000000000000000000000000000000000000000"

// AppendAudit writes one entry to the tamper evident audit chain.
// H(n) = SHA256(H(n-1) || canonical fields)
func (db *Database) AppendAudit(e *AuditEntry) error {
	var prev string
	if err := db.conn.QueryRow("SELECT hash FROM audit_log ORDER BY id DESC LIMIT 1").Scan(&prev); err != nil {
		prev = auditGenesis
	}
	if prev == "" {
		prev = auditGenesis
	}
	e.TS = now()
	e.PrevHash = prev
	e.Hash = auditHash(e)
	_, err := db.conn.Exec(`
		INSERT INTO audit_log (ts, tenant_id, operator_id, username, category, action, severity, details, prev_hash, hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.TS, e.TenantID, e.OperatorID, e.Username, e.Category, e.Action, e.Severity, e.Details, e.PrevHash, e.Hash)
	return err
}

func auditHash(e *AuditEntry) string {
	payload := strings.Join([]string{
		e.PrevHash, e.TS, fmt.Sprintf("%d", e.TenantID), fmt.Sprintf("%d", e.OperatorID),
		e.Username, e.Category, e.Action, e.Severity, e.Details,
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (db *Database) ListAudit(tenantID int64, limit int) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, ts, tenant_id, operator_id, username, category, action, severity, details, prev_hash, hash FROM audit_log`
	var args []any
	if tenantID > 0 {
		query += " WHERE tenant_id = ?"
		args = append(args, tenantID)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.TS, &e.TenantID, &e.OperatorID, &e.Username, &e.Category,
			&e.Action, &e.Severity, &e.Details, &e.PrevHash, &e.Hash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// VerifyAuditChain recomputes the whole chain and reports the first break.
func (db *Database) VerifyAuditChain() (bool, string, error) {
	rows, err := db.conn.Query(`SELECT id, ts, tenant_id, operator_id, username, category, action,
		severity, details, prev_hash, hash FROM audit_log ORDER BY id ASC`)
	if err != nil {
		return false, "", err
	}
	defer rows.Close()

	prev := auditGenesis
	count := 0
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.TS, &e.TenantID, &e.OperatorID, &e.Username, &e.Category,
			&e.Action, &e.Severity, &e.Details, &e.PrevHash, &e.Hash); err != nil {
			return false, "", err
		}
		if e.PrevHash != prev {
			return false, fmt.Sprintf("entry %d: prev_hash mismatch", e.ID), nil
		}
		if auditHash(&e) != e.Hash {
			return false, fmt.Sprintf("entry %d: hash mismatch", e.ID), nil
		}
		prev = e.Hash
		count++
	}
	return true, fmt.Sprintf("%d entries verified", count), rows.Err()
}

// -------------------------------------------------------- refresh tokens ---

func (db *Database) StoreRefreshToken(jti string, operatorID, tenantID int64, expires time.Time) error {
	_, err := db.conn.Exec(
		"INSERT INTO refresh_tokens (jti, operator_id, tenant_id, expires_at, revoked, created_at) VALUES (?, ?, ?, ?, 0, ?)",
		jti, operatorID, tenantID, expires.UTC().Format(time.RFC3339), now())
	return err
}

func (db *Database) GetRefreshToken(jti string) (*RefreshToken, error) {
	var t RefreshToken
	var expires string
	var revoked int
	err := db.conn.QueryRow(
		"SELECT jti, operator_id, tenant_id, expires_at, revoked FROM refresh_tokens WHERE jti = ?", jti).
		Scan(&t.JTI, &t.OperatorID, &t.TenantID, &expires, &revoked)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Revoked = revoked == 1
	if ts, err := time.Parse(time.RFC3339, expires); err == nil {
		t.ExpiresAt = ts
	}
	return &t, nil
}

func (db *Database) RevokeRefreshToken(jti string) error {
	_, err := db.conn.Exec("UPDATE refresh_tokens SET revoked = 1 WHERE jti = ?", jti)
	return err
}

func (db *Database) RevokeOperatorRefresh(operatorID int64) error {
	_, err := db.conn.Exec("UPDATE refresh_tokens SET revoked = 1 WHERE operator_id = ?", operatorID)
	return err
}

// ------------------------------------------------------------- marketplace ---

func (db *Database) InstallPackage(tenantID int64, packageID, version, checksum string, by int64) error {
	_, err := db.conn.Exec(`
		INSERT INTO marketplace_installs (tenant_id, package_id, package_version, checksum, installed_at, installed_by)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, package_id) DO UPDATE SET
			package_version = excluded.package_version,
			checksum = excluded.checksum,
			installed_at = excluded.installed_at,
			installed_by = excluded.installed_by
	`, tenantID, packageID, version, checksum, now(), by)
	return err
}

func (db *Database) UninstallPackage(tenantID int64, packageID string) error {
	_, err := db.conn.Exec("DELETE FROM marketplace_installs WHERE tenant_id = ? AND package_id = ?", tenantID, packageID)
	return err
}

func (db *Database) ListInstalls(tenantID int64) ([]MarketplaceInstall, error) {
	rows, err := db.conn.Query(`
		SELECT i.id, i.tenant_id, i.package_id, i.package_version, i.checksum, i.installed_at, i.installed_by,
		       COALESCE(o.username, '')
		FROM marketplace_installs i
		LEFT JOIN operators o ON o.id = i.installed_by
		WHERE i.tenant_id = ? ORDER BY i.installed_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MarketplaceInstall
	for rows.Next() {
		var m MarketplaceInstall
		if err := rows.Scan(&m.ID, &m.TenantID, &m.PackageID, &m.PackageVersion, &m.Checksum,
			&m.InstalledAt, &m.InstalledBy, &m.InstalledByName); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (db *Database) IsPackageInstalled(tenantID int64, packageID string) bool {
	var n int
	db.conn.QueryRow(
		"SELECT COUNT(*) FROM marketplace_installs WHERE tenant_id = ? AND package_id = ?", tenantID, packageID).Scan(&n)
	return n > 0
}

// MarketplacePackageRow is a package published by the operator.
type MarketplacePackageRow struct {
	ID        int64  `json:"id"`
	PackageID string `json:"package_id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Source    string `json:"source"`
	Scope     string `json:"scope"`
	Signature string `json:"sig"`
	Kid       string `json:"kid"`
	ScriptID  string `json:"script_id"`
	Checksum  string `json:"checksum"`
	TenantID  int64  `json:"tenant_id"`
	CreatedAt string `json:"created_at"`
}

// SaveCustomPackage stores a package published from the console.
func (db *Database) SaveCustomPackage(p *MarketplacePackageRow) error {
	if p.CreatedAt == "" {
		p.CreatedAt = now()
	}
	_, err := db.conn.Exec(`
		INSERT INTO marketplace_packages (package_id, name, version, source, scope, sig, kid, script_id, checksum, tenant_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(package_id) DO UPDATE SET
			name = excluded.name, version = excluded.version, source = excluded.source,
			scope = excluded.scope, sig = excluded.sig, kid = excluded.kid,
			script_id = excluded.script_id, checksum = excluded.checksum,
			tenant_id = excluded.tenant_id, created_at = excluded.created_at
	`, p.PackageID, p.Name, p.Version, p.Source, p.Scope, p.Signature, p.Kid, p.ScriptID,
		p.Checksum, p.TenantID, p.CreatedAt)
	return err
}

// ListCustomPackages returns the packages published for a tenant.
func (db *Database) ListCustomPackages(tenantID int64) ([]MarketplacePackageRow, error) {
	rows, err := db.conn.Query(`
		SELECT id, package_id, name, version, source, scope, sig, kid, script_id, checksum, tenant_id, created_at
		FROM marketplace_packages WHERE tenant_id = ? ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MarketplacePackageRow
	for rows.Next() {
		var p MarketplacePackageRow
		if err := rows.Scan(&p.ID, &p.PackageID, &p.Name, &p.Version, &p.Source, &p.Scope,
			&p.Signature, &p.Kid, &p.ScriptID, &p.Checksum, &p.TenantID, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetCustomPackage returns one published package.
func (db *Database) GetCustomPackage(packageID string) (*MarketplacePackageRow, error) {
	p := &MarketplacePackageRow{}
	err := db.conn.QueryRow(`
		SELECT id, package_id, name, version, source, scope, sig, kid, script_id, checksum, tenant_id, created_at
		FROM marketplace_packages WHERE package_id = ?
	`, packageID).Scan(&p.ID, &p.PackageID, &p.Name, &p.Version, &p.Source, &p.Scope,
		&p.Signature, &p.Kid, &p.ScriptID, &p.Checksum, &p.TenantID, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// EnsureMarketKey returns the signing seed of the marketplace. The key is
// created on the first run and stored so the signatures stay verifiable
// across restarts.
func (db *Database) EnsureMarketKey() (kid string, seed []byte, err error) {
	var storedKid string
	var storedSeed []byte
	err = db.conn.QueryRow("SELECT kid, seed FROM marketplace_keys WHERE active = 1 ORDER BY id DESC LIMIT 1").
		Scan(&storedKid, &storedSeed)
	if err == nil && len(storedSeed) == 32 {
		return storedKid, storedSeed, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return "", nil, err
	}
	seed = make([]byte, 32)
	if _, err = cryptorand.Read(seed); err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(seed)
	kid = hex.EncodeToString(sum[:])[:16]
	if _, err = db.conn.Exec(
		"INSERT INTO marketplace_keys (kid, seed, active, created_at) VALUES (?, ?, 1, ?)",
		kid, seed, now()); err != nil {
		return "", nil, err
	}
	return kid, seed, nil
}

// ----------------------------------------------------------------- helpers ---

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
