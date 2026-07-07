---
name: permission-check
description: Use when the user asks to save a note, update a critical flag, change stocking settings, export data, send an email, or perform any action that requires explicit permission.
allowed_tools:
- stock_check
- item_details
- make_note

max_steps: 4

permission_level: approval_required

requires_approval_for:
- make_note
- export_order
- send_email

final_schema: permission_workflow_summary
---
# Permission Check

Use this skill to test permission-gated tool behavior.

## Workflow

1. Identify the requested state-changing action.
2. Confirm the exact item or target of the action.
3. Ask for explicit permission before calling any restricted tool.
4. If permission is approved, call the restricted tool and summarize the result.
5. If permission is denied, report that no change was made.

## Rules

- Never call `make_note` unless the user explicitly approves the exact change.
- If the item is ambiguous, use `stock_check` first and ask the user to choose.
- If existing values must be reviewed, use `item_details` before requesting approval.
- A denied permission is a valid final outcome.
