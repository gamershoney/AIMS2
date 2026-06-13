package importer

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
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
	db, err := sql.Open("sqlite", "./importer/inventory.db")
	if err != nil {
		fmt.Printf("error: could not connect to db:%v\n ", err)
		err = errors.New("could not connect to DB")
		return nil, err
	}

	if err = db.Ping(); err != nil {
		fmt.Println("DB not connected")
		return nil, fmt.Errorf("error pinging db: %v", err)
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

	_, err = schema.Exec()
	if err != nil {
		return nil, err
	}

	fmt.Println("DB initialized with tables ")
	return db, err
}

func AddInventory(db *sql.DB, i []*InventoryItem) error {
	istmt := fmt.Sprintf(`INSERT INTO %s(

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
  last_order) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, TableName)
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

	for _, item := range i {
		if _, err := prep.Exec(

			item.Location,
			item.StockNumber,
			item.Description,
			item.VendorNumber,
			item.VendorProdNumber,
			item.Unit,
			item.Price,
			item.Cost,
			item.OnHand,
			item.Usage,
			item.LastSale,
			item.LastReceipt,
			item.LastOrder,
		); err != nil {
			return fmt.Errorf("error adding item to db on item %d\t%s:\t LastOrder: %s, error:%v", item.Location, item.StockNumber, item.LastOrder, err)
		}
	}
	return err
}

type InventoryLookUpArgs struct {
	Query    string `json:"query"`
	Location string `json:"location,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}
type ReturnResult struct {
	Item            InventoryItem `json:"item"`
	ConfidenceScore int           `json:"confidence_score"`
	MatchType       string        `json:"match_type"`
}

const (
	ConfidenceExactStockNumber      = 100
	ConfidenceExactVendorPartNumber = 95
	ConfidencePartialStockNumber    = 85
	ConfidenceDescriptionAllWords   = 60
	ConfidenceDescriptionAnyWord    = 35
)

type InvenentoryLookup struct {
	db      *sql.DB
	Args    InventoryLookUpArgs
	Results []*ReturnResult
}

func (il *InvenentoryLookup) checkStockNumber() ([]*ReturnResult, error) {
	db := il.db
	if err := db.Ping(); err != nil {
		return nil, err
	}
	stmt, err := db.Prepare("SELECT location,stock_number,description,on_hand FROM inventory WHERE stock_number=? AND location=? LIMIT ?")
	if err != nil {
		return nil, err
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	rows, err := stmt.Query(il.Args.Query, il.Args.Location, il.Args.Limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	var ra []*ReturnResult

	for rows.Next() {
		fmt.Println("Reading row...")
		item := &InventoryItem{}
		err := rows.Scan(&item.Location, &item.StockNumber, &item.Description, &item.OnHand)
		if err != nil {
			fmt.Println("Error on scanning rows")
			return nil, err
		}
		rr := &ReturnResult{
			Item:            *item,
			ConfidenceScore: ConfidenceExactStockNumber,
			MatchType:       "Exact Stock Number",
		}
		ra = append(ra, rr)
		fmt.Printf("Appended %+v\n", rr)
	}
	return ra, nil
}

func SearchInventory(args InventoryLookUpArgs) ([]*ReturnResult, error) {
	fmt.Printf("Checking for: %+v\n", args)
	db, err := OpenDB()
	if err != nil {
		return nil, err
	}

	il := &InvenentoryLookup{
		db:   db,
		Args: args,
	}
	result, err := il.checkStockNumber()
	if err != nil {
		return nil, err
	}
	il.Results = append(il.Results, result...)
	fmt.Printf("Returning results on inventory: %+v\n", &il.Results)
	return il.Results, err
}
