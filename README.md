# AIMS2

AIMS2 is a Wails desktop prototype for internal inventory, purchasing, and sales workflows. The application combines a Go backend, a Vite frontend, a local SQLite inventory store, and AI-assisted tooling for stock lookups and item notes.

## Repository layout

- `AIMS2/` - main Wails application
- `AIMS2/frontend/` - Vite frontend used by the desktop UI
- `AIMS2/importer/` - inventory schema, workbook import code, search logic, and note storage
- `AIMS2/build/` - packaging assets for desktop builds

## Current capabilities

- Chat-style desktop interface for sales and purchasing questions
- Inventory lookups by stock number, vendor part number, or description
- Item notes with critical flags and minimum/target on-hand values
- Priority vendor reporting based on low-stock usage data
- Conversation logging to `conversation_log.jsonl`

## Requirements

- Go 1.25
- Node.js and npm
- Wails CLI for desktop development and packaging
- Access to the chat and web-search services configured by the backend

## Working on the app

All application commands should be run from `AIMS2/`:

```bash
cd AIMS2
```

### Test the Go code

```bash
go test ./...
```

### Install frontend dependencies and build

```bash
cd frontend
npm install
npm run build
```

### Run the desktop app in development

```bash
wails dev
```

### Build a distributable app

```bash
wails build
```

## Inventory data notes

- The SQLite database is stored at `AIMS2/importer/inventory.db`.
- The database schema is defined in `AIMS2/importer/schema.sql`.
- The bundled inventory workbook used by the importer is `AIMS2/importer/Inv Master UPC USQL 20251119.xlsx`.
- Importer code relies on relative paths under `./importer`, so tests and local commands need to run from `AIMS2/`.

## Current validation status

- `cd AIMS2/frontend && npm install && npm run build` succeeds.
- `cd AIMS2 && go test ./...` currently fails in `AIMS2/importer` because the database file cannot be opened in the test environment.
