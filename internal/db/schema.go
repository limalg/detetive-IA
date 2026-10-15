package db

import (
	"database/sql"
)

// Schema contains the DDL for the Detetive IA database.
const Schema = `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS cases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    victim TEXT NOT NULL,
    solution_suspect_id INTEGER,
    solution_weapon TEXT NOT NULL,
    solution_motive TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS suspects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    case_id INTEGER NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    personality TEXT NOT NULL,
    alibi TEXT NOT NULL,
    secret TEXT NOT NULL,
    is_culprit BOOLEAN NOT NULL DEFAULT 0,
    tier1_alibi TEXT DEFAULT '',
    tier2_petty_crime TEXT DEFAULT '',
    tier3_truth TEXT DEFAULT '',
    cross_suspicion TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS clues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    case_id INTEGER NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    location TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS interrogation_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    suspect_id INTEGER NOT NULL REFERENCES suspects(id) ON DELETE CASCADE,
    sender TEXT NOT NULL,
    message TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS investigations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    case_id INTEGER NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'in_progress',
    time_remaining_minutes INTEGER NOT NULL DEFAULT 420,
    warrants_remaining INTEGER NOT NULL DEFAULT 2,
    accused_suspect_id INTEGER REFERENCES suspects(id),
    accused_weapon TEXT,
    accused_motive TEXT,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_suspects_case_id ON suspects(case_id);
CREATE INDEX IF NOT EXISTS idx_clues_case_id ON clues(case_id);
CREATE INDEX IF NOT EXISTS idx_interrogation_messages_session ON interrogation_messages(session_id, suspect_id);
CREATE INDEX IF NOT EXISTS idx_investigations_user_case ON investigations(user_id, case_id);
`

// RunMigrations executes all DDL statements to ensure all tables and indexes exist.
func RunMigrations(database *sql.DB) error {
	if _, err := database.Exec(Schema); err != nil {
		return err
	}
	alterQueries := []string{
		"ALTER TABLE investigations ADD COLUMN time_remaining_minutes INTEGER NOT NULL DEFAULT 420;",
		"ALTER TABLE investigations ADD COLUMN warrants_remaining INTEGER NOT NULL DEFAULT 2;",
		"ALTER TABLE suspects ADD COLUMN tier1_alibi TEXT DEFAULT '';",
		"ALTER TABLE suspects ADD COLUMN tier2_petty_crime TEXT DEFAULT '';",
		"ALTER TABLE suspects ADD COLUMN tier3_truth TEXT DEFAULT '';",
		"ALTER TABLE suspects ADD COLUMN cross_suspicion TEXT DEFAULT '';",
	}
	for _, q := range alterQueries {
		_, _ = database.Exec(q)
	}
	return nil
}
