/**
 * SPDX-FileComment: Delivery Service Tests
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file delivery_test.go
 * @brief Unit tests for Packaging and Delivery
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Zheng-Bote/go_mitm/internal/crypto"
	"github.com/Zheng-Bote/go_mitm/internal/storage"
)

func TestPackagingAndDelivery(t *testing.T) {
	// 1. Setup
	dbPath := "test_delivery.db"
	defer os.Remove(dbPath)

	store, _ := storage.NewStore(dbPath)
	defer store.Close()

	masterKey, _ := crypto.GenerateKey()

	// Seed fragments
	dek, _ := crypto.GenerateKey()
	payload, _ := crypto.Encrypt(dek, []byte(`{"user":"john","val":42}`))
	encDEK, _ := crypto.Encrypt(masterKey, dek)
	store.SaveFragment("f1", "adapter1", payload, encDEK)

	// 2. Mock SaaS API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("X-Idempotency-Key") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	svc := NewService(store, masterKey, server.URL, "secret_token")

	// 3. Test Packaging
	err := svc.PackageFragments(context.Background(), 10)
	if err != nil {
		t.Fatalf("PackageFragments failed: %v", err)
	}

	pkgs, _ := store.GetPendingPackages(10)
	if len(pkgs) != 1 {
		t.Errorf("Expected 1 package, got %d", len(pkgs))
	}

	// Verify fragment status
	var status string
	store.GetDB().QueryRow("SELECT status FROM fragments WHERE id='f1'").Scan(&status)
	if status != "packaged" {
		t.Errorf("Expected fragment status 'packaged', got %s", status)
	}

	// 4. Test Delivery
	err = svc.DeliverPackages(context.Background())
	if err != nil {
		t.Fatalf("DeliverPackages failed: %v", err)
	}

	// Verify package status
	store.GetDB().QueryRow("SELECT status FROM packages WHERE id=?", pkgs[0].ID).Scan(&status)
	if status != "delivered" {
		t.Errorf("Expected package status 'delivered', got %s", status)
	}
}
