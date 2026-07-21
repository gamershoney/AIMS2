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
- [x] Model request size diagnostics
- [x] Test coverage for compact `stock_check` output
- [x] Assertion-based validation in the compact-result test
- [x] In-memory workflow state slot for recent tool context
- [x] Context compaction for old raw tool messages
- [x] Desktop chat interface
- [x] Skill registry and `SKILL.md` parser
- [x] Isolated skill chat sessions with per-skill tool allowlists
- [x] Bounded skill runner with completion summaries returned to the parent chat
- [x] In-memory skill/job state with status, summary, warnings, and data
- [x] Code-enforced permission gate for state-changing tools
- [x] Frontend permission popup with explicit Allow and Deny actions
- [x] Conversation log excluded from Git
- [x] Production config and secrets moved out of code
- [ ] Reliable multi-step business workflows

### Current Agentic Workflow Assessment

The project now has a usable tool foundation for supervised inventory assistance. The biggest improvement already made is separating broad item discovery from detailed item inspection:

1. `stock_check` now returns compact candidates only.
2. `item_details` retrieves full inventory data only after a specific `stock_number` is known.
3. Tool descriptions now tell the model when to use each tool and when not to.

The project now has the foundation of a supervised workflow runner. The parent chat can select a parsed skill, create an isolated child conversation, expose only that skill's allowed tools, enforce a maximum number of steps, update skill state, and return a compact completion summary to the parent.

State-changing tools are now paused in code until the user approves or denies the exact pending call in the frontend. The remaining gap is production-grade business execution: deterministic purchasing/reorder tools, durable cross-session state and resume behavior, stronger failure recovery, and repeatable end-to-end evaluations.

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
- Important warnings and notes remain visible in summarized results. ✅
- All high-volume tools have explicit size limits. ✅

### Completed Details

`stock_check` currently returns:

```json
{
  "count": 1,
  "results": [
    {
      "stock_number": "TEST-16114",
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
  "stock_number": "TEST-16114",
  "store_id": 1,
  "description": "Example inventory item",
  "on_hand": 4,
  "confidence_score": 100,
  "match_type": "Exact Stock Number",
  "has_note": true,
  "note": "Synthetic replenishment example",
  "is_critical": true
}
```

Full commercial and transaction fields such as price, cost, vendor, usage, and transaction dates are intentionally deferred to `item_details`. `stock_check` only includes compact note signals and note text when present.

`stock_number` is still the primary model-facing item reference. Because the business tracks the same `stock_number` across multiple locations, `store_id` should travel with item references when workflow state is introduced. No composite `item_key` is planned right now.

## Milestone 2: Workflow-Oriented Tools

Move filtering, calculations, sorting, and deduplication into deterministic Go code.

- [x] Separate general item search from exact item detail retrieval
- [x] Improve tool descriptions so the model can choose tools more consistently
- [x] Clarify that price and cost lookup is read-only and does not require confirmation
- [x] Mark `make_note` as state-changing in its tool description
- [ ] Implement `analyze_vendor_order`
- [ ] Implement `build_reorder_candidates`
- [ ] Implement `review_critical_inventory`
- [ ] Implement `prepare_purchase_recommendation`
- [ ] Implement `compare_inventory_items`
- [ ] Return structured summaries instead of raw database rows
- [x] Enforce read-only vs state-changing behavior in code, not just descriptions
- [x] Add confirmation gating before `make_note`

### Acceptance Criteria

- The model does not calculate purchasing quantities from large raw datasets.
- Each workflow tool returns a small, documented response schema.
- The same input produces consistent calculations and ordering.
- State-changing tools are blocked unless the user clearly approved the change.

## Milestone 3: Explicit Workflow State

Store workflow progress outside the conversation history.

- [x] Define a minimal `WorkflowState` structure
- [x] Add a reserved system-message slot for current workflow state
- [x] Update workflow state from tool results
- [x] Apply workflow state during tool execution
- [x] Assign each active skill session an in-memory job ID
- [x] Track the workflow reason and lifecycle state
- [ ] Track selected inventory item IDs
- [x] Track compact summaries, warnings, and extensible skill data
- [ ] Track decisions and assumptions as explicit typed records
- [x] Persist workflow state between model turns in memory
- [x] Send current relevant state to the model
- [ ] Support resuming and cancelling workflows

Current minimal structure:

```go
type WorkflowState struct {
    LastToolUsed   string `json:"last_tool_used,omitempty"`
    LastItemSearch string `json:"last_item_searched,omitempty"`
    LastItemFound  string `json:"last_item_found,omitempty"`
}
```

Future fuller structure:

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

Current skill-session state:

```go
type SkillState struct {
    JobID         int
    SkillName     string
    StateName     StateName
    EnteredReason string
    Status        string
    Summary       string
    Data          map[string]any
    Warnings      []string
    PendingTool   string
}
```

### Acceptance Criteria

- A workflow can continue without replaying every previous tool result. ✅
- The most recent item/tool context is available without replaying raw tool payloads. ✅
- The current stage and next valid actions are always identifiable.
- A user can resume an interrupted workflow.

### Completed Details

Tool results can now return workflow state, and `RunToolCalls` applies that state into a reserved system message. This gives the model a compact working memory such as the last tool used, the last item searched, and the last confirmed item found.

The skill runner now adds an in-memory job ID and lifecycle state (`entered`, `working`, or `completed`), along with summary, warning, pending-permission, and extensible data fields. The state is injected into the isolated skill chat and synchronized back to the parent workflow.

