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
	if match.StockNumber != "3M-16114" || match.OnHand != 4 || match.ConfidenceScore != 100 {
		t.Fatalf("unexpected compact match: %+v", match)
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
