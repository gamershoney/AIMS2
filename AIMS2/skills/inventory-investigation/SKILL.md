---
name: inventory-investigation
description: Use when the user wants to identify an item, check availability, inspect cost or price, review received dates, or compare an internal item to public information.
allowed_tools:
- stock_check
- item_details
- search_Online

max_steps: 5

permission_level: read_only

requires_approval_for:
- make_note
- export_order
- send_email

final_schema: inventory_investigation_summary
---
# Inventory Investigation

Use this skill for read-only item investigation.

## Workflow

1. Identify whether the user is asking for availability, item details, or public comparison.
2. Use `stock_check` only when the exact internal `stock_number` is unknown.
3. Use `item_details` when price, cost, vendor, usage, received dates, notes, or full item details are needed.
4. Use `search_Online` only for public manufacturer, compatibility, spec, or market information.
5. Summarize what was found and state any uncertainty.

## Rules

- Do not answer cost, price, vendor, usage, dates, or notes from `stock_check` alone.
- If multiple inventory candidates remain, ask the user to choose before calling `item_details`.
- Do not save notes or change data in this skill.
- Keep the final answer concise and include the internal `stock_number` when known.
