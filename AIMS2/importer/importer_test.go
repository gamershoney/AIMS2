package importer

import "testing"

func TestTestOpen(t *testing.T) {
	err := TestOpen()
	if err != nil {
		t.Errorf(`error on TestOpen(): %v`, err)
	}
}
