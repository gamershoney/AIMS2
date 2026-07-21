package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gamershoney/AIMS2/AIMS2/importer"
)

func TestRunToolCalls(t *testing.T) {
	tc := ToolCall{
		ID: "testID", Function: struct {
			Index     int64          "json:\"index\""
			Name      string         "json:\"name\""
			Arguments map[string]any "json:\"arguments\""
		}{
			Index:     1,
			Name:      "stock_check",
			Arguments: map[string]any{"query": "TEST-16114"},
		},
	}

	tool := &StockCheck{}

	testMsg := tool.Execute(context.TODO(), tc)
	if testMsg.Error != nil {
		t.Fatalf("stock check failed: %v", testMsg.Error)
	}
}

func TestAppSendMsgReturnsModelError(t *testing.T) {
	modelErr := errors.New("model unavailable")
	app := &App{Thread: &Chat{
		sendOnceFunc: func(*Chat) (*Answer, error) {
			return nil, modelErr
		},
	}}

	response, err := app.SendMsg("test request")
	if !errors.Is(err, modelErr) {
		t.Fatalf("expected model error to propagate, got %v", err)
	}
	if response != "" {
		t.Fatalf("expected empty response on model failure, got %q", response)
	}
}

func TestStockCheckExecuteReturnsWorkflowState(t *testing.T) {
	tc := ToolCall{
		ID: "testID", Function: struct {
			Index     int64          "json:\"index\""
			Name      string         "json:\"name\""
			Arguments map[string]any "json:\"arguments\""
		}{
			Index:     1,
			Name:      "stock_check",
			Arguments: map[string]any{"query": "TEST-16114"},
		},
	}

	tool := &StockCheck{}
	result := tool.Execute(context.TODO(), tc)
	if result.Error != nil {
		t.Fatalf("stock check failed: %v", result.Error)
	}
	if result.State == nil {
		t.Fatalf("expected stock_check to return workflow state")
	}
	if result.State.LastToolUsed != "stock_check" {
		t.Fatalf("expected last tool used to be stock_check, got %q", result.State.LastToolUsed)
	}
	if result.State.LastItemSearch != "TEST-16114" {
		t.Fatalf("expected last item search to be recorded, got %q", result.State.LastItemSearch)
	}
	if result.State.LastItemFound != "" {
		t.Fatalf("stock_check should not set a confirmed last item found, got %q", result.State.LastItemFound)
	}
}

func TestRunToolCallsAppliesWorkflowState(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(&StockCheck{})

	chat := &Chat{
		registry: registry,
		Messages: []Messages{
			{Role: "system", Content: "base system prompt"},
			{Role: "system", Content: ""},
		},
	}

	ai := AIMessage{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				ID: "testID", Function: struct {
					Index     int64          "json:\"index\""
					Name      string         "json:\"name\""
					Arguments map[string]any "json:\"arguments\""
				}{
					Index:     1,
					Name:      "stock_check",
					Arguments: map[string]any{"query": "TEST-16114"},
				},
			},
		},
	}

	results := chat.RunToolCalls(ai)
	if len(results) != 1 {
		t.Fatalf("expected one tool result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Fatalf("stock check failed: %v", results[0].Error)
	}
	if chat.workflowState == nil {
		t.Fatalf("expected RunToolCalls to apply workflow state to chat")
	}
	if chat.workflowState.LastToolUsed != "stock_check" {
		t.Fatalf("expected last tool used to be stock_check, got %q", chat.workflowState.LastToolUsed)
	}
	if chat.workflowState.LastItemSearch != "TEST-16114" {
		t.Fatalf("expected last item search to be recorded, got %q", chat.workflowState.LastItemSearch)
	}
	if len(chat.Messages) < 2 {
		t.Fatalf("expected workflow state message to be applied, got messages=%+v", chat.Messages)
	}
	stateMessage := chat.Messages[1]
	if stateMessage.Role != "system" {
		t.Fatalf("expected workflow state message to be system role, got %+v", stateMessage)
	}
	for _, want := range []string{
		"Current Working State:",
		`"last_tool_used": "stock_check"`,
		`"last_item_searched": "TEST-16114"`,
	} {
		if !strings.Contains(stateMessage.Content, want) {
			t.Fatalf("workflow state message missing %q: %s", want, stateMessage.Content)
		}
	}
}

func TestStockCheckParseInventoryDataReturnsCompactResults(t *testing.T) {
	tool := &StockCheck{}
	results := []*importer.ReturnResult{
		{
			Item: importer.InventoryItem{
				StockNumber:      "TEST-16114",
				Location:         1,
				Description:      "Example inventory item",
				VendorNumber:     "TEST",
				VendorProdNumber: "SYNTH-16114",
				Price:            99.95,
				Cost:             50.25,
				OnHand:           4,
				Usage:            2,
				LastSale:         "2026-06-01",
			},
			ConfidenceScore: 100,
			MatchType:       "Exact Stock Number",
		},
	}

	raw, err := tool.ParseInventoryData(results)
	if err != nil {
		t.Fatalf("parse inventory data: %v", err)
	}
	var response StockCheckResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("unmarshal compact response: %v", err)
	}
	if response.Count != 1 || len(response.Results) != 1 {
		t.Fatalf("expected one result, got count=%d results=%d", response.Count, len(response.Results))
	}

	match := response.Results[0]
	if match.StockNumber != "TEST-16114" || match.StoreID != 1 || match.OnHand != 4 || match.ConfidenceScore != 100 {
		t.Fatalf("unexpected compact match: %+v", match)
	}
	if match.HasNote {
		t.Fatalf("expected has_note to be false for result without a note")
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("unmarshal response fields: %v", err)
	}
	encodedResults, ok := fields["results"].([]any)
	if !ok || len(encodedResults) != 1 {
		t.Fatalf("missing compact results: %s", raw)
	}
	encodedMatch, ok := encodedResults[0].(map[string]any)
	if !ok {
		t.Fatalf("invalid compact result: %s", raw)
	}
	for _, omitted := range []string{"price", "cost", "avg_usage", "vendor_number", "vendor_product_number", "last_sale", "note"} {
		if _, exists := encodedMatch[omitted]; exists {
			t.Errorf("compact result unexpectedly includes %q", omitted)
		}
	}
}

