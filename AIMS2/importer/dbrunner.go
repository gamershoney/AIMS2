package importer

import (
	"cmp"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"slices"
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
	InStock  bool   `json:"in_stock"`
	Location string `json:"location,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}
type ReturnResult struct {
	Item            InventoryItem `json:"item"`
	HasNote         bool          `json:"has_note"`
	Note            *Note         `json:"note,omitempty"`
	ConfidenceScore int           `json:"confidence_score"`
	MatchType       string        `json:"match_type"`
}

const (
	ConfidenceExactStockNumber      = 100
	ConfidenceExactVendorPartNumber = 95
	ConfidencePartialStockNumber    = 85
	ConfidenceLowPartialStockNumber = 45
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
	stmt, err := db.Prepare("SELECT location,stock_number,vendor_number,vendor_product_number,description,on_hand,price,cost,avg_usage,last_sale,last_receipt,last_order FROM inventory WHERE stock_number=? AND location=? AND on_hand>? LIMIT ?")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer func() {
		err := stmt.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	var stock int
	if il.Args.InStock {
		stock = 0
	} else {
		stock = -1
	}

	rows, err := stmt.Query(il.Args.Query, il.Args.Location, stock, il.Args.Limit)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ra, nil
}

func normalizeSearch(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))

	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}

	return b.String()
}

func scoreResult(r *ReturnResult, query string) int {
	if r == nil {
		return 0
	}

	item := r.Item

	qNorm := normalizeSearch(query)
	stockNorm := normalizeSearch(item.StockNumber)
	vendorProdNorm := normalizeSearch(item.VendorProdNumber)
	descUpper := strings.ToUpper(item.Description)
	descNorm := normalizeSearch(item.Description)
	vendorUpper := strings.ToUpper(strings.TrimSpace(item.VendorNumber))
	queryUpper := strings.ToUpper(query)

	score := r.ConfidenceScore

	if stockNorm == qNorm {
		score += 200
	}

	if vendorProdNorm == qNorm {
		score += 180
	}

	if qNorm != "" && strings.Contains(stockNorm, qNorm) {
		score += 100
	}

	if qNorm != "" && strings.Contains(vendorProdNorm, qNorm) {
		score += 90
	}

	cleanTokens := CleanTokens(query)

	for _, token := range cleanTokens {
		tNorm := normalizeSearch(token)

		if tNorm == "" {
			continue
		}

		tokenStrong := hasLettersAndDigits(tNorm) && len(tNorm) >= 4

		if strings.Contains(stockNorm, tNorm) {
			if tokenStrong {
				score += 80
			} else {
				score += 20
			}
		}

		if strings.Contains(vendorProdNorm, tNorm) {
			if tokenStrong {
				score += 70
			} else {
				score += 15
			}
		}

		if strings.Contains(descNorm, tNorm) {
			if tokenStrong {
				score += 60
			} else {
				score += 10
			}
		}
	}

	if strings.Contains(queryUpper, "TIG") {
		if strings.Contains(descUpper, "TIG") {
			score += 25
		}

		if strings.Contains(descUpper, "WIRE") || strings.Contains(descUpper, "ROD") {
			score += 25
		}
	}

	if strings.Contains(queryUpper, "ROD") {
		if strings.Contains(descUpper, "ROD") || strings.Contains(descUpper, "WIRE") {
			score += 20
		}
	}

	if strings.Contains(normalizeSearch(query), "70S6") {
		if strings.Contains(stockNorm, "70S6") ||
			strings.Contains(vendorProdNorm, "70S6") ||
			strings.Contains(descNorm, "70S6") {
			score += 100
		} else {
			score -= 100
		}
	}

	if strings.Contains(queryUpper, "OXF") || strings.Contains(queryUpper, "OXFORD") {
		if vendorUpper == "OXFO" || strings.Contains(stockNorm, "OXF") {
			score += 60
		} else {
			score -= 30
		}
	}

	if strings.Contains(queryUpper, "ROD") || strings.Contains(queryUpper, "WIRE") || strings.Contains(normalizeSearch(query), "70S6") {
		badCategories := []string{
			"TORCH",
			"WELDER",
			"RENTAL",
			"PARTS KIT",
			"TIG PEN",
			"ASSY",
			"COVER",
		}

		for _, bad := range badCategories {
			if strings.Contains(descUpper, bad) {
				score -= 100
			}
		}
	}

	if item.OnHand > 0 {
		score += 10
	}

	return score
}

func CleanTokens(s string) []string {
	up := strings.ToUpper(strings.TrimSpace(s))

	var tokens []string
	var b strings.Builder

	flush := func() {
		if b.Len() == 0 {
			return
		}

		token := b.String()

		if len(token) >= 3 {
			tokens = append(tokens, token)
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

func LikeTokens(s string) []string {
	clean := CleanTokens(s)

	likes := make([]string, 0, len(clean))
	for _, token := range clean {
		likes = append(likes, "%"+token+"%")
	}

	return likes
}

func hasLettersAndDigits(s string) bool {
	hasLetter := false
	hasDigit := false

	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
		}

		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}

	return hasLetter && hasDigit
}

func StrongLikeTokens(query string) []string {
	clean := CleanTokens(query)

	var strong []string
	for _, token := range clean {
		if hasLettersAndDigits(token) && len(token) >= 4 {
			strong = append(strong, "%"+token+"%")
		}
	}

	return strong
}

func finalizeResults(results []*ReturnResult, query string, limit int) []*ReturnResult {
	if limit <= 0 {
		limit = 8
	}

	seen := make(map[string]*ReturnResult)

	for _, r := range results {
		if r == nil {
			continue
		}

		key := fmt.Sprintf("%.0f:%s", r.Item.Location, strings.ToUpper(strings.TrimSpace(r.Item.StockNumber)))

		score := scoreResult(r, query)
		r.ConfidenceScore = score

		existing, ok := seen[key]
		if !ok || score > existing.ConfidenceScore {
			seen[key] = r
		}
	}

	deduped := make([]*ReturnResult, 0, len(seen))
	for _, r := range seen {
		deduped = append(deduped, r)
	}

	slices.SortStableFunc(deduped, func(a, b *ReturnResult) int {
		return cmp.Compare(b.ConfidenceScore, a.ConfidenceScore)
	})

	filtered := deduped[:0]
	for _, r := range deduped {
		if r.ConfidenceScore < 50 {
			continue
		}

		filtered = append(filtered, r)
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered
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

func (il *InvenentoryLookup) descriptionQuery(s string, instock int, stmt *sql.Stmt, location string, limit int, confidence int, reason string) error {
	rows, err := stmt.Query(s, location, instock, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	fmt.Println("Reading row descriptions...")

	for rows.Next() {
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
			return err
		}
		rr := &ReturnResult{
			Item:            *item,
			ConfidenceScore: confidence,
			MatchType:       reason,
		}
		il.Results = append(il.Results, rr)

	}
	return rows.Err()
}

func (il *InvenentoryLookup) partialQuery(s string, instock int, stmt *sql.Stmt, location string, limit int, confidence int, reason string) error {
	rows, err := stmt.Query(s, s, location, instock, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	fmt.Println("Reading row...")

	for rows.Next() {
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
			return err
		}
		rr := &ReturnResult{
			Item:            *item,
			ConfidenceScore: confidence,
			MatchType:       reason,
		}
		il.Results = append(il.Results, rr)

	}
	return rows.Err()
}

func (il *InvenentoryLookup) checkPartialStockNumber() error {
	db := il.db
	if err := db.Ping(); err != nil {
		return err
	}
	stmt, err := db.Prepare("SELECT location,stock_number,vendor_number,vendor_product_number,description,on_hand,price,cost,avg_usage,last_sale,last_receipt,last_order FROM inventory WHERE (stock_number LIKE ? OR vendor_product_number LIKE ?) AND location=? AND on_hand>? LIMIT ?")
	if err != nil {
		return err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	var stock int
	if il.Args.InStock {
		stock = 0
	} else {
		stock = -1
	}

	tokens := StrongLikeTokens(il.Args.Query)
	confidence := ConfidencePartialStockNumber
	reason := "Matched a strong part-number token."

	if len(tokens) == 0 {
		tokens = LikeTokens(il.Args.Query)
		confidence = ConfidenceLowPartialStockNumber
		reason = "Matched a weak part-number token."
	}

	for _, token := range tokens {
		err := il.partialQuery(
			token,
			stock,
			stmt,
			il.Args.Location,
			il.Args.Limit,
			confidence,
			reason,
		)
		if err != nil {
			return err
		}
	}
	return err
}

func (il *InvenentoryLookup) searchDescription() error {
	db := il.db
	var err error
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
AND on_hand>?
LIMIT ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	var stock int
	if il.Args.InStock {
		stock = 0
	} else {
		stock = -1
	}
	phrase := "%" + strings.ToUpper(strings.TrimSpace(il.Args.Query)) + "%"

	err = il.descriptionQuery(
		phrase,
		stock,
		stmt,
		il.Args.Location,
		10,
		ConfidenceDescriptionAllWords,
		"Description matched full query phrase.",
	)
	if err != nil {
		return err
	}

	strongTokens := StrongLikeTokens(il.Args.Query)

	if len(strongTokens) > 0 {
		for _, token := range strongTokens {
			err := il.descriptionQuery(
				token,
				stock,
				stmt,
				il.Args.Location,
				5,
				ConfidenceDescriptionAnyWord,
				"Description matched a strong query token.",
			)
			if err != nil {
				return err
			}
		}
	} else {
		tokens := LikeTokens(il.Args.Query)

		for _, token := range tokens {
			err := il.descriptionQuery(
				token,
				stock,
				stmt,
				il.Args.Location,
				5,
				ConfidenceDescriptionAnyWord,
				"Description matched a query token.",
			)
			if err != nil {
				return err
			}
		}
	}
	return err
}

func AttachNotes(sku string, location int, db *sql.DB) (*Note, error) {
	notes, err := GetNote(sku, location, db)
	if err != nil {
		return nil, err
	}
	if len(notes) > 0 {
		return notes[0], err
	}
	return nil, err
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
	if err != nil {
		fmt.Println("Error on exact stock number: ", err)
	}
	fmt.Println("Exact results found: ")
	for _, v := range result {
		fmt.Printf("Location: %v\t StockNumber: %v\n", v.Item.Location, v.Item.StockNumber)
		fmt.Println()
	}
	il.Results = append(il.Results, result...)
	if len(result) == 0 {
		err = il.checkPartialStockNumber()
		if err != nil {
			fmt.Println("error on partial stock number", err)
			return nil, err
		}
		err = il.searchDescription()
		if err != nil {
			fmt.Println("error on description search", err)
			return nil, err
		}
	}
	il.Results = finalizeResults(il.Results, args.Query, args.Limit)

	for _, v := range il.Results {
		note, err := AttachNotes(v.Item.StockNumber, int(v.Item.Location), il.db)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if note != nil {
			v.Note = note
			v.HasNote = true
		} else {
			v.HasNote = false
		}
	}

	fmt.Printf("Returning %d results on inventory\n", len(il.Results))
	return il.Results, nil
}

func SingleSearch(args InventoryLookUpArgs) (*ReturnResult, error) {
	fmt.Printf("Running detailed item query for: %+v\n", args)

	args.Limit = 1

	args.Location = "1"

	db, err := OpenDB()
	if err != nil {
		return nil, err
	}
	il := &InvenentoryLookup{
		db:   db,
		Args: args,
	}

	items, err := il.checkExactStockNumber()
	if err != nil {
		return nil, err
	}
	if len(items) <= 0 {
		return nil, errors.New("error: no items found")
	}
	item := items[0]
	note, err := GetNote(item.Item.StockNumber, int(item.Item.Location), db)
	if err != nil {
		return nil, err
	}
	item.HasNote = false
	if len(note) > 0 {
		item.HasNote = true
		item.Note = note[0]
	}
	return item, err
}
