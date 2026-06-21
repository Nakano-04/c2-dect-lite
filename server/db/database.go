package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

type Database struct {
	conn *sql.DB
}

type Operator struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
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
	Tags         string `json:"tags"`
}

type Task struct {
	ID         int64  `json:"id"`
	SessionID  string `json:"session_id"`
	Command    string `json:"command"`
	Args       string `json:"args"`
	Status     string `json:"status"` // pending, sent, completed, error
	Result     string `json:"result"`
	Error      string `json:"error"`
	OperatorID int64  `json:"operator_id"`
	CreatedAt  string `json:"created_at"`
	SentAt     string `json:"sent_at"`
	CompletedAt string `json:"completed_at"`
}

type Loot struct {
	ID        int64  `json:"id"`
	SessionID string `json:"session_id"`
	Type      string `json:"type"` // file, credential, hash
	Name      string `json:"name"`
	Data      []byte `json:"data"`
	Path      string `json:"path"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"created_at"`
}

func New(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	db := &Database{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *Database) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS operators (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		role TEXT DEFAULT 'operator',
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
		tags TEXT DEFAULT ''
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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		sent_at DATETIME,
		completed_at DATETIME,
		FOREIGN KEY (session_id) REFERENCES sessions(id),
		FOREIGN KEY (operator_id) REFERENCES operators(id)
	);

	CREATE TABLE IF NOT EXISTS loot (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		data BLOB,
		path TEXT DEFAULT '',
		hash TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}
	// Add public_key column if not exists (migration)
	db.conn.Exec("ALTER TABLE sessions ADD COLUMN public_key BLOB DEFAULT NULL")
	return nil
}

func (db *Database) CreateOperator(username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec("INSERT INTO operators (username, password, role) VALUES (?, ?, ?)",
		username, string(hash), role)
	return err
}

func (db *Database) AuthenticateOperator(username, password string) (*Operator, error) {
	op := &Operator{}
	err := db.conn.QueryRow("SELECT id, username, password, role FROM operators WHERE username = ?",
		username).Scan(&op.ID, &op.Username, &op.Password, &op.Role)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(op.Password), []byte(password)); err != nil {
		return nil, err
	}
	return op, nil
}

