package importer

import (
	"fmt"
	"testing"
)

func TestTestOpen(t *testing.T) {
	err := TestOpen()
	if err != nil {
		t.Error(err)
	}
}

func TestSearchInventoryByStockNumber(t *testing.T) {
	results, err := SearchInventory(InventoryLookUpArgs{
		InStock:  true,
		Query:    "1598",
		Location: "1",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("search inventory: %v", err)
	}
	if len(results) <= 0 {
		t.Fatalf("error no results found.")
	}

	for _, v := range results {
		fmt.Printf("Result: %+v\n", v)
	}
}
