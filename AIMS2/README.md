# AIMS2 application

This directory contains the Wails desktop application for AIMS2. The backend is written in Go, the frontend is built with Vite, and inventory data is stored in the local SQLite files under `importer/`.

## Main pieces

- `main.go` - Wails application entry point
- `app.go` - bindings exposed to the frontend
- `handler.go` - chat request flow and tool-call loop
- `tools.go` - AI tool registry for inventory lookups, notes, vendor prioritization, and web search
- `importer/` - spreadsheet import, database access, search, and notes
- `frontend/` - desktop UI assets and JavaScript

## Development

Run commands from this directory:

```bash
go test ./...
```

```bash
cd frontend
npm install
npm run build
```

```bash
wails dev
```

## Build

To create a production desktop bundle:

```bash
wails build
```

## Notes

- The importer opens `./importer/inventory.db`, so the working directory matters for tests and local runs.
- The frontend build is wired through `wails.json` with `npm install` and `npm run build`.
- The current Go test suite depends on local importer database access and presently fails when that database cannot be opened by the test environment.
