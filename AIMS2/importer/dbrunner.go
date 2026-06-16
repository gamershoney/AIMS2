package importer

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"unicode"

	_ "modernc.org/sqlite"
)

var TableName string = "inventory"

type InventoryItem struct {
	ID               int
	Location         float64 `json:"store_id"`
	StockNumber      string  `json:"stock_number"`
	Description      string  `json:"description"`
	VendorNumber     string
	VendorProdNumber string
	Unit             string
	Price            float64
	Cost             float64
	OnHand           float64
	Usage            float64
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
			return fmt.Errorf("error adding item to db on item %f\t%s:\t LastOrder: %s, error:%v", item.Location, item.StockNumber, item.LastOrder, err)
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
	ConfidenceLowPartialStockNumber = 70
	ConfidenceDescriptionAllWords   = 60
	ConfidenceDescriptionAnyWord    = 35
)

type InvenentoryLookup struct {
	db      *sql.DB
	Args    InventoryLookUpArgs
	Results []*ReturnResult
}

func (il *InvenentoryLookup) checkExactStockNumber() ([]*ReturnResult, error) {
	db := il.db
	if err := db.Ping(); err != nil {
		return nil, err
	}
	stmt, err := db.Prepare("SELECT location,stock_number,vendor_number,vendor_product_number,description,on_hand,price,cost,avg_usage,last_sale,last_receipt,last_order FROM inventory WHERE stock_number=? AND location=? LIMIT ?")
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
		err := rows.Scan(&item.Location,
			&item.StockNumber,
			&item.VendorNumber,
			&item.VendorProdNumber,
			&item.Description,
			&item.OnHand,
			&item.Price,
			&item.Cost,
			&item.Usage,
			&item.LastSale,
			&item.LastReceipt,
			&item.LastOrder)
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

func Tokenize(s string) []string {
	up := strings.ToUpper(strings.TrimSpace(s))

	var tokens []string
	var b strings.Builder

	flush := func() {
		if b.Len() == 0 {
			return
		}

		token := b.String()

		// Skip very short noisy tokens like "KP", "M", "1"
		if len(token) >= 3 {
			tokens = append(tokens, "%"+token+"%")
		}

		b.Reset()
	}

	for _, r := range up {
		if unicode.IsDigit(r) || unicode.IsLetter(r) {
			b.WriteRune(r)
			continue
		}

		flush()
	}

	flush()

	return tokens
}

func (il *InvenentoryLookup) partialQuery(s string, stmt *sql.Stmt, location string, limit int, confidence int, reason string) {
	rows, err := stmt.Query(s, location, limit)
	if err != nil {
		return
	}

	for rows.Next() {
		fmt.Println("Reading row...")
		item := &InventoryItem{}
		err := rows.Scan(&item.Location,
			&item.StockNumber,
			&item.VendorNumber,
			&item.VendorProdNumber,
			&item.Description,
			&item.OnHand,
			&item.Price,
			&item.Cost,
			&item.Usage,
			&item.LastSale,
			&item.LastReceipt,
			&item.LastOrder)
		if err != nil {
			fmt.Println("Error on scanning rows")
			return
		}
		rr := &ReturnResult{
			Item:            *item,
			ConfidenceScore: confidence,
			MatchType:       reason,
		}
		il.Results = append(il.Results, rr)

	}
}

func (il *InvenentoryLookup) checkPartialStockNumber() {
	db := il.db
	if err := db.Ping(); err != nil {
		return
	}
	stmt, err := db.Prepare("SELECT location,stock_number,vendor_number,vendor_product_number,description,on_hand,price,cost,avg_usage,last_sale,last_receipt,last_order FROM inventory WHERE (stock_number LIKE ? OR vendor_product_number LIKE ?) AND location=? LIMIT ?")
	if err != nil {
		return
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	qry := il.Args.Query
	tokens := Tokenize(qry)

	for _, v := range tokens {
		switch {
		case len(v) > 2:
			il.partialQuery(v, stmt, il.Args.Location, il.Args.Limit, ConfidencePartialStockNumber, "Matched a text token greater than 2 characters.")
			break
		case len(v) <= 2:
			il.partialQuery(v, stmt, il.Args.Location, il.Args.Limit, ConfidenceLowPartialStockNumber, "Matched a text token of less than 3 characters.")
		}
	}
}

func (il *InvenentoryLookup) searchDescription() error {
	db := il.db

	stmt, err := db.Prepare(`
SELECT
	location,
	stock_number,
	vendor_number,
	vendor_product_number,
	description,
	on_hand,
	price,
	cost,
	avg_usage,
	last_sale,
	last_receipt,
	last_order
FROM inventory
WHERE description LIKE ?
AND location = ?
LIMIT ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	phrase := "%" + strings.ToUpper(strings.TrimSpace(il.Args.Query)) + "%"

	il.partialQuery(
		phrase,
		stmt,
		il.Args.Location,
		10,
		ConfidenceDescriptionAllWords,
		"Description matched full query phrase.",
	)

	tokens := Tokenize(il.Args.Query)

	for _, token := range tokens {
		il.partialQuery(
			token,
			stmt,
			il.Args.Location,
			5,
			ConfidenceDescriptionAnyWord,
			"Description matched a query token.",
		)
	}

	return nil
}

func SearchInventory(args InventoryLookUpArgs) ([]*ReturnResult, error) {
	fmt.Printf("Checking for: %+v\n", args)

	if args.Limit <= 0 {
		args.Limit = 10
	}

	if args.Location == "" {
		args.Location = "1"
	}

	db, err := OpenDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	il := &InvenentoryLookup{
		db:   db,
		Args: args,
	}
	result, err := il.checkExactStockNumber()
	il.Results = append(il.Results, result...)
	il.checkPartialStockNumber()
	il.searchDescription()
	if err != nil {
		return nil, err
	}

	fmt.Printf("Returning results on inventory: %+v\n", &il.Results)
	return il.Results, err
}
