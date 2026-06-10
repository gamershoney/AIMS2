package importer

import (
	"fmt"
	"testing"
)

func TestTestOpen(t *testing.T) {
	err := TestOpen()
	if err != nil {
		fmt.Println(fmt.Errorf(`error on TestOpen(): %v`, err))
	}
}
