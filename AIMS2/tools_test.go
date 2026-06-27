package main

import (
	"AIMS2/importer"
	"context"
	"encoding/json"
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
