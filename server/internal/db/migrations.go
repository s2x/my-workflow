package db

import "fmt"

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL UNIQUE,
			repo_path    TEXT NOT NULL,
			base_branch  TEXT NOT NULL DEFAULT 'main',
			auto_test    INTEGER NOT NULL DEFAULT 0,
			auto_review  INTEGER NOT NULL DEFAULT 0,
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

	addColumnIfNotExists := func(table, column, definition string) error {
		var count int
		err := db.conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", table, column)).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			_, err = db.conn.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
			return err
		}
		return nil
	}

	if err := addColumnIfNotExists("projects", "auto_test", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("adding auto_test column: %w", err)
	}
	if err := addColumnIfNotExists("projects", "auto_review", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("adding auto_review column: %w", err)
	}
	if err := addColumnIfNotExists("projects", "runner", "TEXT NOT NULL DEFAULT 'qwen'"); err != nil {
		return fmt.Errorf("adding runner column: %w", err)
	}
	if err := addColumnIfNotExists("projects", "model", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("adding model column: %w", err)
	}
	if err := addColumnIfNotExists("projects", "model_high", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("adding model_high column: %w", err)
	}
	if err := addColumnIfNotExists("projects", "model_medium", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("adding model_medium column: %w", err)
	}
	if err := addColumnIfNotExists("projects", "model_low", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("adding model_low column: %w", err)
	}

	if err := addColumnIfNotExists("tickets", "ai_generated", "BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
		return fmt.Errorf("adding ai_generated column: %w", err)
	}
	if err := addColumnIfNotExists("tickets", "ai_metadata", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return fmt.Errorf("adding ai_metadata column: %w", err)
	}
	if err := addColumnIfNotExists("tickets", "refinement_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("adding refinement_count column: %w", err)
	}

	return nil
}
