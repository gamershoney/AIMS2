package main

import "encoding/json"

type Parameter struct {
	Type string
	Properties
}

type Tool struct {
	Type        string
	Name        string
	Description string
	Parameters  json.RawMessage
}
