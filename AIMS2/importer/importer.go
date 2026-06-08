// Package importer is used for importing spreadsheets from inventory data
package importer

import (
	"fmt"

	excelize "github.com/xuri/excelize/v2"
)

func TestOpen() error {
	f, err := excelize.OpenFile("Inv Master UPC USQL 20251119.xlsx")
	if err != nil {
		return fmt.Errorf("error: package importer: %v", err)
	}
	defer func() {
		if err != nil {
			fmt.Println("Error on closing spreadsheet for read: ", err)
		}
	}()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return fmt.Errorf("error: Sheet index 0 not found")
	}

	rows, err := f.Rows(sheet)
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}
	index := 0
	for rows.Next() && index < 49 {
		row, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("error: %v", err)
		}
		for i, cell := range row {
			if i > 5 {
				break
			}
			fmt.Printf("%s \t", cell)
		}
		fmt.Println()
		index += 1
	}

	return nil
}
