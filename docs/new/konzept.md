# Konzept: MitM Daten-Aggregator

Dieses Dokument definiert das funktionale Konzept, die strategische Ausrichtung und den operativen Workflow des MitM Daten-Aggregators. Es dient als Leitfaden für die Implementierung der Multi-Domain-Fähigkeit und der Sicherheits-Frameworks.

---

## Dokumenten-Lenkung

| VERSION    | 0.2.0                |
| :--------- | :------------------- |
| **DATE**   | 2026-05-04           |
| **STATUS** | ___Draft___          |
| **AUTHOR** | ZHENG Robert, FG-514 |
| **FILE**   | konzept.md           |



**Änderungshistorie:**
| Version | Datum | Autor | Änderungen | Status | Approver |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 0.1.0 | 20.04.2026 | ZHENG Robert | Initiale Erstellung | Draft | -/- |
| 0.2.0 | 04.05.2026 | ZHENG Robert | Konsolidierung und Optimierung der Architektur | Draft | -/- |

---

## Inhaltsverzeichnis
- [1. Projektzielsetzung](#1-projektzielsetzung)
- [2. Technologiestrategie (Neutral)](#2-technologiestrategie-neutral)
  - [2.1 Implementierungsschichten](#21-implementierungsschichten)
  - [2.2 Persistenz-Konzept](#22-persistenz-konzept)
- [3. Funktionales Design](#3-funktionales-design)
  - [3.1 Datenmodell & Kategorisierung](#31-datenmodell--kategorisierung)
  - [3.2 Modularer Workflow](#32-modularer-workflow)
- [4. Erweiterbarkeit & Wartbarkeit](#4-erweiterbarkeit--wartbarkeit)
  - [4.1 Zukunftssichere Schnittstellen](#41-zukunftssichere-schnittstellen)
  - [4.2 Betrieb & Überwachung](#42-betrieb--überwachung)
  - [4.3 Sicherheits-Framework](#43-sicherheits-framework)
- [5. Roadmap (Phasenmodell)](#5-roadmap-phasenmodell)

---

## 1. Projektzielsetzung
Dieses Konzept beschreibt den Aufbau eines resilienten Man-in-the-Middle (MitM) Aggregators. Das System fungiert als intelligenter Puffer und Transformator zwischen heterogenen Quellsystemen und spezialisierten SaaS-Endpoints.

### Kernaufgaben
- **Sicheres Ingesting:** Abruf von Daten aus diversen Quellen (CSV, SQL, API).
- **Zustandshaltung:** Persistierung mit Fokus auf Datenintegrität und PII-Schutz.
- **Multitarget-Aggregation:** Paketierung von Daten nach Typ (HR, Org, Geo) für spezifische Ziel-Schnittstellen.
- **Zuverlässige Delivery:** Versand mit automatisiertem Fehlermanagement.

---

## 2. Technologiestrategie (Neutral)

### 2.1 Implementierungsschichten
Um Flexibilität und Robustheit zu vereinen, wird ein hybrider Ansatz verfolgt:
- **Prototyping & Validierung:** Python (schnelle Iteration, API-Mocks).
- **Kern-Aggregator (Produktion):** Typsichere Sprache (Go, Java oder C++) zur Sicherstellung der Datenkonsistenz und Performance im CI/CD-Prozess (AWS ECR Fokus).

### 2.2 Persistenz-Konzept
Je nach Einsatzszenario kann die Persistenz modular gewählt werden:
1. **Lightweight:** SQLite für lokale, dateibasierte SQL-Power (Ideal für Edge/Single-Host).
2. **Enterprise:** PostgreSQL für hohe Last und zentrale Verwaltung.
3. **Hybrid:** Metadaten in DB, große Payloads (Blobs) verschlüsselt im Dateisystem.

---

## 3. Funktionales Design

### 3.1 Datenmodell & Kategorisierung
Das System unterscheidet zwischen verschiedenen Daten-Domänen, die jeweils eigene Endpoints bedienen:

| Domäne  | Beispiel-Daten                    | Ziel-Endpoint         |
| :------ | :-------------------------------- | :-------------------- |
| **HR**  | Mitarbeiterstamm, Verträge        | `SaaS/api/hr/POST`    |
| **Org** | Abteilungsstrukturen, Hierarchien | `SaaS/api/org/PUT`    |
| **Geo** | Standorte, Raumkoordinaten        | `SaaS/api/geo/UPDATE` |

### 3.2 Modularer Workflow
1. **Poller (Source Layer):** Liest Daten ein. Nutzt Domänen-spezifische Adapter.
2. **Verschlüsselung (Security Layer):** AES-GCM Envelope Encryption (KEK/DEK).
3. **Storage (Persistence Layer):** Speichert Fragmente domänenübergreifend.
4. **Assembler (Packaging Layer):** Gruppiert Fragmente nach Domäne und Ziel-Anforderung.
5. **Dispatcher (Delivery Layer):** Überträgt Pakete an die korrekten Endpoints.

---

## 4. Erweiterbarkeit & Wartbarkeit

### 4.1 Zukunftssichere Schnittstellen
- **Quellen:** Neue Adapter können über ein standardisiertes Interface (z.B. `SourcePoller`) hinzugefügt werden.
- **Ziele:** Die Konfiguration der Ziel-APIs erfolgt über eine Registry, die Endpoints, Auth-Methoden und Schemata verwaltet.

### 4.2 Betrieb & Überwachung
- **Health-Reporting:** Status-Monitoring für jede Quell-Ziel-Verbindung einzeln.

### 4.3 Sicherheits-Framework
Das Sicherheitskonzept folgt dem "Zero Trust"-Ansatz für Daten:
- **Verschlüsselung At-Rest:** Konsequente Nutzung von **Envelope Encryption**. Jedes Fragment ist durch einen eigenen **DEK** geschützt, der wiederum vom globalen **KEK** (Master Key) umschlossen wird. Der KEK wird sicher in den RAM injiziert und niemals auf die Festplatte geschrieben.
- **Verschlüsselung In-Transit:** Erzwungene Nutzung von **TLS 1.2+** für alle API-Aufrufe. Keine Kommunikation über unverschlüsselte Kanäle.
- **Traceability:** Ein lückenloses **Audit-Log** erfasst, wer wann auf welche Daten-Domäne zugegriffen hat oder wann Schlüssel-Operationen durchgeführt wurden. Dies stellt die Konformität mit Datenschutzrichtlinien (z.B. DSGVO) sicher.

---

## 5. Roadmap (Phasenmodell)

- **Phase 1: Prototyping (Python).** Validierung der SaaS-API-Contracts für HR und Geo-Daten.
- **Phase 2: Core-Setup.** Implementierung des typsicheren Kerns und der gewählten Persistenz-Schicht.
- **Phase 3: Multi-Domain Support.** Anbindung der ersten produktiven HR- und Org-Quellen.
- **Phase 4: Optimization.** Feinjustierung der Paketgrößen und Implementierung von Parallel-Delivery für unterschiedliche Endpoints.
