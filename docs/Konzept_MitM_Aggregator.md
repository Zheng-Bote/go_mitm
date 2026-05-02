# Draft-Konzept: Man-in-the-Middle (MitM) Daten-Aggregator

## 1. Ziel und Kontext

**Zweck:** Bereitstellung eines zuverlässigen, sicheren und entkoppelten Systems (MitM), das Daten aus diversen Quellsystemen einsammelt, zwischenspeichert, täglich zu JSON-Paketen aggregiert und über eine REST-Schnittstelle an eine Ziel-SaaS-Lösung überträgt.

**Stakeholder:**

- Quellsystem-Verantwortliche (Datenbereitstellung)
- SaaS-Anbieter (Datenabnahme)
- IT-Betrieb / Admins (Deployment & Monitoring)
- Security & Compliance (Datenschutz)

**Nicht-funktionale Anforderungen:**

- **SLA & Latenz:** Tägliche asynchrone Batch-Verarbeitung. Latenz der Einzel-Events ist nachrangig (24h Fenster).
- **Datenschutz:** **Alle PII-Daten (Personally Identifiable Information) müssen at-rest verschlüsselt werden** (Envelope Encryption).
- **Durchsatz:** Skalierbar für Millionen von Fragmenten pro Tag, aggregiert in handhabbaren JSON-Packages.
- **Verfügbarkeit:** Fokus auf Resilienz und Retry-Fähigkeit anstatt auf 99.999% Uptime, da tägliche Fenster genügend Zeit für automatische Retries bieten.

## 2. Bounded Contexts und Komponenten

- **Ingest (Adapter / Poller):** Verantwortung für das Anbinden heterogener Quellen (CSV, APIs, DBs). Holt Daten ab und normalisiert sie in ein "Fragment".
- **State (Storage):** Speichert den Fortschritt (Cursors) und die asynchron gepufferten Fragmente persistent und sicher ab (SQLite lokal + lokales Filesystem/Blob).
- **Transform / Package:** Ein Scheduler-gesteuerter Job, der rohe Fragmente liest, dechiffriert, in das Ziel-JSON-Format transformiert und als "Package" speichert.
- **Delivery (Sender):** Nimmt fertige Packages und sendet sie per REST an die SaaS. Behandelt Rate Limits, Retries und Dead Letter Queue (DLQ) bei persistenten Fehlern.
- **Security:** Querschnittskomponente für Verschlüsselung (Envelope Encryption), Key Management und Audit-Logging.

## 3. Schnittstellen

- **Adapter-Interface:** Definition, wie Poller Daten lesen und in Fragmente überführen (siehe Go-Code-Artefakte).
- **REST-Contract zur SaaS:** Übermittlung per POST-Request inkl. Authentifizierung und Idempotency-Schlüssel.
- **DB-Schema:** Zustandshaltung via SQLite (siehe SQL DDL Artefakte).
- **DLQ & Audit:** Tabellen für nicht zustellbare Payloads (DLQ) und ein Write-Only Audit-Log für sicherheitsrelevante Events.

## 4. Datenfluss und Workflow

1. **Scheduler triggert Poller:** Abhängig vom Quell-System fragt ein Adapter periodisch neue Daten ab (unter Nutzung des letzten `Cursors`).
2. **Fragment-Erzeugung:** Jede Dateneinheit wird normalisiert, via DEK (Data Encryption Key) verschlüsselt und in der `fragments`-Tabelle gespeichert.
3. **Packaging:** Ein täglicher Cron-Job liest offene Fragmente, generiert JSON-Payloads und speichert diese in der `packages`-Tabelle. Status der Fragmente wechselt auf "packaged".
4. **Delivery:** Der Delivery-Worker liest `packages`, sendet sie an die SaaS.
   - **Erfolg:** Package-Status wird auf `delivered` gesetzt.
   - **Temporärer Fehler (z.B. 429, 503):** Exponential Backoff & Retry.
   - **Permanenter Fehler (z.B. 400):** Verschiebung in die DLQ für manuelles Review/Replay.
5. **Backfill/Replay:** Administratoren können Cursors zurücksetzen oder DLQ-Einträge re-injizieren.

## 5. Deployment und Betrieb

