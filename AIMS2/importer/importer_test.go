package importer

import (
	"testing"
)

func TestSearchInventoryByStockNumber(t *testing.T) {
	results, err := SearchInventory(InventoryLookUpArgs{
		Query:    "3M-16114",
		Location: "1",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("search inventory: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected at least one result")
	}

	got := results[0]

	if got.Item.StockNumber != "3M-16114" {
		t.Fatalf("expected stock number %q, got %q", "3M-16114", got.Item.StockNumber)
	}

	if got.Item.Location != 1 {
		t.Fatalf("expected location 1, got %d", got.Item.Location)
	}

	t.Logf("found item: %+v", got)
}
