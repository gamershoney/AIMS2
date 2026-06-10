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

	items, err := LoadSheet("")
	if err != nil {
		fmt.Println(err)
		return err
	}
	for _, v := range items {
		fmt.Printf("Location: %d\t StockNumber: %s\t LastOrder:%s\n", v.Location, v.StockNumber, v.LastOrder)
	}
	err = AddInventory(db, items)
	return err
}

func LoadSheet(location string) ([]*InventoryItem, error) {
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
		item := &InventoryItem{}
		row, err := rows.Columns()
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		item.Location, _ = strconv.Atoi(row[indexes["Location"]])
		item.StockNumber = row[indexes["Stock_Number"]]
		item.Description = row[indexes["Description"]]
		item.VendorNumber = row[indexes["Vendor_Number"]]
		item.VendorProdNumber = row[indexes["Vendor_Prod_Number"]]
		item.Unit = row[indexes["Unit"]]
		item.Price, _ = strconv.Atoi(row[indexes["Price"]])
		item.Cost, _ = strconv.Atoi(row[indexes["Unit_Replace_Cost"]])
		item.OnHand, _ = strconv.Atoi(row[indexes["On_Hand"]])
		item.Usage, _ = strconv.Atoi(row[indexes["Avg_Usage"]])
		item.LastSale = row[indexes["Date_Last_Issue"]]
		item.LastReceipt = row[indexes["Date_Last_Receipt"]]
		item.LastOrder = row[indexes["Date_Last_Order"]]
		items = append(items, item)
	}

	return items, err
}
