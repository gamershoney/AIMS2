# AIMS2

AIMS2 is an experimental desktop inventory assistant built with Go and Wails. It combines compact inventory tools, isolated Markdown-defined workflow skills, bounded model/tool loops, explicit workflow state, and user approval gates for state-changing actions.

For the architecture overview, workflow diagram, quick start, and troubleshooting guide, begin with the [repository README](../README.md).

The project is under active development. It is suitable for local experimentation and supervised evaluation, but it is not yet a production purchasing system and does not place orders.

## Features

- Compact inventory discovery followed by explicit single-item detail retrieval
- Markdown-defined workflow skills with tool allowlists and step limits
- Isolated skill conversations that return compact summaries to the parent chat
- In-memory workflow and skill state
- Frontend approval prompts for permission-gated tools
- Context compaction and bounded tool-response sizes
- Wails desktop frontend

See [AGENTIC_WORKFLOW_ROADMAP.md](AGENTIC_WORKFLOW_ROADMAP.md) for current implementation status and planned work.

## Requirements

- Go 1.25 or later
- Node.js and npm
- Wails v2 CLI
- An Ollama-compatible chat endpoint
- A local inventory database using your own non-sensitive data

## Configuration

AIMS2 reads runtime configuration from environment variables. Copy `.env.example` as a reference and export the values through your shell, process manager, or secret manager. The application does not automatically load env files.

| Variable | Purpose | Default |
| --- | --- | --- |
| `AIMS2_MODEL_API_URL` | Ollama-compatible API base URL | `http://localhost:11434/api/` |
| `AIMS2_MODEL` | Chat model name | `gemma4:12b` |
| `AIMS2_KEEP_ALIVE` | Model keep-alive value | `5m` |
| `AIMS2_HTTP_TIMEOUT` | Outbound HTTP timeout | `2m` |
| `OLLAMA_API_KEY` | Optional Ollama web-search credential | unset |
| `AIMS2_WEB_SEARCH_API_URL` | Web-search endpoint | Ollama web search |
| `AIMS2_INVENTORY_DB_PATH` | Local SQLite inventory database | `./importer/inventory.db` |
| `AIMS2_INVENTORY_IMPORT_PATH` | Optional spreadsheet import path | unset |
| `AIMS2_EXCLUDED_VENDOR_CODES` | Comma-separated vendor codes omitted from priority ranking | unset |
| `AIMS2_CONVERSATION_LOG` | Opt-in NDJSON conversation log path | disabled |
| `AIMS2_DEBUG_MODEL_PAYLOADS` | Print full model payloads | `false` |

Never commit API keys, conversation logs, inventory databases, or spreadsheet exports. Conversation logs and model payloads can contain prompts, tool arguments, item notes, prices, costs, and other confidential business data.

## Development

Install frontend dependencies:

```sh
cd frontend
npm ci
cd ..
```

Run the desktop application with live reload:

```sh
wails dev
```

Run tests and build the frontend:

```sh
go test ./...
cd frontend && npm run build
```

Build a desktop binary:

```sh
wails build
```

## Inventory data

No real inventory database or business export is distributed with the public project. Tests create temporary SQLite databases populated with synthetic fixtures. Provide your own local data through the configured paths and review it for confidential or personal information before use.

## Security and privacy

State-changing tools are paused until the user explicitly approves or denies the pending call. This is a safety boundary for supervised workflows, not a replacement for authentication, authorization, database permissions, or operational review.

Conversation logging is opt-in. Full model payload debugging is also opt-in and should only be used with non-sensitive data. See [SECURITY.md](SECURITY.md) for vulnerability reporting and deployment guidance.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Licensed under the MIT License. See the repository-level `LICENSE` file.
