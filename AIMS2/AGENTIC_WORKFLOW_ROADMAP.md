# AIMS2 Agentic Workflow Roadmap

## Objective

Build a reliable supervised agent that can complete deeper purchasing and inventory workflows without losing important context or overwhelming the model with large tool results.

## Current Status

- [x] Multi-round model and tool execution loop
- [x] Structured tool registry
- [x] Inventory search and ranking
- [x] Compact `stock_check` response schema
- [x] `item_details` tool for full single-item lookup
- [x] Tool descriptions updated for better model tool selection
- [x] Persistent inventory notes
- [x] Vendor-priority analysis
- [x] Conversation and tool logging
- [x] Test coverage for compact `stock_check` output
- [x] More diagnostic logging in the compact-result test
- [x] Desktop chat interface
- [x] Conversation log excluded from Git
- [ ] Production config and secrets moved out of code
- [ ] Reliable multi-step business workflows

### Current Agentic Workflow Assessment

The project now has a usable tool foundation for supervised inventory assistance. The biggest improvement already made is separating broad item discovery from detailed item inspection:

1. `stock_check` now returns compact candidates only.
2. `item_details` retrieves full inventory data only after a specific `stock_number` is known.
3. Tool descriptions now tell the model when to use each tool and when not to.

The remaining gap is not basic tool calling. The remaining gap is workflow control: durable state, deterministic purchasing/reorder tools, bounded context, confirmation rules, and end-to-end evaluations.

## Milestone 1: Compact Tool Results

Reduce the amount of inventory data placed directly into model context.

- [x] Return result counts and only the top relevant matches
- [x] Limit initial inventory responses to fields needed for the question
- [x] Include `store_id` with inventory item results for model clarity
- [ ] Preserve `store_id` through workflow state wherever `stock_number` is stored
- [x] Add a tool for retrieving complete details for selected items
- [ ] Add pagination or continuation support
- [x] Include concise warnings and confidence information
- [x] Enforce maximum tool-response sizes
- [x] Compact or cap `priority_vendors` output
- [x] Compact or cap `search_Online` output

### Acceptance Criteria

- A broad inventory query does not return an unbounded JSON payload. ✅
- The model can select an item and request its full details. ✅
- Important warnings and notes remain visible in summarized results.
- All high-volume tools have explicit size limits.

### Completed Details

`stock_check` currently returns:

```json
{
  "count": 1,
  "results": [
    {
      "stock_number": "3M-16114",
      "store_id": 1,
      "description": "Example inventory item",
      "on_hand": 4,
      "confidence_score": 100,
      "match_type": "Exact Stock Number",
      "has_note": false
    }
  ]
}
```

When a note exists, `stock_check` includes compact note signals:

```json
{
  "stock_number": "3M-16114",
  "store_id": 1,
  "description": "Example inventory item",
  "on_hand": 4,
  "confidence_score": 100,
  "match_type": "Exact Stock Number",
  "has_note": true,
  "note": "Keep extra on hand for contract customer",
  "is_critical": true
}
```

Full commercial and transaction fields such as price, cost, vendor, usage, and transaction dates are intentionally deferred to `item_details`. `stock_check` only includes compact note signals and note text when present.

`stock_number` is still the primary model-facing item reference. Because the business tracks the same `stock_number` across multiple locations, `store_id` should travel with item references when workflow state is introduced. No composite `item_key` is planned right now.

## Milestone 2: Workflow-Oriented Tools

Move filtering, calculations, sorting, and deduplication into deterministic Go code.

- [x] Separate general item search from exact item detail retrieval
- [x] Improve tool descriptions so the model can choose tools more consistently
- [x] Mark `make_note` as state-changing in its tool description
- [ ] Implement `analyze_vendor_order`
- [ ] Implement `build_reorder_candidates`
- [ ] Implement `review_critical_inventory`
- [ ] Implement `prepare_purchase_recommendation`
- [ ] Implement `compare_inventory_items`
- [ ] Return structured summaries instead of raw database rows
- [ ] Enforce read-only vs state-changing behavior in code, not just descriptions
- [ ] Add confirmation gating before `make_note`

### Acceptance Criteria

- The model does not calculate purchasing quantities from large raw datasets.
- Each workflow tool returns a small, documented response schema.
- The same input produces consistent calculations and ordering.
- State-changing tools are blocked unless the user clearly approved the change.

## Milestone 3: Explicit Workflow State

Store workflow progress outside the conversation history.

- [ ] Define a `WorkflowState` structure
- [ ] Assign each workflow a stable ID
- [ ] Track the workflow goal and current stage
- [ ] Track selected inventory item IDs
- [ ] Track decisions, assumptions, and warnings
- [ ] Persist workflow state between model turns
- [ ] Send only the current stage and relevant state to the model
- [ ] Support resuming and cancelling workflows

Suggested structure:

```go
type WorkflowState struct {
    WorkflowID string
    Goal       string
    Stage      string
    ItemIDs    []string
    Decisions  []Decision
    Warnings   []string
}
```

### Acceptance Criteria

- A workflow can continue without replaying every previous tool result.
- The current stage and next valid actions are always identifiable.
- A user can resume an interrupted workflow.

## Milestone 4: Skills and Procedures

Create compact procedural instructions after the supporting tools and workflow state exist.

