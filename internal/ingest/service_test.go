/**
 * SPDX-FileComment: Ingest Service Tests
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file service_test.go
 * @brief Unit tests for the Ingest Service
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package ingest

import (
	"context"
	"encoding/csv"
	"os"
	"testing"

	"github.com/Zheng-Bote/go_mitm/internal/adapter"
	"github.com/Zheng-Bote/go_mitm/internal/crypto"
	"github.com/Zheng-Bote/go_mitm/internal/storage"
)

func TestIngestService(t *testing.T) {
	// 1. Setup
	dbPath := "test_ingest.db"
	csvPath := "test_data.csv"
	defer os.Remove(dbPath)
	defer os.Remove(csvPath)

	// Create test CSV
	f, _ := os.Create(csvPath)
	w := csv.NewWriter(f)
	w.Write([]string{"id", "name", "email"})
	w.Write([]string{"1", "John Doe", "john@example.com"})
	w.Write([]string{"2", "Jane Doe", "jane@example.com"})
	w.Flush()
	f.Close()

	store, _ := storage.NewStore(dbPath)
	defer store.Close()

	masterKey, _ := crypto.GenerateKey()
	svc := NewService(store, masterKey)

	csvAdapter := adapter.NewCSVAdapter("test_csv")
	csvAdapter.Init(context.Background(), map[string]interface{}{"file_path": csvPath})

	// 2. First Poll
	err := svc.ProcessAdapter(context.Background(), csvAdapter)
	if err != nil {
		t.Fatalf("ProcessAdapter failed: %v", err)
	}

	// Verify count
	var count int
	store.GetDB().QueryRow("SELECT COUNT(*) FROM fragments WHERE adapter_id = 'test_csv'").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 fragments, got %d", count)
	}

	// Verify cursor
	cursor, _ := store.GetCursor("test_csv")
	if cursor != "2" {
		t.Errorf("Expected cursor '2', got %s", cursor)
	}

	// 3. Second Poll (append data)
	f, _ = os.OpenFile(csvPath, os.O_APPEND|os.O_WRONLY, 0644)
	w = csv.NewWriter(f)
	w.Write([]string{"3", "Bob", "bob@example.com"})
	w.Flush()
	f.Close()

	err = svc.ProcessAdapter(context.Background(), csvAdapter)
	if err != nil {
		t.Fatalf("Second ProcessAdapter failed: %v", err)
	}

	store.GetDB().QueryRow("SELECT COUNT(*) FROM fragments WHERE adapter_id = 'test_csv'").Scan(&count)
	if count != 3 {
		t.Errorf("Expected 3 fragments total, got %d", count)
	}

	cursor, _ = store.GetCursor("test_csv")
	if cursor != "3" {
		t.Errorf("Expected cursor '3', got %s", cursor)
	}
}
