package importer

import (
	"fmt"
	"testing"
)

func TestOpenDB(t *testing.T) {
	db, err := OpenDB()
	if err != nil {
		t.Error(err)
	}
	err = db.Ping()
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

func TestNotes(t *testing.T) {
	db, err := OpenDB()
	if err != nil {
		t.Error(err)
	}
	note := NewNote("BRN-1598", 1, db)

	note.Note = "This is a test note"
	err = note.WriteNote()
	if err != nil {
		t.Error(err)
	}
	note.Note = "We rewrote this note lol"
	err = note.WriteNote()
	if err != nil {
		t.Error(err)
	}
	notes, err := GetNote("BRN-1598", 1, db)
	if err != nil {
		t.Error(err)
	}
	for _, v := range notes {
		fmt.Printf("read note: %+v\t %+v\n", v.StockNumber, v.Note)
	}
}

func TestGetCriticalItems(t *testing.T) {
	items, err := GetPriorityVendor()
	if err != nil {
		t.Errorf("error: %v", err)
	}
	for _, v := range items {
		fmt.Printf("item %s\t %d\n", v.Vendor, v.Items)
	}
}
