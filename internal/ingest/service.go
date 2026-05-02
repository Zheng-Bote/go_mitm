/**
 * SPDX-FileComment: Ingest Service Implementation
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file service.go
 * @brief Orchestrates data ingestion, encryption, and storage
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
	"fmt"

	"github.com/Zheng-Bote/go_mitm/internal/adapter"
	"github.com/Zheng-Bote/go_mitm/internal/crypto"
	"github.com/Zheng-Bote/go_mitm/internal/monitor"
	"github.com/Zheng-Bote/go_mitm/internal/storage"
	"github.com/rs/zerolog/log"
)

type Service struct {
	store     *storage.Store
	masterKey []byte
}

func NewService(store *storage.Store, masterKey []byte) *Service {
	return &Service{
		store:     store,
		masterKey: masterKey,
	}
}

// ProcessAdapter performs a single poll-encrypt-store cycle for an adapter.
func (s *Service) ProcessAdapter(ctx context.Context, poller adapter.Poller) error {
	adapterID := poller.Name()
	log.Info().Str("adapter", adapterID).Msg("Starting ingestion")

	// 1. Ensure adapter is registered
	if err := s.store.RegisterAdapter(adapterID, adapterID); err != nil {
		return fmt.Errorf("failed to register adapter: %w", err)
	}

	// 2. Get last cursor
	cursor, err := s.store.GetCursor(adapterID)
	if err != nil {
		return fmt.Errorf("failed to get cursor: %w", err)
	}

	// 3. Poll for fragments
	fragments, newCursor, err := poller.Poll(ctx, cursor)
	if err != nil {
		return fmt.Errorf("polling failed: %w", err)
	}

	if len(fragments) == 0 {
		log.Info().Str("adapter", adapterID).Msg("No new data found")
		return nil
	}

	log.Info().Str("adapter", adapterID).Int("count", len(fragments)).Msg("Processing fragments")

	// 4. Encrypt and Store each fragment
	for _, frag := range fragments {
		// Generate DEK
		dek, err := crypto.GenerateKey()
		if err != nil {
			return fmt.Errorf("failed to generate DEK: %w", err)
		}

		// Encrypt Payload with DEK
		encPayload, err := crypto.Encrypt(dek, frag.RawData)
		if err != nil {
			return fmt.Errorf("failed to encrypt payload: %w", err)
		}

		// Encrypt DEK with MasterKey (KEK)
		encDEK, err := crypto.Encrypt(s.masterKey, dek)
		if err != nil {
			return fmt.Errorf("failed to encrypt DEK: %w", err)
		}

		// Store in DB
		if err := s.store.SaveFragment(frag.ID, frag.AdapterID, encPayload, encDEK); err != nil {
			return fmt.Errorf("failed to save fragment: %w", err)
		}
		monitor.FragmentsIngested.WithLabelValues(adapterID).Inc()
	}

	// 5. Update Cursor
	if err := s.store.UpdateCursor(adapterID, newCursor); err != nil {
		return fmt.Errorf("failed to update cursor: %w", err)
	}

	log.Info().Str("adapter", adapterID).Str("cursor", newCursor).Msg("Ingestion completed successfully")
	return nil
}
