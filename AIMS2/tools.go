package main

import (
	"AIMS2/importer"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const apiEndpoint = "https://ollama.com/api/web_search"

func InitializeTools() *Registry {
	r := &Registry{}
	r.tools = make(map[string]Tool)
	r.Register(&WebSearchTool{})
	r.Register(&StockCheck{})
	return r
}

type Property struct {
	Type        string
	Description string
}

type Parameters struct {
	Type       string
	Properties map[string]Property
	Required   []string
}

type Registry struct {
	tools map[string]Tool
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

type ModelFunction struct {
	Name        string
	Description string
	Parameters  Parameters
}

type ModelTool struct {
	Type     string
	Function ModelFunction
}

type ToolResult struct {
	Name     string
	Response string
	Error    error
}

func (r *Registry) ModelTools() []ModelTool {
	var mt []ModelTool
	for _, tool := range r.tools {
		mt = append(mt, ModelTool{
			Type: "function",
			Function: ModelFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}

	return mt
}

type Tool interface {
	Name() string
	Description() string
	Parameters() Parameters
	Execute(ctx context.Context, tc ToolCall) ToolResult
}

type WebSearchTool struct {
	URL string
}

func (t *WebSearchTool) Name() string {
	return "search_Online"
}

func (t *WebSearchTool) Description() string {
	return "use to return a set of search results from the google api"
}

func (t *WebSearchTool) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"Query": {Type: "string", Description: "Enter the query you would like to search"},
		},
		Required: []string{"Query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, tc ToolCall) ToolResult {
	qry := tc.Function.Arguments["Query"]
	tr := ToolResult{Name: tc.Function.Name}
	qs, ok := qry.(string)

	if !ok {
		tr.Error = errors.New("Could not find args in websearch query")
		return tr
	}
	rs := func() (string, error) {
		searchResults, err := RunWebSearch(qs)
		if err != nil {
			fmt.Println("Error on search: ", err)
		}
		return searchResults, err
	}
	tr.Response, tr.Error = rs()
	return tr
}

func RunWebSearch(qry string) (string, error) {
	//ctx := context.TODO

	body := &struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}{Query: qry, MaxResults: 10}

	qryReturn := struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}{}

	jbdy, err := json.Marshal(body)
	if err != nil {
		fmt.Print("Error marshalling web search query: ", err)
		return "", err
	}
	jrdr := bytes.NewReader(jbdy)
	rq, err := http.NewRequest("POST", apiEndpoint, jrdr)
	if err != nil {
		fmt.Println("error: creating new request: ", err)
	}
	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Authorization", "Bearer "+"3fb93a7acbe44f74b53612ec567268a6.rIM2NDZ_3eFZmDMkF-4QpjvJ")

	client := &http.Client{}
	resp, err := client.Do(rq)
	if err != nil {
		fmt.Println("Error on post request", err)
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error: Cannot read response body in web search")
	}

	err = json.Unmarshal(b, &qryReturn)
	if err != nil {
		fmt.Println("Error unmarshalling web search results: ", err)
		return "", err
	}
	var strQry strings.Builder
	for i, v := range qryReturn.Results {
		if i > 3 {
			break
		}
		if len(v.Content) > 1200 {
			v.Content = v.Content[:1200]
		}
		fmt.Fprintf(&strQry, "Result %d:\nTitle: %s\nURL: %s\nContent: %s\n\n",
			i+1,
			v.Title,
			v.URL,
			v.Content)
	}

	//	strQry := fmt.Sprintf("%v", qryReturn)
	return strQry.String(), nil
}

type StockCheck struct {
	db *sql.DB
}

func (t *StockCheck) Name() string {
	return "stock_check"
}

func (t *StockCheck) Description() string {
	return "use when to check if our company stocks an item by part number. Will return an item with the location it was found in, the part number found, the description of the item, how many are on hand,  a confidence score with a minimum of 0 and a maximum of 100, and how the item was matched along with other useful information."
}

func (t *StockCheck) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"query":    {Type: "string", Description: "Enter the exact part number or keyphrase you would like to search our inventory for."},
			"in_stock": {Type: "boolean", Description: "enter true if the item must be in stock, enter false if the item does not need to be in stock."},
		},
		Required: []string{"query"},
	}
}

func (t *StockCheck) ParseInventoryData(results []*importer.ReturnResult) (string, error) {
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal inventory results: %w", err)
	}

	return string(b), nil
}

func (t *StockCheck) Execute(ctx context.Context, tc ToolCall) ToolResult {
	qry := tc.Function.Arguments["query"]
	instock, ok := tc.Function.Arguments["in_stock"]
	if !ok {
		instock = false
	}
	tr := ToolResult{Name: tc.Function.Name}

	qs, ok := qry.(string)
	if !ok || strings.TrimSpace(qs) == "" {
		tr.Error = errors.New("missing required argument: part_number")
		return tr
	}
	instock, valid := instock.(bool)
	if !valid {
		tr.Error = errors.New("invalid usage on instock boolean")
	}
	results, err := importer.SearchInventory(importer.InventoryLookUpArgs{
		Query:    qs,
		InStock:  instock.(bool),
		Location: "1",
		Limit:    10,
	})
	if err != nil {
		tr.Error = err
		return tr
	}

	if len(results) == 0 {
		tr.Response = `{"results":[],"message":"no results found"}`
		return tr
	}

	response, err := t.ParseInventoryData(results)
	if err != nil {
		tr.Error = err
		return tr
	}

	tr.Response = response
	return tr
}
