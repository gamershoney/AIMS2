package importer

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
)

var TableName string = "inventory"

type InventoryItem struct {
	ID               int
	Location         int
	StockNumber      string
	Description      string
	VendorNumber     string
	VendorProdNumber string
	Unit             string
	Price            int
	Cost             int
	OnHand           int
	Usage            int
	LastSale         string
	LastReceipt      string
	LastOrder        string
}

//go:embed schema.sql
var dbSQL string

func OpenDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "./inventory.db")
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	if dbSQL == "" {
		return nil, errors.New("error: empty or no file found")
	}
	schema, err := db.Prepare(dbSQL)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := schema.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	defer func() {
		err := db.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	_, err = schema.Exec()
	if err != nil {
		return nil, err
	}

	fmt.Println("DB initialized with tables ")
	return db, err
}

func AddInventory(db *sql.DB, i []InventoryItem) error {
	istmt := fmt.Sprintf(`INSERT INTO %s(
	id,
	location,
	stock_number,
  description,
  vendor_number,
  vendor_product_number,
  unit,
  price,
  cost,
  on_hand,
  avg_usage,
  last_sale,
  last_receipt,
  last_order) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, TableName)
	prep, err := db.Prepare(istmt)
	if err != nil {
		return err
	}
	defer func() {
		err := prep.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()

	for id, item := range i {
		if _, err := prep.Exec(
			id+1,
			item.StockNumber,
			item.Description,
			item.VendorNumber,
			item.Unit,
			item.Price,
			item.Cost,
			item.OnHand,
			item.Usage,
			item.LastSale,
			item.LastReceipt,
			item.LastOrder,
		); err != nil {
			return err
		}
	}
	return err
}