- [x] Identify the first high-value workflow skill candidate
- [ ] Keep skills focused on procedure rather than inventory data
- [ ] Define tool-selection rules for each stage
- [ ] Define required clarification questions
- [ ] Define uncertainty and exception handling
- [ ] Define completion criteria
- [ ] Define approval boundaries for state-changing actions

### Initial Skill Candidate: Supervised Inventory Investigation

This is the best first skill with the tools currently available because it can use `stock_check`, `item_details`, `priority_vendors`, `search_Online`, and `make_note` without requiring new reorder-calculation tools first.

- [ ] Clarify whether the user is identifying an item, checking availability, researching public specs, or saving a note
- [ ] Use `stock_check` for candidate discovery
- [ ] Use `item_details` only after an exact `stock_number` is known
- [ ] Use `search_Online` only for public manufacturer/spec/market information
- [ ] Require explicit confirmation before calling `make_note`
- [ ] Summarize uncertainty and ask for clarification when multiple candidates remain

### Later Skill: Vendor Reorder

- [ ] Identify or clarify the vendor
- [ ] Retrieve summarized reorder candidates
- [ ] Investigate critical notes and exceptions
- [ ] Generate deterministic quantity recommendations
- [ ] Present a reviewable proposal
- [ ] Require confirmation before recording or exporting changes

### Acceptance Criteria

- The model follows the same major stages for repeated workflow runs.
- The skill remains short enough to avoid competing with business data for context.
- Tool failures and ambiguous requests have defined recovery behavior.

## Milestone 5: Reliability and Control

- [ ] Add HTTP request timeouts
- [ ] Add request cancellation
- [ ] Add bounded retries for transient failures
- [x] Validate required `stock_check.query`
- [x] Validate `stock_check.in_stock` type
- [ ] Validate all remaining tool arguments
- [ ] Return structured tool errors
- [ ] Stop execution immediately after all unrecoverable tool errors
- [ ] Execute dependent tool calls sequentially
- [ ] Add conversation memory summarization and trim old raw tool messages from model context
- [ ] Correctly reset backend state when starting a new chat
- [ ] Require user confirmation before state-changing operations
- [ ] Fix `make_note` numeric argument parsing from JSON `float64`
- [ ] Return immediately after validation errors in `make_note` and `item_details`
- [ ] Add deterministic tool ordering

### Acceptance Criteria

- Failed tools do not crash or corrupt the workflow.
- A new chat resets both frontend and backend conversation state.
- No inventory note or purchasing change occurs without the required approval.

## Milestone 6: Evaluation and Testing

- [x] Add assertions for compact `stock_check` response shape
- [x] Add diagnostic test logging for compact `stock_check` output
- [x] Test compact note warning fields in `stock_check`
- [ ] Replace remaining print-only tests with assertions
- [ ] Use isolated temporary databases in tests
- [ ] Test compact result limits
- [ ] Test invalid and missing tool arguments
- [ ] Test workflow interruption and resumption
- [ ] Test tool failure recovery
- [ ] Test approval requirements
- [ ] Create scripted end-to-end workflow scenarios
- [ ] Fix `go test ./...` working-directory dependency in `importer` tests

### Initial Evaluation Scenarios

- [x] Unit-test compact serialization for an exact stock item
- [ ] Find an exact part and report availability through the full chat loop
- [ ] Resolve an ambiguous part-number search through the full chat loop
- [ ] Find an item, add a note, and verify the note
- [ ] Review critical inventory for a vendor
- [ ] Build a reorder proposal containing multiple items
- [ ] Resume a partially completed reorder workflow
- [ ] Recover from an unavailable model or tool service

### Acceptance Criteria

- Core tests run reliably without depending on the developer's working directory.
- End-to-end scenarios produce consistent, reviewable outcomes.
- Regressions in tool schemas and workflow stages are detected automatically.

## Production Configuration and Security

These items are planned for production configuration rather than the workflow implementation itself.

- [ ] Move API keys to environment variables or secret management
- [ ] Move model and service endpoints to configuration
- [ ] Rotate any credentials previously committed to Git
- [ ] Configure safe logging and redact sensitive values
- [x] Add `conversation_log.jsonl` to `.gitignore`
- [ ] Decide whether conversation logs should be disabled, redacted, or stored outside the repo by default
- [ ] Define access controls for state-changing tools

## Recommended Implementation Order

1. Fix `make_note` and `item_details` validation/error-return behavior.
2. Add confirmation gating before state-changing tool calls.
3. Add tests for compact result limits.
4. Add deterministic tool ordering.
5. Add HTTP timeouts and cancellation.
6. Create the first skill: supervised inventory investigation.
7. Implement the first workflow-oriented vendor reorder tool.
8. Introduce persistent workflow state.
9. Build end-to-end evaluations.

## Definition of a Solid Supervised Workflow

- [ ] Handles a multi-step purchasing task without losing its goal
- [x] Keeps initial inventory search datasets outside model context
- [ ] Keeps all large tool outputs outside model context
- [ ] Uses deterministic code for business calculations
- [ ] Preserves decisions and progress in explicit workflow state
- [ ] Explains uncertainty and requests clarification when necessary
- [ ] Requires approval before making changes
- [ ] Recovers safely from tool and model failures
- [ ] Passes repeatable end-to-end workflow evaluations
