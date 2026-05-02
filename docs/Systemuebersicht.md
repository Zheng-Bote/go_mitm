# Systemübersicht: MitM Aggregator

Dieses Dokument bietet einen Überblick über das Systemdesign und den Datenfluss des Man-in-the-Middle (MitM) Aggregators.

## 1. Systemkontext (C4 Model - Level 1)

Der Aggregator fungiert als sichere Zwischenschicht zwischen verschiedenen Datenquellen und einer Ziel-SaaS-Plattform.

```mermaid
graph TD
    User([Endanwender / Quellsysteme])
    subgraph "Unternehmensnetzwerk"
        Aggregator[MitM Aggregator]
        DB[(Lokale SQLite DB)]
    end
    SaaS[Ziel SaaS Plattform]

    User -- "CSV, API, SQL" --> Aggregator
    Aggregator -- "Speichert verschlüsselt" --> DB
    Aggregator -- "Täglicher Batch (REST)" --> SaaS
```

## 2. Container-Ansicht (C4 Model - Level 2)

Detailliertere Ansicht der internen Komponenten der Go-Applikation.

```mermaid
graph TB
    subgraph "MitM Aggregator (Go Binary)"
        Ingest[Ingest Service]
        Crypto[Crypto Module]
        Storage[Storage Module]
        Delivery[Delivery Service]
        Monitor[Monitor & Metrics]
    end

    Sources[(Datenquellen)]
    SQLite[(SQLite DB)]
    SaaS_API[SaaS REST API]
    Prometheus[Prometheus / Grafana]

    Sources --> Ingest
    Ingest -- "Nutzt" --> Crypto
    Ingest -- "Persistiert" --> Storage
    Storage <--> SQLite
    
    Delivery -- "Liest Fragments" --> Storage
    Delivery -- "Entschlüsselt" --> Crypto
    Delivery -- "POST JSON" --> SaaS_API
    
    Ingest & Delivery -- "Metrics" --> Monitor
    Monitor -- ":8080/metrics" --> Prometheus
```

## 3. Datenfluss: Ingest & Encryption

Der Prozess der Datenaufnahme und der **Envelope Encryption**.

```mermaid
sequenceDiagram
    participant S as Quelle (z.B. CSV)
    participant I as Ingest Service
    participant C as Crypto Module
    participant DB as SQLite Storage

    S->>I: Rohdaten (PII)
    I->>C: Generate DEK
    C-->>I: DEK (Plain)
    I->>C: Encrypt Data with DEK
    C-->>I: Encrypted Data
    I->>C: Encrypt DEK with MasterKey (KEK)
    C-->>I: Encrypted DEK
    I->>DB: Store (EncData + EncDEK)
    Note over I,DB: Daten sind at-rest sicher
```

## 4. Datenfluss: Packaging & Delivery

Der tägliche Prozess der Aggregation und Übertragung.

```mermaid
sequenceDiagram
    participant DB as SQLite Storage
    participant D as Delivery Service
    participant C as Crypto Module
    participant SaaS as SaaS API

    D->>DB: Get Pending Fragments
    DB-->>D: List of EncFragments
    loop Pro Fragment
        D->>C: Decrypt DEK with KEK
        C-->>D: DEK (Plain)
        D->>C: Decrypt Data with DEK
        C-->>D: Plain Data
    end
    D->>D: Aggregiere zu JSON Package
    D->>DB: Save Package
    D->>SaaS: POST JSON (with Idempotency Key)
    SaaS-->>D: 202 Accepted
    D->>DB: Update Status: Delivered
```

## 5. Monitoring & Betrieb

- **Health Checks:**
  - `/healthz`: Liveness Probe (App läuft).
  - `/readyz`: Readiness Probe (DB-Verbindung steht).
- **Metriken:**
  - `mitm_fragments_ingested_total`: Anzahl aufgenommener Datensätze.
  - `mitm_packages_delivered_total`: Erfolgreiche Übertragungen.
  - `mitm_delivery_failures_total`: Fehlgeschlagene Übertragungen (Retries/DLQ).
