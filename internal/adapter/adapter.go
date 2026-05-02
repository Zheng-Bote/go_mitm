/**
 * SPDX-FileComment: Adapter Interface Definition
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file adapter.go
 * @brief Common interfaces and structures for data source adapters
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package adapter

import "context"

// Fragment represents a raw data unit from a source.
type Fragment struct {
	ID        string
	AdapterID string
	RawData   []byte
}

// Poller is the interface that all source system adapters must implement.
type Poller interface {
	// Init configures the adapter.
	Init(ctx context.Context, config map[string]interface{}) error
	// Poll retrieves new data based on the last cursor.
	// Returns a list of fragments, the new cursor value, and an error if any.
	Poll(ctx context.Context, lastCursor string) ([]Fragment, string, error)
	// Name returns the unique identifier of the adapter.
	Name() string
}
