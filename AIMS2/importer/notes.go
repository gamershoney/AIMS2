package importer

import (
	"cmp"
	"database/sql"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
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
	defer rows.Close()
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
	_, err = stmt.Exec(&note.StockNumber, &note.Location, &note.Note, &note.IsCritical, &note.MinimumOnHand, &note.TargetOnHand)
	if err != nil {
		return err
	}
	return err
}

type OrderItem struct {
	Location     float64 `json:"store_id"`
	StockNumber  string  `json:"stock_number"`
	VendorNumber string  `json:"vendor_number,omitempty"`
	OnHand       float64 `json:"on_hand,omitzero"`
	LastReceipt  string  `json:"last_receipt,omitempty"`
	LastOrder    string  `json:"last_order,omitempty"`
	Usage        float64 `json:"monthly_usage,omitzero"`
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
		SELECT stock_number, minimum_on_hand,target_on_hand,
		is_critical,
		note
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
	defer rows.Close()
	for rows.Next() {
		item := &OrderItem{
			Note: &Note{},
		}
		if err := rows.Scan(&item.StockNumber,
			&item.Note.MinimumOnHand,
			&item.Note.TargetOnHand,
			&item.Note.IsCritical,
			&item.Note.Note); err != nil {
			return nil, fmt.Errorf("scan critical inventory note: %w", err)
		}
		result, err := SearchInventory(InventoryLookUpArgs{
			Query:   item.StockNumber,
			InStock: false,
		})
		if err != nil {
			return nil, fmt.Errorf("error on finding cricitcal item in inventory: %v\n", err)
		}
		if len(result) == 0 {
			continue
		}
		rslt := result[0]
		item.Location = rslt.Item.Location
		item.VendorNumber = rslt.Item.VendorNumber
		item.OnHand = rslt.Item.OnHand
		item.LastReceipt = rslt.Item.LastReceipt
		items = append(items, item)
	}
	return items, rows.Err()
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

func GetHighTurn() ([]*OrderItem, error) {
	var err error
	db, err := OpenDB()
	if err != nil {
		return nil, err
	}
	stmt, err := db.Prepare(`
		SELECT 
		i.location,
		i.stock_number,
		i.vendor_number,
		i.on_hand,
		i.last_receipt,
		i.last_order,
		i.avg_usage,
		COALESCE (n.note , '') AS note
		FROM inventory i 
		LEFT JOIN inventory_notes n
		USING (stock_number, location)
		WHERE i.location=1 
		AND
		i.on_hand<i.avg_usage*0.8
		AND i.avg_usage>0
		;
		
		`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	var items []*OrderItem
	for rows.Next() {
		item := &OrderItem{Note: &Note{}}
		err = rows.Scan(&item.Location,
			&item.StockNumber,
			&item.VendorNumber,
			&item.OnHand,
			&item.LastReceipt,
			&item.LastOrder,
			&item.Usage,
			&item.Note.Note)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, err
}

type VendorCount struct {
	Vendor string `json:"vendor_number"`
	Items  int    `json:"low_items"`
}

func GetPriorityVendor() ([]*VendorCount, error) {
	critical, err := GetCriticalItems()
	if err != nil {
		return nil, err
	}
	high, err := GetHighTurn()
	if err != nil {
		return nil, err
	}
	critical = append(critical, high...)

	vendors := make(map[string]int)
	excludedVendors := make(map[string]struct{})
	for _, vendor := range strings.Split(os.Getenv("AIMS2_EXCLUDED_VENDOR_CODES"), ",") {
		if vendor = strings.ToUpper(strings.TrimSpace(vendor)); vendor != "" {
			excludedVendors[vendor] = struct{}{}
		}
	}
	for _, v := range critical {
		if _, excluded := excludedVendors[strings.ToUpper(strings.TrimSpace(v.VendorNumber))]; excluded {
			continue
		}
		vendors[v.VendorNumber] += 1
	}
	maps.DeleteFunc(vendors, func(k string, v int) bool {
		return v < 2
	})
	var vcount []*VendorCount
	for i, v := range vendors {
		vendor := &VendorCount{
			Vendor: i,
			Items:  v,
		}
		vcount = append(vcount, vendor)
	}
	slices.SortFunc(vcount, func(a, b *VendorCount) int {
		return cmp.Compare(b.Items, a.Items)
	})
	if len(vcount) > 6 {
		vcount = vcount[:6]
	}
	return vcount, err
}
