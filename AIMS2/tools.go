package main

import (
	"context"
	"encoding/json"
)

type Property struct {
	Type        string
	Description string
}

type Paramters struct {
	Type       string
	Properties map[string]Property
	Required   []string
}

type Tool interface {
	Name() string
	Description() string
	Paramaters() Paramters
	Execute(ctx context.Context, args json.RawMessage) (any, error)
}

type WebSearchTool struct {
	URL string
}

func (t *WebSearchTool) Name() string {
	return "Searh_Online"
}

func (t *WebSearchTool) Description() string {
	return "use to return a set of search results from the google api"
}

func (t *WebSearchTool) Paramaters() Paramters {
	return Paramters{
		Type: "object",
		Properties: map[string]Property{
			"Query": {Type: "string", Description: "Enter the query you would like to search"},
		},
		Required: []string{"Query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var input struct {
		Query string
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	rs := func() string {
		return "This is an example" + input.Query
	}
	return rs(), nil
}
