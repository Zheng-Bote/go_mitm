# Glossar

| Term | Deutsche Erläuterung |
| :--- | :--- |
| **KEK** | Key Encryption Key (Master Key) – Der Hauptschlüssel zur Verschlüsselung der DEKs. Wird nie auf der Festplatte gespeichert, nur zur Laufzeit im RAM gehalten. |
| **DEK** | Data Encryption Key – Individueller Schlüssel für jedes Datenfragment. Mit dem KEK verschlüsselt in der Datenbank gespeichert. |
| **Fragment** | Kleinste Dateneinheit aus einer Quelle (z. B. eine Zeile einer CSV-Datei). |
| **Package** | Aggregation mehrerer Fragmente zu einem JSON-Dokument für den SaaS-Versand. |
| **Envelope Encryption** | Sicherheitskonzept bei dem Daten mit einem DEK verschlüsselt werden und der DEK wiederum mit einem KEK. |
| **Adapter** | Schnittstellenmodul zur Anbindung von Datenquellen (CSV, REST, SQL, etc.). |
| **DLQ** | Dead Letter Queue – Ablage für dauerhaft fehlgeschlagene Datensätze. |
| **Ingest** | Phase des Einlesens und Verschlüsselns von Datenfragmenten aus Quellsystemen. |
| **Delivery** | Phase des Sendens aggregierter JSON-Pakete an die Ziel-SaaS-Plattform. |
| **WAL-Mode** | Write-Ahead Logging – SQLite-Modus für bessere Schreibperformance und Resilienz. |
| **Idempotency** | Eigenschaft einer API-Operation, die bei mehrfacher Ausführung das gleiche Ergebnis liefert. |
| **Cursors** | Fortschrittsmarken um nur neue Daten seit dem letzten Lauf zu laden (verhindert Duplikate). |