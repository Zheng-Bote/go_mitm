/**
 * SPDX-FileComment: Delivery Service Implementation
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file delivery.go
 * @brief Handles packaging of fragments and delivery to SaaS API
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Zheng-Bote/go_mitm/internal/crypto"
	"github.com/Zheng-Bote/go_mitm/internal/monitor"
	"github.com/Zheng-Bote/go_mitm/internal/storage"
	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog/log"
)

type Service struct {
	store      *storage.Store
	masterKey  []byte
	saasURL    string
	saasToken  string
	httpClient *retryablehttp.Client
}

func NewService(store *storage.Store, masterKey []byte, saasURL, saasToken string) *Service {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.Logger = nil // Disable internal logging to use zerolog if needed

	return &Service{
		store:      store,
		masterKey:  masterKey,
		saasURL:    saasURL,
		saasToken:  saasToken,
		httpClient: retryClient,
	}
}

// PackageFragments decrypts pending fragments and aggregates them into a JSON package.
func (s *Service) PackageFragments(ctx context.Context, batchSize int) error {
	fragments, err := s.store.GetPendingFragments(batchSize)
	if err != nil {
		return fmt.Errorf("failed to fetch pending fragments: %w", err)
	}

	if len(fragments) == 0 {
		return nil
	}

	log.Info().Int("count", len(fragments)).Msg("Packaging fragments")

	var records []map[string]interface{}
	var fragmentIDs []string

	for _, frag := range fragments {
		// 1. Decrypt DEK
		dek, err := crypto.Decrypt(s.masterKey, frag.EncryptedDEK)
		if err != nil {
			log.Error().Str("id", frag.ID).Err(err).Msg("Failed to decrypt DEK")
			continue
		}

		// 2. Decrypt Payload
		payload, err := crypto.Decrypt(dek, frag.PayloadEncrypted)
		if err != nil {
			log.Error().Str("id", frag.ID).Err(err).Msg("Failed to decrypt payload")
			continue
		}

		// 3. Unmarshal JSON record
		var record map[string]interface{}
		if err := json.Unmarshal(payload, &record); err != nil {
			log.Error().Str("id", frag.ID).Err(err).Msg("Failed to unmarshal fragment data")
			continue
		}

		records = append(records, map[string]interface{}{
			"source_id": frag.AdapterID,
			"data":      record,
		})
		fragmentIDs = append(fragmentIDs, frag.ID)
	}

	if len(records) == 0 {
		return nil
	}

	// 4. Build SaaS Package
	packageID := uuid.New().String()
	saasPkg := map[string]interface{}{
		"package_id": packageID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"records":    records,
	}

	pkgJSON, err := json.Marshal(saasPkg)
	if err != nil {
		return fmt.Errorf("failed to marshal SaaS package: %w", err)
	}

	// 5. Store Package and update fragment status
	// Ideally this should be in a transaction
	if err := s.store.SavePackage(packageID, string(pkgJSON), "pkg_"+packageID); err != nil {
		return fmt.Errorf("failed to save package: %w", err)
	}

	if err := s.store.UpdateFragmentsStatus(fragmentIDs, "packaged"); err != nil {
		return fmt.Errorf("failed to update fragments status: %w", err)
	}

	log.Info().Str("package_id", packageID).Msg("Created new package")
	return nil
}

// DeliverPackages sends pending packages to the SaaS REST API.
func (s *Service) DeliverPackages(ctx context.Context) error {
	packages, err := s.store.GetPendingPackages(10) // Small batch for delivery
	if err != nil {
		return fmt.Errorf("failed to fetch pending packages: %w", err)
	}

	for _, pkg := range packages {
		log.Info().Str("id", pkg.ID).Msg("Delivering package")

		req, err := retryablehttp.NewRequest("POST", s.saasURL, bytes.NewBufferString(pkg.PayloadJSON))
		if err != nil {
			log.Error().Str("id", pkg.ID).Err(err).Msg("Failed to create request")
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.saasToken)
		req.Header.Set("X-Idempotency-Key", pkg.IdempotencyKey)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			log.Error().Str("id", pkg.ID).Err(err).Msg("Request failed")
			monitor.DeliveryFailures.Inc()
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
			if err := s.store.UpdatePackageStatus(pkg.ID, "delivered"); err != nil {
				log.Error().Str("id", pkg.ID).Err(err).Msg("Failed to update status to delivered")
			}
			log.Info().Str("id", pkg.ID).Int("status", resp.StatusCode).Msg("Package delivered")
			monitor.PackagesDelivered.Inc()
		} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// Permanent client error (e.g. 400 Bad Request)
			reason := fmt.Sprintf("HTTP %d", resp.StatusCode)
			log.Error().Str("id", pkg.ID).Str("reason", reason).Msg("Permanent failure, moving to DLQ")
			s.store.UpdatePackageStatus(pkg.ID, "failed")
			s.store.MoveToDLQ(uuid.New().String(), pkg.ID, "package", reason)
			monitor.DeliveryFailures.Inc()
		} else {
			// 5xx errors or other temporary issues are handled by retryablehttp
			log.Warn().Str("id", pkg.ID).Int("status", resp.StatusCode).Msg("Temporary failure or max retries reached")
			monitor.DeliveryFailures.Inc()
		}
	}

	return nil
}