func (db *Database) RegisterSession(s *Session) error {
	if s.UUID == "" {
		s.UUID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.FirstCheckIn = now
	s.LastCheckIn = now
	if s.Status == "" {
		s.Status = "active"
	}
	if s.SleepSec == 0 {
		s.SleepSec = 10
	}

	_, err := db.conn.Exec(`
		INSERT INTO sessions (id, uuid, hostname, username, internal_ip, external_ip, os, arch, pid, process, status, last_checkin, first_checkin, sleep_sec)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_checkin = excluded.last_checkin,
			status = 'active',
			pid = excluded.pid,
			hostname = excluded.hostname,
			username = excluded.username,
			internal_ip = excluded.internal_ip,
			external_ip = excluded.external_ip
	`, s.ID, s.UUID, s.Hostname, s.Username, s.InternalIP, s.ExternalIP,
		s.OS, s.Arch, s.PID, s.Process, s.Status, s.LastCheckIn, s.FirstCheckIn, s.SleepSec)
	return err
}

func (db *Database) UpdateSessionCheckIn(sessionID, externalIP string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.Exec("UPDATE sessions SET last_checkin = ?, status = 'active', external_ip = ? WHERE id = ?",
		now, externalIP, sessionID)
	return err
}

func (db *Database) SetSessionStatus(sessionID, status string) error {
	_, err := db.conn.Exec("UPDATE sessions SET status = ? WHERE id = ?", status, sessionID)
	return err
}

func (db *Database) GetSession(sessionID string) (*Session, error) {
	s := &Session{}
	err := db.conn.QueryRow(`
		SELECT id, uuid, hostname, username, internal_ip, external_ip, os, arch, pid, process, status, last_checkin, first_checkin, sleep_sec, tags
		FROM sessions WHERE id = ?
	`, sessionID).Scan(&s.ID, &s.UUID, &s.Hostname, &s.Username, &s.InternalIP, &s.ExternalIP,
		&s.OS, &s.Arch, &s.PID, &s.Process, &s.Status, &s.LastCheckIn, &s.FirstCheckIn, &s.SleepSec, &s.Tags)
	return s, err
}

func (db *Database) ListSessions(status string) ([]Session, error) {
	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = db.conn.Query("SELECT id, uuid, hostname, username, internal_ip, external_ip, os, arch, pid, process, status, last_checkin, first_checkin, sleep_sec, tags FROM sessions WHERE status = ? ORDER BY last_checkin DESC", status)
	} else {
		rows, err = db.conn.Query("SELECT id, uuid, hostname, username, internal_ip, external_ip, os, arch, pid, process, status, last_checkin, first_checkin, sleep_sec, tags FROM sessions ORDER BY last_checkin DESC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UUID, &s.Hostname, &s.Username, &s.InternalIP, &s.ExternalIP,
			&s.OS, &s.Arch, &s.PID, &s.Process, &s.Status, &s.LastCheckIn, &s.FirstCheckIn, &s.SleepSec, &s.Tags); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (db *Database) TagSession(sessionID, tags string) error {
	_, err := db.conn.Exec("UPDATE sessions SET tags = ? WHERE id = ?", tags, sessionID)
	return err
}

func (db *Database) CreateTask(t *Task) error {
	now := time.Now().UTC().Format(time.RFC3339)
	t.CreatedAt = now
	t.Status = "pending"
	result, err := db.conn.Exec(`
		INSERT INTO tasks (session_id, command, args, status, operator_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, t.SessionID, t.Command, t.Args, t.Status, t.OperatorID, t.CreatedAt)
	if err != nil {
		return err
	}
	t.ID, _ = result.LastInsertId()
	return nil
}

func (db *Database) GetPendingTasks(sessionID string) ([]Task, error) {
	rows, err := db.conn.Query(`
		SELECT id, session_id, command, args, status, created_at
		FROM tasks WHERE session_id = ? AND status = 'pending' ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Command, &t.Args, &t.Status, &t.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (db *Database) GetNextTask(sessionID string) (*Task, error) {
	t := &Task{}
	err := db.conn.QueryRow(`
		SELECT id, session_id, command, args, status, created_at
		FROM tasks WHERE session_id = ? AND status = 'pending' ORDER BY created_at ASC LIMIT 1
	`, sessionID).Scan(&t.ID, &t.SessionID, &t.Command, &t.Args, &t.Status, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	db.conn.Exec("UPDATE tasks SET status = 'sent', sent_at = ? WHERE id = ?", now, t.ID)
	t.Status = "sent"
	return t, nil
}

func (db *Database) CompleteTask(taskID int64, result, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	status := "completed"
	if errMsg != "" {
		status = "error"
	}
	_, err := db.conn.Exec("UPDATE tasks SET status = ?, result = ?, error = ?, completed_at = ? WHERE id = ?",
		status, result, errMsg, now, taskID)
	return err
}

func (db *Database) GetTasksBySession(sessionID string, limit int) ([]Task, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(`
		SELECT id, session_id, command, args, status, result, error, created_at, sent_at, completed_at
		FROM tasks WHERE session_id = ? ORDER BY created_at DESC LIMIT ?
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Command, &t.Args, &t.Status, &t.Result, &t.Error, &t.CreatedAt, &t.SentAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (db *Database) SaveLoot(l *Loot) error {
	now := time.Now().UTC().Format(time.RFC3339)
	l.CreatedAt = now
	_, err := db.conn.Exec(`
		INSERT INTO loot (session_id, type, name, data, path, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, l.SessionID, l.Type, l.Name, l.Data, l.Path, l.Hash, l.CreatedAt)
	return err
}

func (db *Database) ListLoot(sessionID string) ([]Loot, error) {
	var rows *sql.Rows
	var err error
	if sessionID != "" {
		rows, err = db.conn.Query("SELECT id, session_id, type, name, path, hash, created_at FROM loot WHERE session_id = ? ORDER BY created_at DESC", sessionID)
	} else {
		rows, err = db.conn.Query("SELECT id, session_id, type, name, path, hash, created_at FROM loot ORDER BY created_at DESC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loots []Loot
	for rows.Next() {
		var l Loot
		if err := rows.Scan(&l.ID, &l.SessionID, &l.Type, &l.Name, &l.Path, &l.Hash, &l.CreatedAt); err != nil {
			return nil, err
		}
		loots = append(loots, l)
	}
	return loots, nil
}

func (db *Database) Close() error {
	return db.conn.Close()
}

func (db *Database) GetSessionByUUID(sessionUUID string) (*Session, error) {
	s := &Session{}
	err := db.conn.QueryRow(`
		SELECT id, uuid, hostname, username, internal_ip, external_ip, os, arch, pid, process, status, last_checkin, first_checkin, sleep_sec, tags
		FROM sessions WHERE uuid = ?
	`, sessionUUID).Scan(&s.ID, &s.UUID, &s.Hostname, &s.Username, &s.InternalIP, &s.ExternalIP,
		&s.OS, &s.Arch, &s.PID, &s.Process, &s.Status, &s.LastCheckIn, &s.FirstCheckIn, &s.SleepSec, &s.Tags)
	return s, err
}

func (db *Database) GetOperatorByID(id int64) (*Operator, error) {
	op := &Operator{}
	err := db.conn.QueryRow("SELECT id, username, role, created_at FROM operators WHERE id = ?", id).
		Scan(&op.ID, &op.Username, &op.Role, &op.CreatedAt)
	return op, err
}

func (db *Database) SessionCount() int {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	return count
}

func (db *Database) ActiveSessionCount() int {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM sessions WHERE status = 'active'").Scan(&count)
	return count
}

func (db *Database) PendingTaskCount(sessionID string) int {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE session_id = ? AND status = 'pending'", sessionID).Scan(&count)
	return count
}

func (db *Database) UpdateSessionSleep(sessionID string, sleepSec int) error {
	_, err := db.conn.Exec("UPDATE sessions SET sleep_sec = ? WHERE id = ?", sleepSec, sessionID)
	return err
}

// InitDefaultUser creates a default admin user if none exists
func (db *Database) InitDefaultUser() error {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM operators").Scan(&count)
	if count == 0 {
		return db.CreateOperator("admin", "c2-dect", "admin")
	}
	return nil
}

func (db *Database) GetSessionPublicKey(sessionID string) ([]byte, error) {
	var key []byte
	err := db.conn.QueryRow("SELECT public_key FROM sessions WHERE id = ?", sessionID).Scan(&key)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return key, err
}

func (db *Database) SetSessionPublicKey(sessionID string, publicKey []byte) error {
	_, err := db.conn.Exec("UPDATE sessions SET public_key = ? WHERE id = ?", publicKey, sessionID)
	return err
}

// DeleteSession removes a session and its tasks
func (db *Database) DeleteSession(sessionID string) error {
	db.conn.Exec("DELETE FROM tasks WHERE session_id = ?", sessionID)
	_, err := db.conn.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

// CleanupStaleSessions removes sessions that haven't checked in for given duration
func (db *Database) CleanupStaleSessions(maxAgeMinutes int) (int, error) {
	result, err := db.conn.Exec(
		"DELETE FROM sessions WHERE last_check_in < datetime('now', ?)",
		fmt.Sprintf("-%d minutes", maxAgeMinutes))
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// DeleteAllSessions removes all sessions and tasks
func (db *Database) DeleteAllSessions() error {
	db.conn.Exec("DELETE FROM tasks")
	_, err := db.conn.Exec("DELETE FROM sessions")
	return err
}
