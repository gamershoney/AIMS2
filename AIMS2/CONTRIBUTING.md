# Contributing

Thank you for helping improve AIMS2.

## Before submitting a change

1. Create a focused branch and keep unrelated changes separate.
2. Do not commit credentials, private endpoints, real inventory records, conversation logs, or customer/vendor data.
3. Add or update tests for behavior changes.
4. Run `go test ./...` from the `AIMS2` application directory.
5. Run `npm ci` and `npm run build` in `AIMS2/frontend`.
6. Explain workflow-state, tool-schema, and permission-boundary changes in the pull request.

## Tool and skill changes

- Keep tool responses compact and structured.
- Treat state-changing tools as permission-required in code.
- Keep skills procedural; do not embed private business data in `SKILL.md` files.
- Add deterministic calculations to Go tools instead of asking the model to infer them from large datasets.
- Preserve `stock_number` and `store_id` together when a workflow stores item references.

## Security issues

Follow [SECURITY.md](SECURITY.md) instead of opening a public issue for security-sensitive reports.
