package main

import (
	"context"
	"fmt"
	"testing"
)

func TestRunToolCalls(t *testing.T) {
	tc := ToolCall{
		ID: "testID", Function: struct {
			Index     int64          "json:\"index\""
			Name      string         "json:\"name\""
			Arguments map[string]any "json:\"arguments\""
		}{
			Index:     1,
			Name:      "stock_check",
			Arguments: map[string]any{"part_number": "3M-16114"},
		},
	}

	tool := &StockCheck{}

	testMsg := tool.Execute(context.TODO(), tc)
	fmt.Printf("Sending test message:%+v\n", tc)

	fmt.Printf("returned answer: %+v", testMsg)
}
