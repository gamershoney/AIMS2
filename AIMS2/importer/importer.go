// Package importer is used for importing spreadsheets from inventory data
package importer

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	excelize "github.com/xuri/excelize/v2"
)

func TestOpen() error {
	db, err := OpenDB()
	if err != nil {
		return fmt.Errorf("open inventory database: %w", err)
	}
	defer db.Close()

	items, err := loadSheet("")
	if err != nil {
		return fmt.Errorf("load inventory spreadsheet: %w", err)
	}
	return AddInventory(db, items)
}

func NewItem() *InventoryItem {
	item := &InventoryItem{
		Location:         0,
		StockNumber:      "null",
		Description:      "null",
		VendorNumber:     "null",
		VendorProdNumber: "null",
		Unit:             "null",
		Price:            0,
		Cost:             0,
		OnHand:           0,
		Usage:            0,
		LastSale:         "null",
		LastReceipt:      "null",
		LastOrder:        "null",
	}
	return item
}

func getCell(row []string, indexes map[string]int, item string) string {
	index, ok := indexes[item]
	if !ok {
		return "0"
	}
	value := row[index]
	if value == "" {
		return "0"
	}
	return value
}

func loadSheet(location string) ([]*InventoryItem, error) {
	if strings.TrimSpace(location) == "" {
		location = strings.TrimSpace(os.Getenv("AIMS2_INVENTORY_IMPORT_PATH"))
	}
	if location == "" {
		return nil, fmt.Errorf("inventory import path is required")
	}
	f, err := excelize.OpenFile(location)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, fmt.Errorf("error: no sheet at index")
	}
	indexes := make(map[string]int)
	rows, err := f.Rows(sheet)
	if rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		for i, v := range row {
			indexes[v] = i
		}
	}
	var items []*InventoryItem
	for rows.Next() {
		item := NewItem()
		row, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		item.Location, _ = strconv.ParseFloat(getCell(row, indexes, "Location"), 64)
		item.StockNumber = getCell(row, indexes, "Stock_Number")
		item.Description = getCell(row, indexes, "Description")
		item.VendorNumber = getCell(row, indexes, "Vendor_Number")
		item.VendorProdNumber = getCell(row, indexes, "Vendor_Prod_Num")
		item.Unit = getCell(row, indexes, "Unit")
		item.Price, _ = strconv.ParseFloat(getCell(row, indexes, "Price"), 64)
		item.Cost, _ = strconv.ParseFloat(getCell(row, indexes, "Unit_Replace_Cost"), 64)
		item.OnHand, _ = strconv.ParseFloat(getCell(row, indexes, "On_Hand"), 64)
		item.Usage, _ = strconv.ParseFloat(getCell(row, indexes, "Avg_Usage"), 64)
		item.LastSale = getCell(row, indexes, "Date_Last_Issue")
		item.LastReceipt = getCell(row, indexes, "Date_Last_Receipt")
		item.LastOrder = getCell(row, indexes, "Date_Last_Order")
		items = append(items, item)
	}

	return items, rows.Error()
}
