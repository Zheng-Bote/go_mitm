/**
 * SPDX-FileComment: Storage Module Tests
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file sqlite_test.go
 * @brief Unit Tests for SQLite Storage
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package storage

import (
	"os"
	"testing"
)

func TestNewStore(t *testing.T) {
	dbPath := "test_mitm.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Check if table exists
	rows, err := store.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='fragments';")
	if err != nil {
		t.Fatalf("Failed to query schema: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Errorf("Table 'fragments' was not created")
	}
}

func TestWALMode(t *testing.T) {
	dbPath := "test_wal.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	var mode string
	err = store.db.QueryRow("PRAGMA journal_mode;").Scan(&mode)
	if err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}

	if mode != "wal" {
		t.Errorf("Expected journal_mode=wal, got %s", mode)
	}
}
