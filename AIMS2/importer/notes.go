package importer

import (
	"cmp"
	"database/sql"
	"fmt"
	"slices"
)

type Note struct {
	ID            int     `json:"id,omitempty"`
	StockNumber   string  `json:"stock_number"`
	Location      int     `json:"location,omitempty"`
	Note          string  `json:"note,omitempty"`
	IsCritical    bool    `json:"is_critical,omitempty"`
	MinimumOnHand float32 `json:"minimum_on_hand,omitempty"`
	TargetOnHand  float32 `json:"target_on_hand,omitempty"`
	db            *sql.DB
}

func NewNote(sku string, location int, db *sql.DB) *Note {
	note := &Note{
		StockNumber: sku,
		Location:    location,
		db:          db,
	}
	return note
}

func GetNote(sku string, location int, db *sql.DB) ([]*Note, error) {
	stmt, err := db.Prepare(
		`SELECT stock_number,
		location,
	note,
		is_critical,
		minimum_on_hand,
		target_on_hand

		FROM inventory_notes
		WHERE(
		stock_number=?
		AND 
		location=?)
		LIMIT 5
	`,
	)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(sku, location)
	if err != nil {
		return nil, err
	}
	var na []*Note
	for rows.Next() {
		note := &Note{}
		err := rows.Scan(&note.StockNumber, &note.Location, &note.Note, &note.IsCritical, &note.MinimumOnHand, &note.TargetOnHand)
		if err != nil {
			return nil, err
		}
		na = append(na, note)
	}
	return na, err
}

func (note *Note) WriteNote() error {
	db := note.db
	err := db.Ping()
	if err != nil {
		return err
	}
	stmt, err := db.Prepare(`
		INSERT INTO inventory_notes (
		stock_number,
		location,
		note,
		is_critical,
		minimum_on_hand,
		target_on_hand
		) 
		VALUES(?,?,?,?,?,?)
		ON CONFLICT (stock_number)
		DO UPDATE SET 
		note=excluded.note,
		is_critical=excluded.is_critical,
		minimum_on_hand=excluded.minimum_on_hand,
		target_on_hand=excluded.target_on_hand
		`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(&note.StockNumber, &note.Location, &note.Note, &note.IsCritical, &note.MinimumOnHand, &note.TargetOnHand)
	if err != nil {
		return err
	}
	return err
}

type OrderItem struct {
	Location     float64 `json:"store_id"`
	StockNumber  string  `json:"stock_number"`
	VendorNumber string  `json:"vendor_number"`
	OnHand       float64 `json:"on_hand"`
	LastReceipt  string  `json:"last_receipt"`
	LastOrder    string  `json:"last_order"`
	priority     float64
	Note         *Note `json:"note,omitempty"`
}

func GetCriticalItems() ([]*OrderItem, error) {
	db, err := OpenDB()
	var items []*OrderItem
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v\n", err)
	}
	defer db.Close()
	stmt, err := db.Prepare(`
		SELECT stock_number, minimum_on_hand, is_critical, note
		FROM inventory_notes
		WHERE (is_critical=true)`)
	if err != nil {
		return nil, fmt.Errorf("error on preparing query statement for critical items: %v\n", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("error on querying cricitcal items: %v\n", err)
	}
	for rows.Next() {
		item := &OrderItem{
			Note: &Note{},
		}
		if err := rows.Scan(&item.StockNumber,
			&item.Note.MinimumOnHand,
			&item.Note.IsCritical,
			&item.Note.Note); err != nil {
			return nil, fmt.Errorf("error scanning critical item row: %v\n", err)
		}
		result, err := SearchInventory(InventoryLookUpArgs{
			Query:   item.StockNumber,
			InStock: false,
		})
		if err != nil {
			return nil, fmt.Errorf("error on finding cricitcal item in inventory: %v\n", err)
		}
		if len(result) == 0 {
			fmt.Println("Note results found on critical item")
			continue
		}
		rslt := result[0]
		item.Location = rslt.Item.Location
		item.VendorNumber = rslt.Item.VendorNumber
		item.OnHand = rslt.Item.OnHand
		item.LastReceipt = rslt.Item.LastReceipt
		items = append(items, item)
	}
	return items, err
}

func sortAndDedupe(items []*OrderItem) ([]*OrderItem, error) {
	seen := make(map[string]struct{}, len(items))

	deduped := items[:0]

	for _, item := range items {
		if item == nil {
			continue
		}

		if _, exists := seen[item.StockNumber]; exists {
			continue
		}

		seen[item.StockNumber] = struct{}{}

		if item.Note == nil {
			return nil, fmt.Errorf("missing note for stock number %s", item.StockNumber)
		}

		if item.Note.TargetOnHand <= 0 {
			return nil, fmt.Errorf("invalid target on hand for stock number %s", item.StockNumber)
		}

		item.priority = item.OnHand / float64(item.Note.TargetOnHand)

		deduped = append(deduped, item)
	}

	slices.SortStableFunc(deduped, func(a, b *OrderItem) int {
		return cmp.Compare(a.priority, b.priority)
	})

	return deduped, nil
}

func GetPriorityVendor() error {
	critical, err := GetCriticalItems()
	if err != nil {
		return err
	}
	_, err = sortAndDedupe(critical)
	return err
}
