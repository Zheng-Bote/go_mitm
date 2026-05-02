/**
 * SPDX-FileComment: Monitor Package Implementation
 * SPDX-FileType: SOURCE
 * SPDX-FileContributor: ZHENG Robert
 * SPDX-FileCopyrightText: 2026 ZHENG Robert
 * SPDX-License-Identifier: Apache-2.0
 *
 * @file monitor.go
 * @brief Prometheus metrics and health check handlers
 * @version 0.1.0
 * @date 2026-05-02
 *
 * @author ZHENG Robert (robert @hase-zheng.net)
 * @copyright Copyright (c) 2026 ZHENG Robert
 * @LICENSE Apache-2.0
 */

package monitor

import (
	"net/http"

	"github.com/Zheng-Bote/go_mitm/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	FragmentsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mitm_fragments_ingested_total",
		Help: "The total number of ingested fragments",
	}, []string{"adapter_id"})

	PackagesDelivered = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mitm_packages_delivered_total",
		Help: "The total number of delivered packages",
	})

	DeliveryFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mitm_delivery_failures_total",
		Help: "The total number of package delivery failures",
	})
)

type Monitor struct {
	store *storage.Store
}

func NewMonitor(store *storage.Store) *Monitor {
	return &Monitor{store: store}
}

func (m *Monitor) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (m *Monitor) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if err := m.store.GetDB().Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Database not ready"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("READY"))
}

func (m *Monitor) MetricsHandler() http.Handler {
	return promhttp.Handler()
}
