/**
 * SPDX-FileComment: CSV Adapter Implementation
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file csv_adapter.go
 * @brief Adapter for reading data from CSV files
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package adapter

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/google/uuid"
)

type CSVAdapter struct {
	id       string
	filePath string
}

func NewCSVAdapter(id string) *CSVAdapter {
	return &CSVAdapter{id: id}
}

func (a *CSVAdapter) Init(ctx context.Context, config map[string]interface{}) error {
	path, ok := config["file_path"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'file_path' in config")
	}
	a.filePath = path
	return nil
}

func (a *CSVAdapter) Poll(ctx context.Context, lastCursor string) ([]Fragment, string, error) {
	file, err := os.Open(a.filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	
	// Skip header
	header, err := reader.Read()
	if err != nil {
		return nil, "", fmt.Errorf("failed to read CSV header: %w", err)
	}

	startRow := 0
	if lastCursor != "" {
		startRow, _ = strconv.Atoi(lastCursor)
	}

	var fragments []Fragment
	currentRow := 0
	
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to read CSV row at %d: %w", currentRow, err)
		}

		currentRow++
		if currentRow <= startRow {
			continue
		}

		// Convert CSV record to map for normalization
		data := make(map[string]string)
		for i, val := range record {
			if i < len(header) {
				data[header[i]] = val
			}
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal CSV row to JSON: %w", err)
		}

		fragments = append(fragments, Fragment{
			ID:        uuid.New().String(),
			AdapterID: a.id,
			RawData:   jsonData,
		})
	}

	return fragments, strconv.Itoa(currentRow), nil
}

func (a *CSVAdapter) Name() string {
	return a.id
}
