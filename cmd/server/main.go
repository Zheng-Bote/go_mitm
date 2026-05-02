/**
 * SPDX-FileComment: MitM Aggregator Server Entry Point
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file main.go
 * @brief Server Entry Point and Sprint Demo
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"syscall"

	"github.com/Zheng-Bote/go_mitm/internal/adapter"
	"github.com/Zheng-Bote/go_mitm/internal/crypto"
	"github.com/Zheng-Bote/go_mitm/internal/delivery"
	"github.com/Zheng-Bote/go_mitm/internal/ingest"
	"github.com/Zheng-Bote/go_mitm/internal/monitor"
	"github.com/Zheng-Bote/go_mitm/internal/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	log.Info().Msg("Starting MitM Aggregator Sprint 3")

	// 1. Get Master Key (KEK) from environment
	kekBase64 := os.Getenv("MASTER_KEY")
	if kekBase64 == "" {
		log.Warn().Msg("MASTER_KEY not set in environment. Generating a temporary one for demo.")
		key, _ := crypto.GenerateKey()
		kekBase64 = base64.StdEncoding.EncodeToString(key)
	}

	kek, err := base64.StdEncoding.DecodeString(kekBase64)
	if err != nil || len(kek) != 32 {
		log.Fatal().Err(err).Msg("Invalid MASTER_KEY. Must be 32-byte base64 encoded.")
	}

	// 2. Mock SaaS API for Demo
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Str("path", r.URL.Path).Str("idempotency", r.Header.Get("X-Idempotency-Key")).Msg("Mock SaaS received request")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	// 3. Initialize Storage
	dbPath := "mitm.db"
	store, err := storage.NewStore(dbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer store.Close()

	// 4. Initialize Services
	ingestSvc := ingest.NewService(store, kek)
	deliverySvc := delivery.NewService(store, kek, server.URL, "demo_token")
	mon := monitor.NewMonitor(store)

	// 5. Start HTTP Server for Metrics and Health
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", mon.HealthHandler)
	mux.HandleFunc("/readyz", mon.ReadyHandler)
	mux.Handle("/metrics", mon.MetricsHandler())

	go func() {
		log.Info().Msg("Monitoring server starting on :8080")
		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Error().Err(err).Msg("Monitoring server failed")
		}
	}()

	// 6. Run Ingestion Cycle
	csvAdapter := adapter.NewCSVAdapter("csv_adapter_01")
	csvAdapter.Init(context.Background(), map[string]interface{}{"file_path": "mvp_0/sample.csv"})
	
	if _, err := os.Stat("mvp_0/sample.csv"); os.IsNotExist(err) {
		os.MkdirAll("mvp_0", 0755)
		os.WriteFile("mvp_0/sample.csv", []byte("id,name,value\n1,Demo,42\n2,Test,99\n"), 0644)
	}

	if err := ingestSvc.ProcessAdapter(context.Background(), csvAdapter); err != nil {
		log.Error().Err(err).Msg("Ingestion cycle failed")
	}

	// 6. Run Packaging Cycle
	if err := deliverySvc.PackageFragments(context.Background(), 100); err != nil {
		log.Error().Err(err).Msg("Packaging cycle failed")
	}

	// 8. Run Delivery Cycle
	if err := deliverySvc.DeliverPackages(context.Background()); err != nil {
		log.Error().Err(err).Msg("Delivery cycle failed")
	}

	log.Info().Msg("Sprint 4 demo: System is operational. Press Ctrl+C to stop.")

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info().Msg("Shutting down...")
}
