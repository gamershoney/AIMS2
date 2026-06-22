package importer

import "database/sql"

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
	_, err = stmt.Exec(&note.StockNumber, &note.Location, &note.Note, &note.IsCritical, &note.MinimumOnHand, &note.TargetOnHand)
	if err != nil {
		return err
	}
	return err
}
