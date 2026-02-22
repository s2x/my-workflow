package db

import "fmt"

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL UNIQUE,
			repo_path    TEXT NOT NULL,
			base_branch  TEXT NOT NULL DEFAULT 'main',
			stage_branch TEXT NOT NULL DEFAULT 'stage',
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tickets (
			id                  TEXT PRIMARY KEY,
			project_id          TEXT NOT NULL DEFAULT '' REFERENCES projects(id),
			jira_key            TEXT NOT NULL UNIQUE,
			summary             TEXT NOT NULL DEFAULT '',
			description         TEXT NOT NULL DEFAULT '',
			status              TEXT NOT NULL DEFAULT '',
			priority            TEXT NOT NULL DEFAULT '',
			assignee            TEXT NOT NULL DEFAULT '',
			labels              TEXT NOT NULL DEFAULT '',
			acceptance_criteria TEXT NOT NULL DEFAULT '',
			ticket_type         TEXT NOT NULL DEFAULT '',
			project_key         TEXT NOT NULL DEFAULT '',
			source              TEXT NOT NULL DEFAULT 'manual',
			raw_json            TEXT NOT NULL DEFAULT '{}',
			jira_updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			synced_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS workflows (
			id          TEXT PRIMARY KEY,
			project_id  TEXT NOT NULL DEFAULT '' REFERENCES projects(id),
			ticket_id   TEXT NOT NULL REFERENCES tickets(id),
			status      TEXT NOT NULL DEFAULT 'CREATED',
			branch_name TEXT NOT NULL DEFAULT '',
			spec        TEXT NOT NULL DEFAULT '',
			retry_count INTEGER NOT NULL DEFAULT 0,
			error       TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id             TEXT PRIMARY KEY,
			workflow_id    TEXT NOT NULL REFERENCES workflows(id),
			parent_task_id TEXT NOT NULL DEFAULT '',
			type           TEXT NOT NULL,
			status         TEXT NOT NULL DEFAULT 'QUEUED',
			agent          TEXT NOT NULL,
			prompt         TEXT NOT NULL DEFAULT '',
			output         TEXT NOT NULL DEFAULT '',
			parsed         TEXT NOT NULL DEFAULT '',
			error          TEXT NOT NULL DEFAULT '',
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at     DATETIME,
			completed_at   DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id          TEXT PRIMARY KEY,
			workflow_id TEXT REFERENCES workflows(id),
			role        TEXT NOT NULL,
			agent_type  TEXT NOT NULL DEFAULT '',
			content     TEXT NOT NULL,
			metadata    TEXT NOT NULL DEFAULT '{}',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_logs (
			id        TEXT PRIMARY KEY,
			task_id   TEXT NOT NULL REFERENCES tasks(id),
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			log_level TEXT NOT NULL DEFAULT 'info',
			message   TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON task_logs(task_id, timestamp)`,
	}

	for i, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}

	return nil
}
