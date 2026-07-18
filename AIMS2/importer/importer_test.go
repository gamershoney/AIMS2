package importer

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func testDatabase(t *testing.T, items ...*InventoryItem) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "inventory.db")
	t.Setenv("AIMS2_INVENTORY_DB_PATH", dbPath)
	t.Setenv("AIMS2_EXCLUDED_VENDOR_CODES", "")
	db, err := OpenDB()
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if len(items) > 0 {
		if err := AddInventory(db, items); err != nil {
			t.Fatalf("seed temporary database: %v", err)
		}
	}
	return db
}

func fixtureItem(stockNumber, vendor string) *InventoryItem {
	return &InventoryItem{
		Location:     1,
		StockNumber:  stockNumber,
		Description:  "Test inventory item",
		VendorNumber: vendor,
		OnHand:       12,
		Usage:        4,
	}
}

func TestOpenDB(t *testing.T) {
	db := testDatabase(t)
	if err := db.Ping(); err != nil {
		t.Fatalf("ping temporary database: %v", err)
	}
}

func TestSearchInventoryByStockNumber(t *testing.T) {
	testDatabase(t, fixtureItem("TEST-1598", "TEST"))
	results, err := SearchInventory(InventoryLookUpArgs{
		InStock:  true,
		Query:    "TEST-1598",
		Location: "1",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("search inventory: %v", err)
	}
	if len(results) != 1 || results[0].Item.StockNumber != "TEST-1598" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}

func TestNotes(t *testing.T) {
	db := testDatabase(t, fixtureItem("TEST-NOTE", "TEST"))
	note := NewNote("TEST-NOTE", 1, db)
	note.Note = "First test note"
	if err := note.WriteNote(); err != nil {
		t.Fatalf("write note: %v", err)
	}
	note.Note = "Updated test note"
	if err := note.WriteNote(); err != nil {
		t.Fatalf("update note: %v", err)
	}

	notes, err := GetNote("TEST-NOTE", 1, db)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if len(notes) != 1 || notes[0].Note != "Updated test note" {
		t.Fatalf("unexpected notes: %+v", notes)
	}
}

func TestGetPriorityVendor(t *testing.T) {
	first := fixtureItem("TEST-LOW-1", "TEST")
	first.OnHand = 1
	first.Usage = 10
	second := fixtureItem("TEST-LOW-2", "TEST")
	second.OnHand = 2
	second.Usage = 10
	testDatabase(t, first, second)

	items, err := GetPriorityVendor()
	if err != nil {
		t.Fatalf("get priority vendor: %v", err)
	}
	if len(items) != 1 || items[0].Vendor != "TEST" || items[0].Items != 2 {
		t.Fatalf("unexpected priority vendors: %+v", items)
	}
}
