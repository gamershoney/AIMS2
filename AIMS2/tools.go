package main

import (
	"AIMS2/importer"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const apiEndpoint = "https://ollama.com/api/web_search"

func InitializeTools() *Registry {
	r := &Registry{}
	r.tools = make(map[string]Tool)
	r.Register(&WebSearchTool{})
	r.Register(&StockCheck{})
	r.Register(&NotesTool{})
	return r
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Registry struct {
	tools map[string]Tool
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

type ModelFunction struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type ModelTool struct {
	Type     string        `json:"type"`
	Function ModelFunction `json:"function"`
}

type ToolResult struct {
	Name     string `json:"name"`
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
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
		tr.Error = "could not find args in websearch query"
		return tr
	}
	rs := func() (string, error) {
		searchResults, err := RunWebSearch(qs)
		if err != nil {
			fmt.Println("Error on search: ", err)
		}
		return searchResults, err
	}
	result, err := rs()
	if err != nil {
		tr.Error = err.Error()
	} else {
		tr.Response = result
	}
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
		return "", err
	}
	apiKey := os.Getenv("WEB_SEARCH_API_KEY")
	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Authorization", "Bearer "+apiKey)

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
		return "", err
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
	tr := ToolResult{Name: tc.Function.Name}

	qs, ok := qry.(string)
	if !ok || strings.TrimSpace(qs) == "" {
		tr.Error = "missing required argument: query"
		return tr
	}

	var instock bool
	if raw, ok := tc.Function.Arguments["in_stock"]; ok {
		var valid bool
		instock, valid = raw.(bool)
		if !valid {
			fmt.Printf("stock_check: in_stock argument has unexpected type %T, defaulting to false\n", raw)
		}
	}

	results, err := importer.SearchInventory(importer.InventoryLookUpArgs{
		Query:    qs,
		InStock:  instock,
		Location: "1",
		Limit:    10,
	})
	if err != nil {
		tr.Error = err.Error()
		return tr
	}

	if len(results) == 0 {
		tr.Response = `{"results":[],"message":"no results found"}`
		return tr
	}

	response, err := t.ParseInventoryData(results)
	if err != nil {
		tr.Error = err.Error()
		return tr
	}

	tr.Response = response
	return tr
}

type NotesTool struct{}

func (t *NotesTool) Name() string {
	return "make_note"
}

func (t *NotesTool) Description() string {
	return "use this tool to log any notes on an inventory item. these notes are returned to you when the stock_check tool is called. use this when the user asks you to remember anything on an item. please have an exact stock_number when adding a note or the tool will fail. If you do not already have an exact stock_number use the stock_check to find it."
}

func (t *NotesTool) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"stock_number":    {Type: "string", Description: "enter the stock number you wish to log a note on"},
			"note":            {Type: "string", Description: "use this to add a note to any item. this can be anything about the item either you or the user may need to remember. if the user provides insight on the item it can be logged here."},
			"is_critical":     {Type: "boolean", Description: "use this to mark an item as critical. this can be useful when getting asked purchasing questions. mark true for critical; false for not cricical. Any item will default to false."},
			"minimum_on_hand": {Type: "number", Description: "use this to set a minimum quantity on hand. if the user specifies a minimum quantity of an item to remain on the shelf this is a good place to log it."},
			"target_on_hand":  {Type: "number", Description: "use this to set the target quantity to stock on the shelf when ordering."},
		},
		Required: []string{"stock_number", "note"},
	}
}

func (t *NotesTool) Execute(ctx context.Context, tc ToolCall) ToolResult {
	tr := ToolResult{
		Name:     "make_note",
		Response: "",
	}
	sku, ok := tc.Function.Arguments["stock_number"]
	if !ok {
		tr.Error = "error: missing stock_number"
		return tr
	}
	skustring, ok := sku.(string)
	if !ok {
		tr.Error = "error: cannot convert type on stock_number to string"
		return tr
	}

	db, err := importer.OpenDB()
	if err != nil {
		fmt.Println(err)
		tr.Error = "error: could not connect to inventory database"
		return tr
	}
	note := importer.NewNote(skustring, 1, db)
	text, ok := tc.Function.Arguments["note"].(string)
	if !ok {
		tr.Error = "error: missing note"
		return tr
	}
	note.Note = text

	critical, ok := tc.Function.Arguments["is_critical"].(bool)
	if ok {
		note.IsCritical = critical
	}
	min, ok := tc.Function.Arguments["minimum_on_hand"].(float64)
	if ok {
		note.MinimumOnHand = float32(min)
	}
	target, ok := tc.Function.Arguments["target_on_hand"].(float64)
	if ok {
		note.TargetOnHand = float32(target)
	}
	if err := note.WriteNote(); err != nil {
		fmt.Println(err)
		fmt.Println()
		tr.Error = "error logging note to system"
	} else {
		tr.Response = "Note saved"
	}

	return tr
}