func TestStockCheckParseInventoryDataIncludesNoteWhenPresent(t *testing.T) {
	tool := &StockCheck{}
	results := []*importer.ReturnResult{
		{
			Item: importer.InventoryItem{
				Location:    1,
				StockNumber: "TEST-16114",
				Description: "Example inventory item",
				OnHand:      4,
			},
			Note: &importer.Note{
				StockNumber:   "TEST-16114",
				Location:      1,
				Note:          "Keep extra on hand for contract customer",
				IsCritical:    true,
				MinimumOnHand: 6,
				TargetOnHand:  12,
			},
			HasNote:         true,
			ConfidenceScore: 100,
			MatchType:       "Exact Stock Number",
		},
	}

	raw, err := tool.ParseInventoryData(results)
	if err != nil {
		t.Fatalf("parse inventory data with note: %v", err)
	}
	var response StockCheckResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("unmarshal compact response with note: %v", err)
	}
	if response.Count != 1 || len(response.Results) != 1 {
		t.Fatalf("expected one result, got count=%d results=%d", response.Count, len(response.Results))
	}

	match := response.Results[0]
	if !match.HasNote {
		t.Fatalf("expected has_note to be true")
	}
	if match.Note == "" {
		t.Fatalf("expected note to be included when present")
	}
	if !match.IsCritical {
		t.Fatalf("expected critical note flag")
	}
	if match.Note != "Keep extra on hand for contract customer" {
		t.Fatalf("unexpected note text: %q", match.Note)
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("unmarshal response fields: %v", err)
	}
	encodedResults, ok := fields["results"].([]any)
	if !ok || len(encodedResults) != 1 {
		t.Fatalf("missing compact results: %s", raw)
	}
	encodedMatch, ok := encodedResults[0].(map[string]any)
	if !ok {
		t.Fatalf("invalid compact result: %s", raw)
	}
	if _, exists := encodedMatch["note"]; !exists {
		t.Fatalf("expected serialized note field: %s", raw)
	}
	for _, omitted := range []string{"price", "cost", "avg_usage", "vendor_number", "vendor_product_number", "last_sale"} {
		if _, exists := encodedMatch[omitted]; exists {
			t.Errorf("compact result unexpectedly includes %q", omitted)
		}
	}
}

func TestCompactDropsOldToolMessagesAndClearsOldAssistantToolCalls(t *testing.T) {
	chat := &Chat{
		currentTurn: 3,
		Messages: []Messages{
			{Role: "system", Content: "system prompt", turn: 0},
			{Role: "user", Content: "old user", turn: 1},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "old-tool-call"},
				},
				turn: 1,
			},
			{Role: "tool", ToolName: "stock_check", Content: `{"old":true}`, turn: 1},
			{Role: "assistant", Content: "old final answer", turn: 1},
			{Role: "user", Content: "current user", turn: 3},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "current-tool-call"},
				},
				turn: 3,
			},
			{Role: "tool", ToolName: "stock_check", Content: `{"current":true}`, turn: 3},
			{Role: "assistant", Content: "current final answer", turn: 3},
			{Role: "user", Content: "filler one", turn: 2},
			{Role: "assistant", Content: "filler two", turn: 2},
		},
	}

	chat.Compact()

	if len(chat.Messages) == 0 {
		t.Fatalf("compact removed all messages")
	}
	if chat.Messages[0].Role != "system" || chat.Messages[0].Content != "system prompt" {
		t.Fatalf("system prompt not preserved as first message: %+v", chat.Messages)
	}

	oldToolCount := 0
	currentToolCount := 0
	oldAssistantToolCalls := 0
	currentAssistantToolCalls := 0
	systemCount := 0

	for _, msg := range chat.Messages {
		if msg.Role == "system" {
			systemCount++
		}
		if msg.Role == "tool" && msg.turn == 1 {
			oldToolCount++
		}
		if msg.Role == "tool" && msg.turn == 3 {
			currentToolCount++
		}
		if msg.Role == "assistant" && msg.turn == 1 && len(msg.ToolCalls) > 0 {
			oldAssistantToolCalls++
		}
		if msg.Role == "assistant" && msg.turn == 3 && len(msg.ToolCalls) > 0 {
			currentAssistantToolCalls++
		}
	}

	if systemCount != 1 {
		t.Fatalf("expected exactly one system prompt, got %d messages=%+v", systemCount, chat.Messages)
	}
	if oldToolCount != 0 {
		t.Fatalf("expected old tool messages to be dropped, got %d messages=%+v", oldToolCount, chat.Messages)
	}
	if currentToolCount != 1 {
		t.Fatalf("expected current tool message to be preserved, got %d messages=%+v", currentToolCount, chat.Messages)
	}
	if oldAssistantToolCalls != 0 {
		t.Fatalf("expected old assistant tool calls to be cleared, got %d messages=%+v", oldAssistantToolCalls, chat.Messages)
	}
	if currentAssistantToolCalls != 1 {
		t.Fatalf("expected current assistant tool call to be preserved, got %d messages=%+v", currentAssistantToolCalls, chat.Messages)
	}
}