This is not yet a durable workflow engine. Job IDs are process-local, stages are broad lifecycle states rather than business-specific stages, and there is no selected-item list, typed decision history, resume/cancel behavior, or cross-session persistence.

## Milestone 4: Skills and Procedures

Create compact procedural instructions after the supporting tools and workflow state exist.

- [x] Identify the first high-value workflow skill candidate
- [x] Parse skill metadata and full Markdown bodies from `SKILL.md`
- [x] Keep skills focused on procedure rather than inventory data
- [x] Define tool-selection rules for each stage
- [x] Define required clarification questions
- [x] Define uncertainty and exception handling
- [x] Define completion criteria and final summary schemas
- [x] Define approval boundaries for state-changing actions
- [x] Create isolated child chats with only allowed skill tools
- [x] Add internal `update_skill_state` and `complete_skill` tools
- [x] Stop skill execution at the configured `max_steps`
- [x] Return completed skill summaries to the parent chat

### Initial Skill Candidate: Supervised Inventory Investigation

This is the best first skill with the tools currently available because it can use `stock_check`, `item_details`, `priority_vendors`, `search_Online`, and `make_note` without requiring new reorder-calculation tools first.

- [x] Clarify whether the user is identifying an item, checking availability, researching public specs, or saving a note
- [x] Use `stock_check` for candidate discovery
- [x] Use `item_details` only after an exact `stock_number` is known
- [x] Use `search_Online` only for public manufacturer/spec/market information
- [x] Require explicit confirmation before calling `make_note`
- [x] Summarize uncertainty and ask for clarification when multiple candidates remain

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
- [x] Enforce per-skill tool allowlists
- [x] Enforce per-skill maximum step counts
- [x] Validate required `stock_check.query`
- [x] Validate `stock_check.in_stock` type
- [ ] Validate all remaining tool arguments
- [ ] Return structured tool errors
- [ ] Stop execution immediately after all unrecoverable tool errors
- [ ] Execute dependent tool calls sequentially
- [x] Trim old raw tool messages from model context
- [x] Compact context before and after model responses once the turn threshold is reached
- [x] Log model request byte size and message count
- [ ] Add conversation memory summarization
- [x] Correctly reset backend state when starting a new chat
- [x] Require user confirmation before state-changing operations
- [x] Present pending permission details in the frontend
- [x] Resume the paused skill after approval or denial
- [ ] Fix `make_note` numeric argument parsing from JSON `float64`
- [ ] Return immediately after validation errors in `make_note` and `item_details`
- [ ] Add deterministic tool ordering

### Acceptance Criteria

- Failed tools do not crash or corrupt the workflow.
- A new chat resets both frontend and backend conversation state.
- Old tool payloads are removed from model context after they are no longer part of the active turn. ✅
- No inventory note or purchasing change occurs without the required approval.

## Milestone 6: Evaluation and Testing

- [x] Add assertions for compact `stock_check` response shape
- [x] Replace compact-result debug output with assertions
- [x] Test compact note warning fields in `stock_check`
- [x] Test `stock_check` workflow-state output
- [x] Test tool execution applies workflow state
- [x] Test workflow-state system slot replacement
- [x] Test missing workflow-state slot does not overwrite user messages
- [x] Test compaction drops old tool messages and old assistant tool calls
- [x] Test skill metadata parsing and full skill-body loading
- [x] Test skill selection creates isolated state and allowed tools
- [x] Test disallowed tools are rejected inside an active skill
- [x] Test skill-state updates and completion
- [x] Test completed skill summaries return to the parent chat
- [x] Test the parent loop stops after skill selection
- [x] Test permission-required tools do not execute before approval
- [x] Test frontend permission requests expose pending tool details
- [x] Test frontend approval resumes and executes the pending tool
- [x] Test new chat preserves the skill registry and updates the selector pointer
- [x] Replace remaining print-only tests with assertions
- [x] Use isolated temporary databases in tests
- [ ] Test compact result limits
- [ ] Test invalid and missing tool arguments
- [ ] Test workflow interruption and resumption
- [ ] Test tool failure recovery
- [x] Test approval requirements
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

- [x] Move API keys to environment variables or secret management
- [x] Move model and service endpoints to configuration
- [ ] Rotate any credentials previously committed to Git
- [x] Remove ad hoc debug printing and keep structured logging opt-in
- [ ] Add field-level redaction when logging is enabled
- [x] Add `conversation_log.jsonl` to `.gitignore`
- [x] Keep conversation logging disabled unless an explicit external path is configured
- [ ] Define access controls for state-changing tools

## Recommended Implementation Order

1. Fix `make_note` and `item_details` validation/error-return behavior.
2. Add tests for compact result limits and remaining invalid tool arguments.
3. Implement deterministic reorder-candidate and quantity-calculation tools.
4. Add deterministic tool ordering for dependent calls.
5. Add HTTP timeouts, cancellation, and bounded retries.
6. Add typed order-draft state, selected item references, and decision history.
7. Add workflow cancellation and durable resume support.
8. Build scripted end-to-end evaluations for inventory investigation and order drafting.

## Definition of a Solid Supervised Workflow

- [ ] Handles a multi-step purchasing task without losing its goal
- [x] Keeps initial inventory search datasets outside model context
- [ ] Keeps all large tool outputs outside model context
- [ ] Uses deterministic code for business calculations
- [x] Preserves current progress in explicit in-memory workflow state
- [ ] Preserves typed decisions and progress across application sessions
- [ ] Explains uncertainty and requests clarification when necessary
- [x] Requires approval before permission-gated changes
- [ ] Recovers safely from tool and model failures
- [ ] Passes repeatable end-to-end workflow evaluations