- **Container-Image:** Single Binary Go-App im distroless oder Alpine-Image (minimaler Footprint).
- **Laufzeit:** Docker-Container auf dem Admin-Host (AWS EC2). Kann später problemlos nach Kubernetes migriert werden.
- **CI/CD:** GitHub Enterprise Server (GHES) Actions für automatisierte Builds, Tests und Container-Pushes.
- **Backup-Strategie:** **Regelmäßige Snapshots des SQLite-Files und der Blob-Volumes**. Die Backups sind verschlüsselt in AWS S3 abzulegen.
- **Monitoring & Logging:** `prometheus_client` exportiert `/metrics` (Queue-Größen, Fehler). Strukturiertes Logging im JSON-Format via `zerolog`.
- **Health Checks:** `/healthz` (App-Status) und `/readyz` (DB-Verbindung, Key-Verfügbarkeit).

## 6. Security und Key Management

- **Envelope Encryption:** Jedes Fragment bekommt einen generierten DEK (Data Encryption Key). Der DEK wird mit dem KEK (Key Encryption Key / MasterKey) verschlüsselt gespeichert.
- **MasterKey Handling:** **Der KEK darf niemals persistiert werden**. Er wird via GHES Secrets als Umgebungsvariable oder zur Laufzeit aus einem AWS Secrets Manager/Vault in den Container injiziert.
- **TLS:** Alle Verbindungen nach außen (SaaS, AWS-Dienste) erzwingen TLS 1.2+.
- **Audit:** Ein Audit-Log protokolliert Key-Rotations, App-Starts und fehlgeschlagene Auth-Versuche.
- **Key Rotation:** Der KEK wird regelmäßig rotiert; bestehende verschlüsselte DEKs können im Batch-Verfahren mit dem neuen KEK umgeschlüsselt werden.

## 7. Toolchain und OSS Bibliotheken (MIT / Apache 2.0)

- **Sprache:** Go (effizient, typensicher, Single-Binary).
- **DB Driver:** `github.com/mattn/go-sqlite3` (lokale Persistenz).
- **HTTP Client:** `github.com/hashicorp/go-retryablehttp` (robuste Retries).
- **Logging:** `github.com/rs/zerolog`.
- **Monitoring:** `github.com/prometheus/client_golang`.
- **Verschlüsselung / Ops:** Standard Go `crypto/aes` + `crypto/cipher` (AES-GCM). `sops`/`age` für das Verschlüsseln von Infrastruktur-Configs.

## 8. Risiken und Gegenmaßnahmen

| Risiko                        | Gegenmaßnahme                                                                            |
| :---------------------------- | :--------------------------------------------------------------------------------------- |
| **Datenschutz / Key Leakage** | Envelope Encryption; KEK nur im RAM; keine PII in Logs.                                  |
| **Schema Drift der Quellen**  | Versionierte Adapter; robuste Fallbacks; DLQ bei Parsing-Fehlern.                        |
| **Rate Limits der SaaS**      | Politeness-Delays; `go-retryablehttp` fängt 429er ab und respektiert `Retry-After`.      |
| **SQLite Concurrency**        | **WAL-Mode (Write-Ahead Logging) aktivieren**; ein Schreib-Thread, mehrere Lese-Threads. |

## 9. MVP Roadmap

- **Sprint 0: SaaS API.** Evaluierung der SaaS API(s) und deren Laufzeitverhalten mittels Prototyp uploader.py.
- **Sprint 1: Core & Storage.** Go-Setup, SQLite-Schema inkl. WAL, Envelope-Encryption Logik.
- **Sprint 2: Ingest.** Erster lokaler CSV-Adapter, Cursor-Management, Fragment-Generierung.
- **Sprint 3: Delivery.** Packaging-Logik, Mock-SaaS REST-Client, Retries, Idempotency-Headers.
- **Sprint 4: Ops & Sec.** CI/CD GitHub Actions, Prometheus-Metriken, DLQ-Handling, Dokumentation.

---

# Artefakte

### ERD (Vereinfacht)

| Tabelle     | Primärschlüssel   | Wichtige Spalten                                               |
| :---------- | :---------------- | :------------------------------------------------------------- |
| `adapters`  | `id` (PK)         | `name`, `status`, `config_json`                                |
| `cursors`   | `adapter_id` (PK) | `last_value`, `updated_at`                                     |
| `fragments` | `id` (PK)         | `adapter_id`, `payload_encrypted`, `encrypted_dek`, `status`   |
| `packages`  | `id` (PK)         | `payload_json`, `status`, `idempotency_key`, `created_at`      |
| `dlq`       | `id` (PK)         | `reference_id`, `reference_type`, `error_reason`, `created_at` |
| `audit_log` | `id` (PK)         | `action`, `actor`, `timestamp`, `details`                      |

