package main

import (
	"AIMS2/importer"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			Arguments: map[string]any{"query": "3M-16114"},
		},
	}

	tool := &StockCheck{}

	testMsg := tool.Execute(context.TODO(), tc)
	if testMsg.Error != nil {
		t.Fatalf("stock check failed: %v", testMsg.Error)
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
			Arguments: map[string]any{"query": "3M-16114"},
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
	if result.State.LastItemSearch != "3M-16114" {
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
					Arguments: map[string]any{"query": "3M-16114"},
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
	if chat.workflowState.LastItemSearch != "3M-16114" {
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
		`"last_item_searched": "3M-16114"`,
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
				StockNumber:      "3M-16114",
				Location:         1,
				Description:      "Example inventory item",
				VendorNumber:     "3M",
				VendorProdNumber: "16114",
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
	t.Logf("compact stock_check response: %s", raw)

	var response StockCheckResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		t.Fatalf("unmarshal compact response: %v", err)
	}
	t.Logf("decoded response: count=%d results=%d message=%q",
		response.Count,
		len(response.Results),
		response.Message,
	)

	if response.Count != 1 || len(response.Results) != 1 {
		t.Fatalf("expected one result, got count=%d results=%d", response.Count, len(response.Results))
	}

	match := response.Results[0]
	t.Logf(
		"decoded match: stock_number=%q description=%q on_hand=%v confidence_score=%d match_type=%q",
		match.StockNumber,
		match.Description,
		match.OnHand,
		match.ConfidenceScore,
		match.MatchType,
	)
	if match.StockNumber != "3M-16114" || match.StoreID != 1 || match.OnHand != 4 || match.ConfidenceScore != 100 {
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
		} else {
			t.Logf("confirmed omitted field: %s", omitted)
		}
	}
}

func TestStockCheckParseInventoryDataIncludesNoteWhenPresent(t *testing.T) {
	tool := &StockCheck{}
	results := []*importer.ReturnResult{
		{
			Item: importer.InventoryItem{
				Location:    1,
				StockNumber: "3M-16114",
				Description: "Example inventory item",
				OnHand:      4,
			},
			Note: &importer.Note{
				StockNumber:   "3M-16114",
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
	t.Logf("compact stock_check response with note: %s", raw)

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
		LastItemFound:  "ME-000068",
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
		`"last_item_found": "ME-000068"`,
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
		LastItemFound:  "ME-000068",
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
		LastItemFound: "ME-000068",
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

func TestRunRequiredDetailFollowUpsFetchesDetailsForCostQuestion(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(fakeItemDetailsTool{})

	chat := &Chat{
		registry: registry,
		Messages: []Messages{
			{Role: "user", Content: "What is our cost on WLM-WMK14T-35?"},
		},
	}

	results := []ToolResult{
		{
			Name: "stock_check",
			Response: `{
				"count": 1,
				"results": [
					{
						"stock_number": "WLM-WMK14T-35",
						"store_id": 1,
						"description": "CONTACT TIP .035 TAPER",
						"on_hand": 190,
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
	if !strings.Contains(followUps[0].Response, `"Cost":0.6588`) {
		t.Fatalf("expected detail response with cost, got %s", followUps[0].Response)
	}
	if chat.workflowState == nil || chat.workflowState.LastItemFound != "WLM-WMK14T-35" {
		t.Fatalf("expected item_details follow-up to apply workflow state, got %+v", chat.workflowState)
	}
}

func TestRunRequiredDetailFollowUpsSkipsAvailabilityOnlyQuestion(t *testing.T) {
	registry := &Registry{tools: map[string]Tool{}}
	registry.Register(fakeItemDetailsTool{})

	chat := &Chat{
		registry: registry,
		Messages: []Messages{
			{Role: "user", Content: "Do we stock WLM-WMK14T-35?"},
		},
	}

	results := []ToolResult{
		{
			Name: "stock_check",
			Response: `{
				"count": 1,
				"results": [
					{
						"stock_number": "WLM-WMK14T-35",
						"store_id": 1,
						"description": "CONTACT TIP .035 TAPER",
						"on_hand": 190,
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
			{Role: "user", Content: "What is our cost on 14T-35?"},
		},
	}

	results := []ToolResult{
		{
			Name: "stock_check",
			Response: `{
				"count": 2,
				"results": [
					{"stock_number": "WLM-WMK14T-35", "store_id": 1, "description": "CONTACT TIP .035 TAPER", "on_hand": 190, "confidence_score": 280, "match_type": "Weak", "has_note": false},
					{"stock_number": "WLM-WMK14T-35-5", "store_id": 1, "description": "CONTACT TIP .035 TAPERED", "on_hand": 10, "confidence_score": 280, "match_type": "Weak", "has_note": false}
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

	registry := NewSkillRegistry()
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
		Response: `{"item":{"stock_number":"` + sku + `","Cost":0.6588,"Price":1.88}}`,
		State: &WorkflowState{
			LastToolUsed:   "item_details",
			LastItemSearch: sku,
			LastItemFound:  sku,
		},
	}
}
