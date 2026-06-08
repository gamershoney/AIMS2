// Package importer is used for importing spreadsheets from inventory data
package importer

import (
	"fmt"

	excelize "github.com/xuri/excelize/v2"
)

func TestOpen() error {
	f, err := excelize.OpenFile("Inv Master UPC USQL 20251119.xlsx")
	if err != nil {
		return fmt.Errorf("error: package importer:", err)
	}
	defer func() {
		if err != nil {
			fmt.Println("Error on closing spreadsheet for read: ", err)
		}
	}()
	return nil
}