func TestApplyStateReplacesReservedSystemMessage(t *testing.T) {
	state := &WorkflowState{
		LastToolUsed:   "item_details",
		LastItemSearch: "000068",
		LastItemFound:  "TEST-000068",
	}
	chat := &Chat{
		Messages: []Messages{
			{Role: "system", Content: "base system prompt"},
			{Role: "system", Content: ""},
			{Role: "user", Content: "How many do we have and what is our price?", turn: 1},
		},
	}

	if err := chat.ApplyState(state); err != nil {
		t.Fatalf("apply state: %v", err)
	}

	if len(chat.Messages) != 3 {
		t.Fatalf("expected ApplyState to replace reserved state slot without changing message count, got %d messages=%+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[0].Content != "base system prompt" {
		t.Fatalf("base system prompt was not preserved: %+v", chat.Messages)
	}

	stateMsg := chat.Messages[1]
	if stateMsg.Role != "system" {
		t.Fatalf("expected workflow state message to remain a system message, got %+v", stateMsg)
	}
	for _, want := range []string{
		"Current Working State:",
		`"last_tool_used": "item_details"`,
		`"last_item_searched": "000068"`,
		`"last_item_found": "TEST-000068"`,
	} {
		if !strings.Contains(stateMsg.Content, want) {
			t.Fatalf("workflow state message missing %q: %s", want, stateMsg.Content)
		}
	}
	if chat.Messages[2].Role != "user" || chat.Messages[2].Content == "" {
		t.Fatalf("user message after reserved state slot was overwritten: %+v", chat.Messages)
	}

	firstStateContent := chat.Messages[1].Content
	nextState := &WorkflowState{
		LastToolUsed:   "stock_check",
		LastItemSearch: "000068",
		LastItemFound:  "TEST-000068",
	}
	if err := chat.ApplyState(nextState); err != nil {
		t.Fatalf("re-apply state: %v", err)
	}

	if len(chat.Messages) != 3 {
		t.Fatalf("expected re-applying state not to duplicate messages, got %d messages=%+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[1].Content == firstStateContent {
		t.Fatalf("expected workflow state slot to be updated on re-apply")
	}
	if !strings.Contains(chat.Messages[1].Content, `"last_tool_used": "stock_check"`) {
		t.Fatalf("workflow state slot was not updated: %s", chat.Messages[1].Content)
	}
}

func TestApplyStateWithMissingReservedSlotDoesNotOverwriteUserMessage(t *testing.T) {
	state := &WorkflowState{
		LastToolUsed:  "item_details",
		LastItemFound: "TEST-000068",
	}
	chat := &Chat{
		Messages: []Messages{
			{Role: "system", Content: "base system prompt"},
			{Role: "user", Content: "How many do we have and what is our price?", turn: 1},
		},
	}

	if err := chat.ApplyState(state); err != nil {
		t.Fatalf("apply state: %v", err)
	}

	userMessages := 0
	for _, msg := range chat.Messages {
		if msg.Role == "user" && msg.Content == "How many do we have and what is our price?" {
			userMessages++
		}
	}
	if userMessages != 1 {
		t.Fatalf("expected ApplyState not to overwrite user message when reserved slot is missing, got %d messages=%+v", userMessages, chat.Messages)
	}
}

func TestApplySkillStateReplacesReservedSystemMessage(t *testing.T) {
	chat := &Chat{
		Messages: []Messages{
			{Role: "system", Content: "base system prompt"},
			{Role: "system", ToolName: "skill_catalog", Content: "Available workflow skills:\n- build-order"},
			{Role: "system", ToolName: "workflow_state", Content: ""},
			{Role: "system", ToolName: "skill_state", Content: ""},
			{Role: "user", Content: "Build an order", turn: 1},
		},
	}
	state := &SkillState{
		JobID:         1,
		SkillName:     "build-order",
		StateName:     StateStarted,
		EnteredReason: "User asked to build an order.",
		Status:        "active",
	}

	if err := chat.ApplySkillState(state); err != nil {
		t.Fatalf("apply skill state: %v", err)
	}
	if chat.skillState == nil || chat.skillState.SkillName != "build-order" {
		t.Fatalf("expected chat skill state to be applied, got %+v", chat.skillState)
	}
	if len(chat.Messages) != 5 {
		t.Fatalf("expected skill state replacement without duplication, got %d messages=%+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[1].ToolName != "skill_catalog" {
		t.Fatalf("skill catalog should not be overwritten: %+v", chat.Messages)
	}
	skillStateMsg := chat.Messages[3]
	if skillStateMsg.ToolName != "skill_state" {
		t.Fatalf("expected skill_state slot, got %+v", skillStateMsg)
	}
	for _, want := range []string{
		"Current Skill State:",
		`"skill_name": "build-order"`,
		`"state_name": "entered"`,
	} {
		if !strings.Contains(skillStateMsg.Content, want) {
			t.Fatalf("skill state message missing %q: %s", want, skillStateMsg.Content)
		}
	}
}

func TestRunRequiredDetailFollowUpsFetchesDetailsForCostQuestion(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(fakeItemDetailsTool{})

	chat := &Chat{
		registry: registry,
		Messages: []Messages{
			{Role: "user", Content: "What is our cost on TEST-PART-35-A?"},
		},
	}

	results := []ToolResult{
		{
			Name: "stock_check",
			Response: `{
				"count": 1,
				"results": [
					{
						"stock_number": "TEST-PART-35-A",
						"store_id": 1,
						"description": "Synthetic inventory component",
						"on_hand": 25,
						"confidence_score": 580,
						"match_type": "Exact Stock Number",
						"has_note": true
					}
				]
			}`,
		},
	}

	followUps := chat.RunRequiredDetailFollowUps(results)
	if len(followUps) != 1 {
		t.Fatalf("expected one required item_details follow-up, got %d", len(followUps))
	}
	if followUps[0].Name != "item_details" {
		t.Fatalf("expected item_details follow-up, got %+v", followUps[0])
	}
	if !strings.Contains(followUps[0].Response, `"Cost":12.34`) {
		t.Fatalf("expected detail response with cost, got %s", followUps[0].Response)
	}
	if chat.workflowState == nil || chat.workflowState.LastItemFound != "TEST-PART-35-A" {
		t.Fatalf("expected item_details follow-up to apply workflow state, got %+v", chat.workflowState)
	}
}

func TestRunRequiredDetailFollowUpsSkipsAvailabilityOnlyQuestion(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(fakeItemDetailsTool{})

	chat := &Chat{
		registry: registry,
		Messages: []Messages{
			{Role: "user", Content: "Do we stock TEST-PART-35-A?"},
		},
	}

	results := []ToolResult{
		{
			Name: "stock_check",
			Response: `{
				"count": 1,
				"results": [
					{
						"stock_number": "TEST-PART-35-A",
						"store_id": 1,
						"description": "Synthetic inventory component",
						"on_hand": 25,
						"confidence_score": 580,
						"match_type": "Exact Stock Number",
						"has_note": false
					}
				]
			}`,
		},
	}

	followUps := chat.RunRequiredDetailFollowUps(results)
	if len(followUps) != 0 {
		t.Fatalf("expected no detail follow-up for availability-only question, got %+v", followUps)
	}
}

func TestRunRequiredDetailFollowUpsSkipsAmbiguousStockCheckResults(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(fakeItemDetailsTool{})

	chat := &Chat{
		registry: registry,
		Messages: []Messages{
			{Role: "user", Content: "What is our cost on PART-35?"},
		},
	}

	results := []ToolResult{
		{
			Name: "stock_check",
			Response: `{
				"count": 2,
				"results": [
					{"stock_number": "TEST-PART-35-A", "store_id": 1, "description": "Synthetic inventory component A", "on_hand": 25, "confidence_score": 280, "match_type": "Weak", "has_note": false},
					{"stock_number": "TEST-PART-35-B", "store_id": 1, "description": "Synthetic inventory component B", "on_hand": 5, "confidence_score": 280, "match_type": "Weak", "has_note": false}
				]
			}`,
		},
	}

	followUps := chat.RunRequiredDetailFollowUps(results)
	if len(followUps) != 0 {
		t.Fatalf("expected no automatic detail follow-up for ambiguous candidates, got %+v", followUps)
	}
}

func TestParseSkillsRegistersSkillMetadata(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "build-order")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := []byte(`---
name: build-order
description: Build a reviewable order draft.
allowed_tools:
  - stock_check
  - item_details
requires_approval_for:
  - make_note
max_steps: 6
---
# Build Order Draft

Use this skill when the user wants to build an order.
`)
	if err := os.WriteFile(skillFile, content, 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	registry := NewSkillRegistry(nil)
	if err := registry.ParseSkills(dir); err != nil {
		t.Fatalf("parse skills: %v", err)
	}

	skill, ok := registry.Registry["build-order"]
	if !ok {
		t.Fatalf("expected build-order skill to be registered, got registry=%+v", registry.Registry)
	}
	if skill.Name != "build-order" {
		t.Fatalf("unexpected skill name: %q", skill.Name)
	}
	if skill.Description != "Build a reviewable order draft." {
		t.Fatalf("unexpected description: %q", skill.Description)
	}
	if skill.MaxSteps != 6 {
		t.Fatalf("unexpected max_steps: %d", skill.MaxSteps)
	}
	if len(skill.AllowedTools) != 2 || skill.AllowedTools[0] != "stock_check" || skill.AllowedTools[1] != "item_details" {
		t.Fatalf("unexpected allowed tools: %+v", skill.AllowedTools)
	}
	if len(skill.RestrictedTools) != 1 || skill.RestrictedTools[0] != "make_note" {
		t.Fatalf("unexpected restricted tools: %+v", skill.RestrictedTools)
	}
	if skill.Path != skillFile {
		t.Fatalf("expected skill path %q, got %q", skillFile, skill.Path)
	}
	for _, want := range []string{
		"# Build Order Draft",
		"Use this skill when the user wants to build an order.",
	} {
		if !strings.Contains(skill.Body, want) {
			t.Fatalf("expected skill body to contain %q, got %q", want, skill.Body)
		}
	}
}

func TestNewSkillRegistryIndexesTools(t *testing.T) {
	registry := NewSkillRegistry([]Tool{
		fakeTool{name: "approval_tool", permission: PermissionRequired},
		fakeTool{name: "readonly_tool", permission: PermissionNone},
		fakeItemDetailsTool{},
	})

	if len(registry.Registry) != 0 {
		t.Fatalf("expected empty skill metadata registry before ParseSkills, got %+v", registry.Registry)
	}
	required, ok := registry.Tools["approval_tool"]
	if !ok {
		t.Fatalf("expected approval_tool to be indexed, got %+v", registry.Tools)
	}
	if required.Permission() != PermissionRequired {
		t.Fatalf("expected approval_tool permission required, got %q", required.Permission())
	}
	readOnly, ok := registry.Tools["readonly_tool"]
	if !ok {
		t.Fatalf("expected readonly_tool to be indexed, got %+v", registry.Tools)
	}
	if readOnly.Permission() != PermissionNone {
		t.Fatalf("expected readonly_tool permission none, got %q", readOnly.Permission())
	}
	itemDetails, exists := registry.Tools["item_details"]
	if !exists {
		t.Fatalf("expected item_details to be indexed, got %+v", registry.Tools)
	}
	if itemDetails.Permission() != PermissionNone {
		t.Fatalf("expected item_details permission none, got %q", itemDetails.Permission())
	}
}

func TestModelToolsJSONUsesFunctionField(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(fakeTool{name: "readonly_tool", permission: PermissionNone})

	b, err := json.Marshal(registry.ModelTools())
	if err != nil {
		t.Fatalf("marshal model tools: %v", err)
	}
	raw := string(b)
	if !strings.Contains(raw, `"function"`) {
		t.Fatalf("expected model tool JSON to include function field, got %s", raw)
	}
	if strings.Contains(raw, "function,omitzero") {
		t.Fatalf("model tool JSON has invalid function tag: %s", raw)
	}
}

func TestSelectSkillReturnsSkillStateAndSession(t *testing.T) {
	skill := &Skill{
		Name:         "build-order",
		Description:  "Build a reviewable order draft.",
		AllowedTools: []string{"item_details"},
		Body:         "# Build Order\nUse item details to evaluate order candidates.",
	}
	registry := NewSkillRegistry([]Tool{
		fakeItemDetailsTool{},
		fakeTool{name: "approval_tool", permission: PermissionRequired},
	})
	registry.RegisterSkill(skill)
	chat := &Chat{}
	tool := &SelectSkill{Registry: registry, Chat: chat}

	tc := ToolCall{}
	tc.Function.Name = "select_skill"
	tc.Function.Arguments = map[string]any{
		"skill_name":     "build-order",
		"entered_reason": "User asked to build an order.",
	}

	result := tool.Execute(context.TODO(), tc)
	if result.Error != nil {
		t.Fatalf("select skill failed: %v", result.Error)
	}
	if result.SkillState == nil {
		t.Fatalf("expected select_skill to return skill state")
	}
	if result.SkillState.JobID == 0 {
		t.Fatalf("expected non-zero job id")
	}
	if result.SkillState.SkillName != "build-order" {
		t.Fatalf("unexpected skill name: %+v", result.SkillState)
	}
	if result.SkillState.StateName != StateStarted {
		t.Fatalf("unexpected state name: %+v", result.SkillState)
	}
	if result.SkillState.EnteredReason != "User asked to build an order." {
		t.Fatalf("unexpected entered reason: %+v", result.SkillState)
	}
	if chat.skillSession == nil {
		t.Fatalf("expected select_skill to attach a skill session to chat")
	}
	if _, ok := chat.skillSession.AllowedTools["item_details"]; !ok {
		t.Fatalf("expected allowed item_details tool, got %+v", chat.skillSession.AllowedTools)
	}
	if _, ok := chat.skillSession.AllowedTools["approval_tool"]; ok {
		t.Fatalf("did not expect disallowed approval_tool, got %+v", chat.skillSession.AllowedTools)
	}
	if chat.skillSession.Chat == nil {
		t.Fatalf("expected isolated skill chat to be initialized")
	}
	toolNames := modelToolNames(chat.skillSession.Chat.Tools)
	for _, want := range []string{"item_details", "update_skill_state", "complete_skill"} {
		if !toolNames[want] {
			t.Fatalf("expected isolated skill chat to expose %s, got %+v", want, chat.skillSession.Chat.Tools)
		}
	}
	if len(chat.skillSession.Chat.Messages) < 2 {
		t.Fatalf("expected isolated skill chat messages to be initialized, got %+v", chat.skillSession.Chat.Messages)
	}
	if !strings.Contains(chat.skillSession.Chat.Messages[0].Content, "Use item details to evaluate order candidates.") {
		t.Fatalf("expected isolated skill chat to include skill body, got %q", chat.skillSession.Chat.Messages[0].Content)
	}
	if chat.skillSession.Chat.Messages[1].ToolName != "skill_state" {
		t.Fatalf("expected isolated skill chat to include skill state slot, got %+v", chat.skillSession.Chat.Messages)
	}
}

func TestActiveSkillRejectsDisallowedTool(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(fakeTool{name: "readonly_tool", permission: PermissionNone})
	registry.Register(fakeTool{name: "approval_tool", permission: PermissionRequired})

	chat := &Chat{
		registry: registry,
		skillState: &SkillState{
			JobID:     1,
			SkillName: "test-skill",
			StateName: StateStarted,
			Status:    SkillStatusActive,
		},
		skillSession: &SkillSession{
			AllowedTools: ToolMap{
				"readonly_tool": fakeTool{name: "readonly_tool", permission: PermissionNone},
			},
		},
	}

	ai := AIMessage{ToolCalls: []ToolCall{newToolCall("approval_tool", nil)}}
	results := chat.RunToolCalls(ai)

	if len(results) != 1 {
		t.Fatalf("expected one tool result, got %+v", results)
	}
	if results[0].Error == nil {
		t.Fatalf("expected disallowed tool error, got %+v", results[0])
	}
	if !strings.Contains(results[0].Error.Error(), "not allowed") {
		t.Fatalf("expected not allowed error, got %v", results[0].Error)
	}
}

func TestActiveSkillBlocksPermissionRequiredTool(t *testing.T) {
	chat := &Chat{
		Messages: []Messages{
			{Role: "system", Content: "base"},
			{Role: "system", ToolName: "skill_state", Content: ""},
		},
		skillState: &SkillState{
			JobID:     2,
			SkillName: "permission-check",
			StateName: StateStarted,
			Status:    SkillStatusActive,
		},
		skillSession: &SkillSession{
			AllowedTools: ToolMap{
				"approval_tool": fakeTool{name: "approval_tool", permission: PermissionRequired},
			},
		},
	}

	ai := AIMessage{ToolCalls: []ToolCall{newToolCall("approval_tool", map[string]any{"note": "save this"})}}
	results := chat.RunToolCalls(ai)

	if len(results) != 1 {
		t.Fatalf("expected one tool result, got %+v", results)
	}
	if results[0].Error != nil {
		t.Fatalf("expected permission gate to return model-visible result, got error %v", results[0].Error)
	}
	if !strings.Contains(results[0].Response, "permission_required") {
		t.Fatalf("expected permission_required response, got %s", results[0].Response)
	}
	if chat.skillState == nil || chat.skillState.Status != SkillStatusWaitingPermission {
		t.Fatalf("expected skill state waiting permission, got %+v", chat.skillState)
	}
	if chat.skillState.PendingTool != "approval_tool" {
		t.Fatalf("expected pending tool approval_tool, got %+v", chat.skillState)
	}
	if chat.skillState.Data["pending_tool_args"] == nil {
		t.Fatalf("expected pending tool args to be stored, got %+v", chat.skillState.Data)
	}
	if !strings.Contains(chat.Messages[1].Content, `"status": "waiting_permission"`) {
		t.Fatalf("expected skill state system slot to be updated, got %+v", chat.Messages)
	}
}

func TestUpdateSkillStateToolUpdatesSharedState(t *testing.T) {
	state := &SkillState{
		JobID:     3,
		SkillName: "build-order",
		StateName: StateStarted,
		Status:    SkillStatusActive,
	}
	tool := &UpdateSkillState{State: state}

	result := tool.Execute(context.TODO(), newToolCall("update_skill_state", map[string]any{
		"summary":     "Reviewed two candidates.",
		"state_key":   "last_vendor",
		"state_value": "LINCOLN",
		"warning":     "Quantity still needs approval.",
	}))

	if result.Error != nil {
		t.Fatalf("update skill state failed: %v", result.Error)
	}
	if result.SkillState != state {
		t.Fatalf("expected shared state pointer to be returned")
	}
	if state.StateName != StateWorking || state.Status != SkillStatusActive {
		t.Fatalf("unexpected state after update: %+v", state)
	}
	if state.Summary != "Reviewed two candidates." {
		t.Fatalf("unexpected summary: %+v", state)
	}
	if state.Data["last_vendor"] != "LINCOLN" {
		t.Fatalf("expected state data to be written, got %+v", state.Data)
	}
	if len(state.Warnings) != 1 {
		t.Fatalf("expected warning to be stored, got %+v", state.Warnings)
	}
}

func TestCompleteSkillToolCompletesSharedState(t *testing.T) {
	state := &SkillState{
		JobID:     4,
		SkillName: "build-order",
		StateName: StateWorking,
		Status:    SkillStatusActive,
	}
	tool := &CompleteSkill{State: state}

	result := tool.Execute(context.TODO(), newToolCall("complete_skill", map[string]any{
		"summary": "Order draft ready for review.",
		"warning": "No quantities were finalized.",
	}))

	if result.Error != nil {
		t.Fatalf("complete skill failed: %v", result.Error)
	}
	if state.StateName != StateCompleted || state.Status != SkillStatusCompleted {
		t.Fatalf("expected completed state, got %+v", state)
	}
	if state.Summary != "Order draft ready for review." {
		t.Fatalf("unexpected summary: %+v", state)
	}
	if !strings.Contains(result.Response, "skill_completed") {
		t.Fatalf("expected completion response, got %s", result.Response)
	}
}

func TestRunActiveSkillCompletesAndSummarizesToParent(t *testing.T) {
	skill := &Skill{
		Name:         "build-order",
		Description:  "Build an order draft.",
		AllowedTools: []string{},
		MaxSteps:     3,
		Body:         "# Build Order",
	}
	session := CreateSession(skill, ToolMap{}, "User asked for an order.")
	parent := &Chat{
		Model:        "fake-model",
		skillState:   session.State,
		skillSession: session,
		Messages: []Messages{
			{Role: "system", Content: "parent"},
			{Role: "system", ToolName: "skill_state", Content: ""},
		},
	}

	calls := 0
	session.Chat.sendOnceFunc = func(c *Chat) (*Answer, error) {
		calls++
		if calls == 1 {
			return &Answer{
				Model: "fake-model",
				Message: AIMessage{
					Role:      "assistant",
					ToolCalls: []ToolCall{newToolCall("complete_skill", map[string]any{"summary": "Order draft ready."})},
				},
				Done: true,
			}, nil
		}
		return &Answer{
			Model: "fake-model",
			Message: AIMessage{
				Role:    "assistant",
				Content: "Final model wording.",
			},
			Done: true,
		}, nil
	}

	answer, err := parent.RunActiveSkill("Build the order.")
	if err != nil {
		t.Fatalf("run active skill: %v", err)
	}
	if answer.Message.Content != "Order draft ready." {
		t.Fatalf("expected completed skill summary to be returned, got %q", answer.Message.Content)
	}
	if parent.skillSession != nil {
		t.Fatalf("expected completed skill session to be cleared")
	}
	if parent.skillState == nil || parent.skillState.StateName != StateCompleted {
		t.Fatalf("expected parent skill state to be completed, got %+v", parent.skillState)
	}
	foundSummary := false
	for _, msg := range parent.Messages {
		if msg.ToolName == "skill_summary" && strings.Contains(msg.Content, "Order draft ready.") {
			foundSummary = true
		}
	}
	if !foundSummary {
		t.Fatalf("expected parent chat to receive skill summary, got %+v", parent.Messages)
	}
}

func TestParentSendMessageStopsAfterSkillSelection(t *testing.T) {
	skillRegistry := NewSkillRegistry([]Tool{})
	skillRegistry.RegisterSkill(&Skill{
		Name:         "build-order",
		Description:  "Build an order draft.",
		AllowedTools: []string{},
		MaxSteps:     3,
	})
	chat := &Chat{
		Model:         "fake-model",
		registry:      &Registry{tools: map[string]Tool{}},
		skillregistry: skillRegistry,
		Messages: []Messages{
			{Role: "system", Content: "parent"},
			{Role: "system", ToolName: "skill_state", Content: ""},
		},
	}
	chat.registry.Register(&SelectSkill{Registry: skillRegistry, Chat: chat})

	calls := 0
	chat.sendOnceFunc = func(c *Chat) (*Answer, error) {
		calls++
		if calls > 1 {
			t.Fatalf("parent model should stop after skill selection")
		}
		return &Answer{
			Model: "fake-model",
			Message: AIMessage{
				Role: "assistant",
				ToolCalls: []ToolCall{newToolCall("select_skill", map[string]any{
					"skill_name":     "build-order",
					"entered_reason": "User asked to build an order.",
				})},
			},
			Done: true,
		}, nil
	}

	answer, err := chat.SendMessage()
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one parent model call, got %d", calls)
	}
	if !chat.HasActiveSkill() {
		t.Fatalf("expected active skill after select_skill")
	}
	if answer.Message.Content != "Starting workflow skill." {
		t.Fatalf("unexpected handoff answer: %q", answer.Message.Content)
	}
}

func TestRunActiveSkillApprovalExecutesPendingTool(t *testing.T) {
	recorder := &recordingTool{name: "approval_tool", permission: PermissionRequired}
	skill := &Skill{
		Name:         "permission-check",
		Description:  "Check permission behavior.",
		AllowedTools: []string{"approval_tool"},
		MaxSteps:     3,
	}
	session := CreateSession(skill, ToolMap{"approval_tool": recorder}, "Need permission.")
	parent := &Chat{
		Model:        "fake-model",
		skillState:   session.State,
		skillSession: session,
		Messages: []Messages{
			{Role: "system", Content: "parent"},
			{Role: "system", ToolName: "skill_state", Content: ""},
		},
	}

	results := parent.RunToolCalls(AIMessage{ToolCalls: []ToolCall{
		newToolCall("approval_tool", map[string]any{"note": "save this"}),
	}})
	if len(results) != 1 || !strings.Contains(results[0].Response, "permission_required") {
		t.Fatalf("expected permission request, got %+v", results)
	}
	if recorder.calls != 0 {
		t.Fatalf("permission-required tool should not execute before approval")
	}

	session.Chat.sendOnceFunc = func(c *Chat) (*Answer, error) {
		return &Answer{
			Model: "fake-model",
			Message: AIMessage{
				Role:    "assistant",
				Content: "Permissioned tool ran.",
			},
			Done: true,
		}, nil
	}

	answer, err := parent.RunActiveSkill("approve")
	if err != nil {
		t.Fatalf("run active skill approval: %v", err)
	}
	if recorder.calls != 1 {
		t.Fatalf("expected pending tool to execute once after approval, got %d", recorder.calls)
	}
	if parent.skillState.Status != SkillStatusActive || parent.skillState.PendingTool != "" {
		t.Fatalf("expected pending permission to clear, got %+v", parent.skillState)
	}
	if answer.Message.Content != "Permissioned tool ran." {
		t.Fatalf("unexpected answer after approval: %q", answer.Message.Content)
	}
}

func TestAppExposesAndResolvesPendingToolPermission(t *testing.T) {
	recorder := &recordingTool{name: "approval_tool", permission: PermissionRequired}
	skill := &Skill{
		Name:         "permission-check",
		Description:  "Check permission behavior.",
		AllowedTools: []string{"approval_tool"},
		MaxSteps:     3,
	}
	session := CreateSession(skill, ToolMap{"approval_tool": recorder}, "Validate frontend permission flow.")
	parent := &Chat{
		Model:        "fake-model",
		skillState:   session.State,
		skillSession: session,
		Messages: []Messages{
			{Role: "system", Content: "parent"},
			{Role: "system", ToolName: "skill_state", Content: ""},
		},
	}
	app := &App{Thread: parent}

	parent.RunToolCalls(AIMessage{ToolCalls: []ToolCall{
		newToolCall("approval_tool", map[string]any{"note": "save this"}),
	}})

	request := app.GetPendingPermission()
	if request == nil {
		t.Fatal("expected a pending permission request")
	}
	if request.Tool != "approval_tool" || request.Description != "recording tool" {
		t.Fatalf("unexpected permission request: %+v", request)
	}
	if request.SkillName != skill.Name || request.JobID != session.State.JobID {
		t.Fatalf("expected skill metadata in permission request: %+v", request)
	}
	if request.Arguments["note"] != "save this" {
		t.Fatalf("expected pending arguments in permission request: %+v", request.Arguments)
	}
	if recorder.calls != 0 {
		t.Fatalf("permissioned tool ran before frontend approval")
	}

	session.Chat.sendOnceFunc = func(c *Chat) (*Answer, error) {
		return &Answer{
			Model: "fake-model",
			Message: AIMessage{
				Role:    "assistant",
				Content: "Approved action completed.",
			},
			Done: true,
		}, nil
	}

	response, err := app.ResolveToolPermission(true)
	if err != nil {
		t.Fatalf("resolve frontend permission: %v", err)
	}
	if recorder.calls != 1 {
		t.Fatalf("expected approved tool to run once, got %d", recorder.calls)
	}
	if response != "Approved action completed." {
		t.Fatalf("unexpected permission response: %q", response)
	}
	if app.GetPendingPermission() != nil {
		t.Fatal("expected pending permission to clear after approval")
	}
}

func TestClearChatPreservesSkillRegistryAndSelectSkillPointer(t *testing.T) {
	toolRegistry := &Registry{tools: map[string]Tool{}}
	skillRegistry := NewSkillRegistry([]Tool{fakeItemDetailsTool{}})
	skillRegistry.RegisterSkill(&Skill{
		Name:         "build-order",
		Description:  "Build an order draft.",
		AllowedTools: []string{"item_details"},
	})

	oldChat := &Chat{skillregistry: skillRegistry}
	selectSkill := &SelectSkill{Registry: skillRegistry, Chat: oldChat}
	toolRegistry.Register(selectSkill)

	app := &App{
		Thread:       oldChat,
		ToolRegistry: toolRegistry,
	}

	app.ClearChat()

	if app.Thread == nil {
		t.Fatalf("expected clear chat to create a new chat")
	}
	if app.Thread == oldChat {
		t.Fatalf("expected clear chat to replace old chat")
	}
	if app.Thread.skillregistry != skillRegistry {
		t.Fatalf("expected skill registry to be preserved")
	}
	if selectSkill.Chat != app.Thread {
		t.Fatalf("expected select_skill chat pointer to be updated after clear")
	}
	if len(app.Thread.Tools) != 1 || app.Thread.Tools[0].Function.Name != "select_skill" {
		t.Fatalf("expected model tools to be rebuilt, got %+v", app.Thread.Tools)
	}

	var hasCatalog, hasWorkflowState, hasSkillState bool
	for _, msg := range app.Thread.Messages {
		switch msg.ToolName {
		case "skill_catalog":
			hasCatalog = strings.Contains(msg.Content, "build-order")
		case "workflow_state":
			hasWorkflowState = true
		case "skill_state":
			hasSkillState = true
		}
	}
	if !hasCatalog || !hasWorkflowState || !hasSkillState {
		t.Fatalf("expected system messages to include catalog/workflow/skill slots, got %+v", app.Thread.Messages)
	}
}

type fakeItemDetailsTool struct{}

func (fakeItemDetailsTool) Name() string {
	return "item_details"
}

func (fakeItemDetailsTool) Description() string {
	return "fake item details"
}

func (fakeItemDetailsTool) Parameters() Parameters {
	return Parameters{Type: "object"}
}

func (fakeItemDetailsTool) Execute(ctx context.Context, tc ToolCall) ToolResult {
	sku, _ := tc.Function.Arguments["stock_number"].(string)
	return ToolResult{
		Name:     "item_details",
		Response: `{"item":{"stock_number":"` + sku + `","Cost":12.34,"Price":19.99}}`,
		State: &WorkflowState{
			LastToolUsed:   "item_details",
			LastItemSearch: sku,
			LastItemFound:  sku,
		},
	}
}

func (fakeItemDetailsTool) Permission() ToolPermission {
	return PermissionNone
}

type fakeTool struct {
	name       string
	permission ToolPermission
}

func (t fakeTool) Name() string {
	return t.name
}

func (t fakeTool) Description() string {
	return "fake skill tool"
}

func (t fakeTool) Parameters() Parameters {
	return Parameters{Type: "object"}
}

func (t fakeTool) Execute(ctx context.Context, tc ToolCall) ToolResult {
	return ToolResult{Name: t.name, Response: "{}"}
}

func (t fakeTool) Permission() ToolPermission {
	return t.permission
}

type recordingTool struct {
	name       string
	permission ToolPermission
	calls      int
	lastArgs   map[string]any
}

func (t *recordingTool) Name() string {
	return t.name
}

func (t *recordingTool) Description() string {
	return "recording tool"
}

func (t *recordingTool) Parameters() Parameters {
	return Parameters{Type: "object"}
}

func (t *recordingTool) Execute(ctx context.Context, tc ToolCall) ToolResult {
	t.calls++
	t.lastArgs = tc.Function.Arguments
	return ToolResult{Name: t.name, Response: `{"status":"executed"}`}
}

func (t *recordingTool) Permission() ToolPermission {
	return t.permission
}

func newToolCall(name string, args map[string]any) ToolCall {
	if args == nil {
		args = map[string]any{}
	}
	tc := ToolCall{}
	tc.Function.Name = name
	tc.Function.Arguments = args
	return tc
}

func modelToolNames(tools []ModelTool) map[string]bool {
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Function.Name] = true
	}
	return names
}
