---
name: build-order
description: Use when the user wants to build, review, prepare, or adjust a purchasing order, reorder draft, vendor order, or list of inventory items to consider buying.
allowed_tools:
- stock_check
- item_details
- priority_vendors

max_steps: 6

permission_level: draft

requires_approval_for:
- send_email
- export_order
- make_note

final_schema: order_draft_summary
---
# Build Order Draft

Use this skill when the user wants to build, review, prepare, or adjust a purchasing order.

## Workflow

1. Identify the vendor, item family, or inventory condition the user cares about.
2. Use inventory tools to find relevant candidates.
3. Build or update an order draft in workflow state.
4. Summarize items, quantities, reasons, and warnings.
5. Ask for review or approval before exporting, emailing, or recording changes.

## Rules

- Do not place an order.
- Do not send an email.
- Do not save notes unless the user explicitly approves.
- If no deterministic quantity tool is available, do not invent quantities. Ask the user for quantities or present items as candidates without order quantities.
- If multiple vendors are involved, group the draft by vendor.