### Dateisystem Layout

```text
/opt/mitm-app/
├── bin/
│   └── mitm-server          # Kompiliertes Go-Binary
├── config/
│   └── config.yaml          # Basis-Konfiguration (ohne Secrets!)
├── data/
│   ├── db/
│   │   ├── mitm.db          # SQLite Datenbank
│   │   ├── mitm.db-wal      # Write-Ahead Log
│   │   └── mitm.db-shm      # Shared Memory
│   └── blobs/               # (Optional) Für sehr große Fragmente, verschlüsselt gespeichert
│       ├── 2026/05/01/
│       │   └── frag_12345.enc
└── logs/
    └── application.log      # JSON formatierte Logs (zerolog)
```

### SQLite Schema (SQL DDL)

```sql
PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS adapters (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS cursors (
    adapter_id TEXT PRIMARY KEY,
    last_value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(adapter_id) REFERENCES adapters(id)
);

CREATE TABLE IF NOT EXISTS fragments (
    id TEXT PRIMARY KEY,
    adapter_id TEXT NOT NULL,
    payload_encrypted BLOB NOT NULL,
    encrypted_dek BLOB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, packaged
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(adapter_id) REFERENCES adapters(id)
);

CREATE TABLE IF NOT EXISTS packages (
    id TEXT PRIMARY KEY,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, delivered, failed
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS dlq (
    id TEXT PRIMARY KEY,
    reference_id TEXT NOT NULL,
    reference_type TEXT NOT NULL, -- 'fragment' or 'package'
    error_reason TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_log (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    details TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Adapter API Spec (Go Pseudocode)

```go
package adapter

import "context"

type Fragment struct {
    ID           string
    AdapterID    string
    RawData      []byte
    // Weitere Metadaten nach Bedarf
}

// Poller ist das Interface, das alle Quell-System-Adapter implementieren müssen.
type Poller interface {
    // Init konfiguriert den Adapter (Credentials, Pfade)
    Init(ctx context.Context, config map[string]interface{}) error
    // Poll holt neue Daten basierend auf dem letzten Cursor
    Poll(ctx context.Context, lastCursor string) ([]Fragment, string, error)
    // Name gibt die eindeutige ID des Adapters zurück
    Name() string
}
```

### Delivery Contract

**POST Request:**

```http
POST /api/v1/ingest/packages HTTP/1.1
Host: saas.example.com
Content-Type: application/json
Authorization: Bearer <SaaS_API_TOKEN>
X-Idempotency-Key: pkg_abc123xyz890
X-Source-System: MitM-Aggregator-AWS

{
  "package_id": "pkg_abc123xyz890",
  "timestamp": "2026-05-01T12:00:00Z",
  "records": [
    {
      "source_id": "adapter_csv_01",
      "data": { "user": "test", "value": 42 }
    },
    {
      "source_id": "adapter_api_02",
      "data": { "user": "demo", "value": 99 }
    }
  ]
}
```

**Response (Success):**

```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{
  "status": "success",
  "message": "Package accepted for processing"
}
```

### Security Flow Diagram

1. **Start:** Applikation startet, erhält `MASTER_KEY` (KEK) via Environment von GHES/Vault in den RAM.
2. **Ingest:** Neues Datum trifft ein.
3. **DEK Gen:** App generiert einen zufälligen 32-Byte DEK (Data Encryption Key) für dieses Fragment.
4. **Encrypt Payload:** Datum wird via AES-GCM und DEK verschlüsselt (`payload_encrypted`).
5. **Encrypt DEK:** DEK wird via AES-GCM und dem `MASTER_KEY` verschlüsselt (`encrypted_dek`).
6. **Store:** `payload_encrypted` und `encrypted_dek` werden in SQLite gespeichert. DEK wird aus dem RAM gelöscht.
7. **Read & Send:** Zum Packen liest die App `encrypted_dek`, entschlüsselt ihn mit dem `MASTER_KEY` zum DEK, entschlüsselt damit `payload_encrypted` und verwirft den DEK sofort wieder.
8. **Rotate (Optional):** Neuer `MASTER_KEY` wird bereitgestellt; Batch-Job entschlüsselt alle `encrypted_dek` mit dem alten KEK und verschlüsselt sie mit dem neuen KEK.

### CI/CD Snippet (GHES Actions YAML)

```yaml
name: Build and Run MitM

