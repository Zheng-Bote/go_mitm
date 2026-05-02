# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-05-02

### Added
- **Core Services**: Implemented Ingest, Storage (SQLite), Delivery, and Monitoring components
- **Envelope Encryption**: AES-GCM based encryption with KEK/DEK key management
- **CSV Adapter**: Source adapter for CSV file ingestion
- **Prometheus Metrics**: `/metrics` endpoint for system monitoring
- **Health Endpoints**: `/healthz` (liveness) and `/readyz` (readiness) probes
- **HTTP Server**: Built-in HTTP server for metrics and health endpoints on port 8080
- **Structured Logging**: JSON logging via `zerolog`
- **Retry Logic**: Exponential backoff for resilient API delivery

### Changed
- Upgraded to Go 1.24+
- SQLite configured with WAL-mode for improved concurrency

### Security
- Master Key (KEK) is never persisted to disk - only loaded from environment at runtime
- Each data fragment encrypted with its own unique DEK

## [0.1.0] - 2026-04-20

### Added
- Initial project setup
- Architecture documentation (arc42 based)
- Python prototype (`mvp_0/`) for SaaS API validation