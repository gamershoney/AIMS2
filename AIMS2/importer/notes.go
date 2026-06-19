package importer

import "database/sql"

type Note struct {
	ID            int     `json:"id"`
	StockNumber   string  `json:"stock_number"`
	Location      int     `json:"location"`
	Note          string  `json:"note"`
	IsCritical    bool    `json:"is_critical"`
	MinimumOnHand float32 `json:"minimum_on_hand"`
	TargetOnHand  float32 `json:"target_on_hand"`
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

func GetNote(sku string, location int) ([]*Note, error) {
	db, err := OpenDB()
	if err != nil {
		return nil, err
	}
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
		err := rows.Scan(note.StockNumber, note.Location, note.Note, note.MinimumOnHand, note.TargetOnHand)
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
		VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(note.StockNumber, note.Location, note.Note, note.IsCritical, note.MinimumOnHand, note.TargetOnHand)
	if err != nil {
		return err
	}
	return err
}