on:
  push:
    branches: ["main"]

jobs:
  build-and-run:
    runs-on: self-hosted # Läuft auf dem dedizierten Admin-Host
    steps:
      - name: Check out code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Build Binary
        run: go build -o bin/mitm-server ./cmd/server

      - name: Build Container
        run: docker build -t mitm-server:latest .

      - name: Run Container locally
        run: |
          docker stop mitm-app || true
          docker rm mitm-app || true
          docker run -d \
            --name mitm-app \
            -v /opt/mitm/data:/app/data \
            -e MASTER_KEY="${{ secrets.MITM_MASTER_KEY }}" \
            -e DB_PATH="/app/data/db/mitm.db" \
            mitm-server:latest
```

### Minimaler Prototyp (Go Code)

```go
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := generateRandomBytes(gcm.NonceSize())
	if err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, err
	}
	schema := `
	CREATE TABLE IF NOT EXISTS fragments (
		id TEXT PRIMARY KEY,
		adapter_id TEXT,
		payload_encrypted BLOB,
		encrypted_dek BLOB,
		created_at DATETIME
	);`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	masterKeyB64 := os.Getenv("MASTER_KEY")
	dbPath := os.Getenv("DB_PATH")
	csvPath := os.Getenv("CSV_PATH")

	if masterKeyB64 == "" || dbPath == "" || csvPath == "" {
		log.Fatal("Environment Variablen MASTER_KEY, DB_PATH und CSV_PATH müssen gesetzt sein.")
	}

	masterKey, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil || len(masterKey) != 32 {
		log.Fatal("MASTER_KEY muss valides Base64 und entschlüsselt 32 Bytes lang sein (AES-256).")
	}

	db, err := InitDB(dbPath)
	if err != nil {
		log.Fatalf("Fehler bei DB Initialisierung: %v", err)
	}
	defer db.Close()

	file, err := os.Open(csvPath)
	if err != nil {
		log.Fatalf("Fehler beim Öffnen der CSV: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	adapterID := "csv_importer_01"

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Fehler beim Lesen der CSV Zeile: %v", err)
			continue
		}

		payload := []byte(fmt.Sprintf("%v", record))
		dek, err := generateRandomBytes(32)
		if err != nil {
			log.Fatalf("Fehler bei DEK Generierung: %v", err)
		}
		encPayload, err := encrypt(dek, payload)
		if err != nil {
			log.Fatalf("Fehler bei Payload-Verschlüsselung: %v", err)
		}
		encDEK, err := encrypt(masterKey, dek)
		if err != nil {
			log.Fatalf("Fehler bei DEK-Verschlüsselung: %v", err)
		}

		fragID := fmt.Sprintf("frag_%d", time.Now().UnixNano())
		_, err = db.Exec(`INSERT INTO fragments (id, adapter_id, payload_encrypted, encrypted_dek, created_at) VALUES (?, ?, ?, ?, ?)`,
			fragID, adapterID, encPayload, encDEK, time.Now())
		if err != nil {
			log.Printf("Fehler beim Speichern in SQLite: %v", err)
		} else {
			log.Printf("Fragment %s erfolgreich verschlüsselt und gespeichert.", fragID)
		}
	}
	log.Println("Ingest-Lauf beendet.")
}
```

---

## Akzeptanzkriterien (Checkliste)

- [ ] Struktur entspricht arc42 und deckt alle angeforderten Bereiche ab.
- [ ] PII-Datenschutz durch asymmetrisches Key Management (Envelope Encryption) ist konzeptionell verankert.
- [ ] Architektur berücksichtigt asynchrones Retry-Management und Dead-Letter-Queues für robuste SaaS-Delivery.
- [ ] Das gewählte Tech-Stack (Go, SQLite, OSS-Libs) besteht zu 100% aus offenen, lizenzkostenfreien Komponenten.
- [ ] Alle geforderten Artefakte (ERD, DDL, Go-Spec, CI/CD, Prototyp) sind integriert.
- [ ] Der Prototyp ist in Go geschrieben, baut auf OSS auf und demonstriert die KEK/DEK-Verschlüsselung mit SQLite funktionsfähig.
- [ ] Wichtige Sicherheits- und Betriebsentscheidungen sind im Text deutlich hervorgehoben.
