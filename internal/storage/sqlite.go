/**
 * SPDX-FileComment: Storage Module Implementation
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file sqlite.go
 * @brief SQLite Storage Layer with WAL mode
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

type Store struct {
	db *sql.DB
}

// NewStore initializes a new SQLite connection, enables WAL mode, and sets up the schema.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Info().Str("path", dbPath).Msg("SQLite storage initialized with WAL mode")
	return s, nil
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS adapters (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active'
	);

	CREATE TABLE IF NOT EXISTS cursors (
		adapter_id TEXT PRIMARY KEY,
		last_value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(adapter_id) REFERENCES adapters(id)
	);

	CREATE TABLE IF NOT EXISTS fragments (
		id TEXT PRIMARY KEY,
		adapter_id TEXT NOT NULL,
		payload_encrypted BLOB NOT NULL,
		encrypted_dek BLOB NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(adapter_id) REFERENCES adapters(id)
	);

	CREATE TABLE IF NOT EXISTS packages (
		id TEXT PRIMARY KEY,
		payload_json TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		idempotency_key TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS dlq (
		id TEXT PRIMARY KEY,
		reference_id TEXT NOT NULL,
		reference_type TEXT NOT NULL,
		error_reason TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		details TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// GetDB returns the underlying sql.DB instance for custom operations.
func (s *Store) GetDB() *sql.DB {
	return s.db
}

// RegisterAdapter ensures an adapter entry exists in the database.
func (s *Store) RegisterAdapter(id, name string) error {
	_, err := s.db.Exec(`
		INSERT INTO adapters (id, name, status)
		VALUES (?, ?, 'active')
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, status = 'active'`,
		id, name)
	return err
}

// GetCursor retrieves the last saved cursor for a specific adapter.
func (s *Store) GetCursor(adapterID string) (string, error) {
	var cursor string
	err := s.db.QueryRow("SELECT last_value FROM cursors WHERE adapter_id = ?", adapterID).Scan(&cursor)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return cursor, err
}

// UpdateCursor updates or creates the cursor for an adapter.
func (s *Store) UpdateCursor(adapterID string, lastValue string) error {
	_, err := s.db.Exec(`
		INSERT INTO cursors (adapter_id, last_value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(adapter_id) DO UPDATE SET last_value = excluded.last_value, updated_at = CURRENT_TIMESTAMP`,
		adapterID, lastValue)
	return err
}

// SaveFragment persists an encrypted fragment and its encrypted DEK.
func (s *Store) SaveFragment(id, adapterID string, payloadEnc, dekEnc []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO fragments (id, adapter_id, payload_encrypted, encrypted_dek, status, created_at)
		VALUES (?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP)`,
		id, adapterID, payloadEnc, dekEnc)
	return err
}

// Fragment represents a stored encrypted data unit.
type Fragment struct {
	ID               string
	AdapterID        string
	PayloadEncrypted []byte
	EncryptedDEK     []byte
	Status           string
}

// GetPendingFragments retrieves all fragments with status 'pending'.
func (s *Store) GetPendingFragments(limit int) ([]Fragment, error) {
	rows, err := s.db.Query(`
		SELECT id, adapter_id, payload_encrypted, encrypted_dek, status 
		FROM fragments WHERE status = 'pending' LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fragments []Fragment
	for rows.Next() {
		var f Fragment
		if err := rows.Scan(&f.ID, &f.AdapterID, &f.PayloadEncrypted, &f.EncryptedDEK, &f.Status); err != nil {
			return nil, err
		}
		fragments = append(fragments, f)
	}
	return fragments, nil
}

// UpdateFragmentsStatus updates the status of multiple fragments.
func (s *Store) UpdateFragmentsStatus(ids []string, status string) error {
	if len(ids) == 0 {
		return nil
	}
	// Simplified for brevity, usually should use a transaction and prepare statement
	for _, id := range ids {
		if _, err := s.db.Exec("UPDATE fragments SET status = ? WHERE id = ?", status, id); err != nil {
			return err
		}
	}
	return nil
}

// SavePackage persists a generated JSON package for delivery.
func (s *Store) SavePackage(id, payloadJSON, idempotencyKey string) error {
	_, err := s.db.Exec(`
		INSERT INTO packages (id, payload_json, status, idempotency_key, created_at)
		VALUES (?, ?, 'pending', ?, CURRENT_TIMESTAMP)`,
		id, payloadJSON, idempotencyKey)
	return err
}

// Package represents a JSON package ready for delivery.
type Package struct {
	ID             string
	PayloadJSON    string
	Status         string
	IdempotencyKey string
}

// GetPendingPackages retrieves packages with status 'pending'.
func (s *Store) GetPendingPackages(limit int) ([]Package, error) {
	rows, err := s.db.Query(`
		SELECT id, payload_json, status, idempotency_key 
		FROM packages WHERE status = 'pending' LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.ID, &p.PayloadJSON, &p.Status, &p.IdempotencyKey); err != nil {
			return nil, err
		}
		packages = append(packages, p)
	}
	return packages, nil
}

// UpdatePackageStatus updates the status of a package.
func (s *Store) UpdatePackageStatus(id, status string) error {
	_, err := s.db.Exec("UPDATE packages SET status = ? WHERE id = ?", status, id)
	return err
}

// MoveToDLQ moves a failed record to the Dead Letter Queue.
func (s *Store) MoveToDLQ(id, refID, refType, reason string) error {
	_, err := s.db.Exec(`
		INSERT INTO dlq (id, reference_id, reference_type, error_reason, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		id, refID, refType, reason)
	return err
}
