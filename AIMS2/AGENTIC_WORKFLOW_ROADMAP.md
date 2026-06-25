# AIMS2 Agentic Workflow Roadmap

## Objective

Build a reliable supervised agent that can complete deeper purchasing and inventory workflows without losing important context or overwhelming the model with large tool results.

## Current Status

- [x] Multi-round model and tool execution loop
- [x] Structured tool registry
- [x] Inventory search and ranking
- [x] Persistent inventory notes
- [x] Vendor-priority analysis
- [x] Conversation and tool logging
- [x] Desktop chat interface
- [ ] Reliable multi-step business workflows

## Milestone 1: Compact Tool Results

Reduce the amount of inventory data placed directly into model context.

- [ ] Return result counts and only the top relevant matches
- [ ] Limit initial inventory responses to fields needed for the question
- [ ] Add stable result or item IDs
- [ ] Add a tool for retrieving complete details for selected items
- [ ] Add pagination or continuation support
- [ ] Include concise warnings and confidence information
- [ ] Enforce maximum tool-response sizes

### Acceptance Criteria

- A broad inventory query does not return an unbounded JSON payload.
- The model can select an item and request its full details.
- Important warnings and notes remain visible in summarized results.

## Milestone 2: Workflow-Oriented Tools

Move filtering, calculations, sorting, and deduplication into deterministic Go code.

- [ ] Implement `analyze_vendor_order`
- [ ] Implement `build_reorder_candidates`
- [ ] Implement `review_critical_inventory`
- [ ] Implement `prepare_purchase_recommendation`
- [ ] Implement `compare_inventory_items`
- [ ] Return structured summaries instead of raw database rows
- [ ] Separate read-only tools from state-changing tools

### Acceptance Criteria

- The model does not calculate purchasing quantities from large raw datasets.
- Each workflow tool returns a small, documented response schema.
- The same input produces consistent calculations and ordering.

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

- [ ] Define the first high-value workflow skill
- [ ] Keep skills focused on procedure rather than inventory data
- [ ] Define tool-selection rules for each stage
- [ ] Define required clarification questions
- [ ] Define uncertainty and exception handling
- [ ] Define completion criteria
- [ ] Define approval boundaries for state-changing actions

### Initial Skill: Vendor Reorder

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
- [ ] Validate all tool arguments
- [ ] Return structured tool errors
- [ ] Stop execution after unrecoverable errors
- [ ] Execute dependent tool calls sequentially
- [ ] Add conversation trimming or summarization
- [ ] Correctly reset backend state when starting a new chat
- [ ] Require user confirmation before state-changing operations

### Acceptance Criteria

- Failed tools do not crash or corrupt the workflow.
- A new chat resets both frontend and backend conversation state.
- No inventory note or purchasing change occurs without the required approval.

## Milestone 6: Evaluation and Testing

- [ ] Replace print-only tests with assertions
- [ ] Use isolated temporary databases in tests
- [ ] Test compact result limits
- [ ] Test invalid and missing tool arguments
- [ ] Test workflow interruption and resumption
- [ ] Test tool failure recovery
- [ ] Test approval requirements
- [ ] Create scripted end-to-end workflow scenarios

### Initial Evaluation Scenarios

- [ ] Find an exact part and report availability
- [ ] Resolve an ambiguous part-number search
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
- [ ] Define access controls for state-changing tools

## Recommended Implementation Order

1. Compact `stock_check` results.
2. Add item IDs and a detail-retrieval tool.
3. Implement the first workflow-oriented vendor reorder tool.
4. Introduce persistent workflow state.
5. Add the vendor reorder skill.
6. Add confirmations and failure recovery.
7. Build end-to-end evaluations.

## Definition of a Solid Supervised Workflow

- [ ] Handles a multi-step purchasing task without losing its goal
- [ ] Keeps large datasets outside model context
- [ ] Uses deterministic code for business calculations
- [ ] Preserves decisions and progress in explicit workflow state
- [ ] Explains uncertainty and requests clarification when necessary
- [ ] Requires approval before making changes
- [ ] Recovers safely from tool and model failures
- [ ] Passes repeatable end-to-end workflow evaluations
