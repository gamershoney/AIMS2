// Package importer is used for importing spreadsheets from inventory data
package importer

import (
	"fmt"
	"strconv"

	excelize "github.com/xuri/excelize/v2"
)

func TestOpen() error {
	db, err := OpenDB()
	if err != nil {
		fmt.Println("error: opening database, ", err)
		fmt.Println()
		return err
	}

	items, err := loadSheet("")
	if err != nil {
		fmt.Println(err)
		return err
	}
	for _, v := range items {
		fmt.Printf("Location: %f\t StockNumber: %s\t LastOrder:%s\t Price: %f\n", v.Location, v.StockNumber, v.LastOrder, v.Price)
	}
	err = AddInventory(db, items)
	return err
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
		fmt.Printf("cell search: %s, returned not found\n", item)
		return "0"
	}
	value := row[index]
	if value == "" {
		fmt.Printf("cell search : %s found but returned an empty value\n", item)
		return "0"
	}
	return value
}

func loadSheet(location string) ([]*InventoryItem, error) {
	//l := location
	f, err := excelize.OpenFile("Inv Master UPC USQL 20251119.xlsx")
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	if err != nil {
		return nil, err
	}
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
			fmt.Println(err)
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

	return items, err
}
