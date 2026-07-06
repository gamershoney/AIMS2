package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"AIMS2/importer"
)

const (
	apiEndpoint     = "https://ollama.com/api/web_search"
	maxToolResponse = 12000
)

func InitializeTools() *Registry {
	r := &Registry{}
	r.tools = make(map[string]Tool)
	r.Register(&WebSearchTool{})
	r.Register(&StockCheck{})
	r.Register(&NotesTool{})
	r.Register(&PriorityVendor{})
	r.Register(&ItemDetails{})
	return r
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
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
	Type     string        `json:"type,omitempty"`
	Function ModelFunction `json:"function,omitzero"`
}

type ToolResult struct {
	Name       string         `json:"name"`
	Response   string         `json:"response"`
	Error      error          `json:"error"`
	State      *WorkflowState `json:"current_state,omitempty"`
	SkillState *SkillState    `json:"skill_state,omitempty"`
}

func (tr ToolResult) truncate(max int) string {
	s := tr.Response
	if len(s) <= max {
		return s
	}

	cut := max
	for cut > 0 && !utf8.ValidString(s[:cut]) {
		cut--
	}

	return s[:cut] + "\n\n[tool response trucated]"
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
	return "USE WHEN: the user needs public manufacturer information, specifications, compatibility, manuals, or discontinued-item research. DO NOT USE FOR: company stock, price, cost, usage, notes, or purchasing history. For company inventory, use stock_check first and item_details second."
}

func (t *WebSearchTool) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"Query": {Type: "string", Description: "Public-web search terms. Include the manufacturer and exact part number when known."},
		},
		Required: []string{"Query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, tc ToolCall) ToolResult {
	qry := tc.Function.Arguments["Query"]
	tr := ToolResult{Name: tc.Function.Name}
	qs, ok := qry.(string)

	if !ok {
		tr.Error = errors.New("could not find args in websearch query")
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
	return "USE WHEN: finding whether the company carries an item or discovering its exact internal stock_number from a part number, vendor part number, description, or keyword. RETURNS ONLY: compact candidates with stock_number, store_id, description, on_hand, note flags, confidence_score, and match_type. DOES NOT RETURN: price, cost, vendor, usage, received dates, sales dates, or full notes. REQUIRED NEXT STEP: if the user asks for price, cost, retail, vendor, usage, dates, notes, or full details, call item_details with the best matching internal stock_number before answering. DO NOT USE: when an exact internal stock_number is already known and full details are requested. READ-ONLY: this tool never changes inventory data."
}

func (t *StockCheck) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"query":    {Type: "string", Description: "Part number, vendor part number, description, or keywords. Preserve exact part-number punctuation."},
			"in_stock": {Type: "boolean", Description: "True excludes zero-on-hand items. False includes out-of-stock items and is preferred for item identification."},
		},
		Required: []string{"query"},
	}
}

type StockCheckMatch struct {
	StockNumber     string  `json:"stock_number"`
	StoreID         float64 `json:"store_id"`
	Description     string  `json:"description"`
	OnHand          float64 `json:"on_hand"`
	ConfidenceScore int     `json:"confidence_score"`
	MatchType       string  `json:"match_type"`
	HasNote         bool    `json:"has_note"`
	Note            string  `json:"note,omitempty"`
	IsCritical      bool    `json:"is_critical,omitempty"`
}

type StockCheckResponse struct {
	Count   int               `json:"count"`
	Results []StockCheckMatch `json:"results"`
	Message string            `json:"message,omitempty"`
}

func (t *StockCheck) ParseInventoryData(results []*importer.ReturnResult) (string, error) {
	response := StockCheckResponse{
		Count:   len(results),
		Results: make([]StockCheckMatch, 0, len(results)),
	}

	for _, result := range results {
		if result == nil {
			continue
		}

		match := StockCheckMatch{
			StockNumber:     result.Item.StockNumber,
			StoreID:         result.Item.Location,
			Description:     result.Item.Description,
			OnHand:          result.Item.OnHand,
			ConfidenceScore: result.ConfidenceScore,
			MatchType:       result.MatchType,
			HasNote:         result.HasNote,
		}
		if result.Note != nil {
			match.Note = result.Note.Note
			match.IsCritical = result.Note.IsCritical
		}

		response.Results = append(response.Results, match)
	}
	response.Count = len(response.Results)
	if response.Count == 0 {
		response.Message = "no results found"
	}

	b, err := json.Marshal(response)
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
		tr.Error = errors.New("missing required argument: query")
		return tr
	}
	inStockOnly, valid := instock.(bool)
	if !valid {
		tr.Error = errors.New("invalid usage on instock boolean")
		return tr
	}
	results, err := importer.SearchInventory(importer.InventoryLookUpArgs{
		Query:    qs,
		InStock:  inStockOnly,
		Location: "1",
		Limit:    10,
	})
	if err != nil {
		tr.Error = err
		return tr
	}

	response, err := t.ParseInventoryData(results)
	if err != nil {
		tr.Error = err
		return tr
	}

	tr.Response = response
	tr.State = &WorkflowState{
		LastToolUsed:   tc.Function.Name,
		LastItemSearch: qs,
		LastItemFound:  "",
		clear:          false,
	}
	return tr
}

type NotesTool struct{}

func (t *NotesTool) Name() string {
	return "make_note"
}

func (t *NotesTool) Description() string {
	return "STATE-CHANGING TOOL. USE WHEN: the user explicitly asks to save or update an item note, critical flag, minimum_on_hand, or target_on_hand. REQUIRES: an exact internal stock_number. Use stock_check first if unknown; use item_details first when existing values must be reviewed. DO NOT USE: to read notes, inspect an item, look up price/cost, or save an unconfirmed inference. CONFIRMATION REQUIRED: do not call this tool unless the user clearly approved the change."
}

func (t *NotesTool) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"stock_number":    {Type: "string", Description: "Exact internal stock_number from stock_check or the user."},
			"note":            {Type: "string", Description: "Concise, business-relevant note explicitly requested by the user."},
			"is_critical":     {Type: "boolean", Description: "Set only when the user explicitly marks or unmarks the item as critical."},
			"minimum_on_hand": {Type: "float", Description: "User-approved minimum quantity. Omit if not provided."},
			"target_on_hand":  {Type: "float", Description: "User-approved replenishment target. Omit if not provided."},
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
		tr.Error = errors.New("error: missing stock_number")
		return tr
	}
	skustring, ok := sku.(string)
	if !ok {

		fmt.Println()
		tr.Error = errors.New("error: cannot convert type on stock_number to string")
	}

	db, err := importer.OpenDB()
	if err != nil {
		fmt.Println(err)
		tr.Error = errors.New("error: could not connect to inventory database")
	}
	note := importer.NewNote(skustring, 1, db)
	text, ok := tc.Function.Arguments["note"].(string)
	if !ok {
		tr.Error = errors.New("error: missing note")
	}
	note.Note = text

	critical, ok := tc.Function.Arguments["is_critical"].(bool)
	if ok {
		note.IsCritical = critical
	}
	min, ok := tc.Function.Arguments["minimum_on_hand"].(float32)
	if ok {
		note.MinimumOnHand = min
	}
	target, ok := tc.Function.Arguments["target_on_hand"].(float32)
	if ok {
		note.TargetOnHand = target
	}
	if err := note.WriteNote(); err != nil {
		fmt.Println(err)
		fmt.Println()
		tr.Error = errors.New("error logging note to system")
	} else {
		tr.Response = "Note saved"
	}
	tr.State = &WorkflowState{
		LastToolUsed:   tc.Function.Name,
		LastItemSearch: skustring,
		clear:          true,
	}
	return tr
}

type PriorityVendor struct{}

func (t *PriorityVendor) Name() string {
	return "priority_vendors"
}

func (t *PriorityVendor) Description() string {
	return "USE WHEN: the user asks which vendors need purchasing attention first or requests a broad reorder-priority overview. RETURNS: vendor numbers ranked by count of critical or high-turn low-stock items. DOES NOT RETURN: item lists, quantities to order, or purchase orders. Use stock_check and item_details to investigate specific items afterward."
}

func (t *PriorityVendor) Parameters() Parameters {
	return Parameters{
		Type: "none",
	}
}

func (t *PriorityVendor) Execute(ctx context.Context, tc ToolCall) ToolResult {
	results, err := importer.GetPriorityVendor()
	tr := ToolResult{Name: tc.Function.Name, Response: ""}
	if err != nil {
		tr.Error = errors.New("could not retrieve vendor data at this time")
	}
	b, err := json.MarshalIndent(results, "", "")
	if err != nil {
		tr.Error = errors.New("could not return inventory data due to database and tool calling error")
		return tr
	}
	tr.Response = string(b)
	tr.State = &WorkflowState{
		LastToolUsed:   tc.Function.Name,
		LastItemSearch: "searched for all priority vendors",
		LastItemFound:  "",
		clear:          false,
	}
	return tr
}

type ItemDetails struct{}

func (t *ItemDetails) Name() string {
	return "item_details"
}

func (t *ItemDetails) Description() string {
	return "USE WHEN: full company data is needed for one item whose exact internal stock_number is known, especially when the user asks for price, cost, vendor, usage, dates, notes, or stocking settings. RETURNS: vendor, price, cost, on_hand, average usage, transaction dates, notes, and stocking settings. DO NOT USE: for keywords, descriptions, uncertain part numbers, or finding candidates; use stock_check first, then retry item_details with the best matching stock_number. READ-ONLY: this tool never changes inventory data. Price and cost lookup is allowed when requested by the user and does not require confirmation."
}

func (t *ItemDetails) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"stock_number": {Type: "string", Description: "Exact internal stock_number only. Do not pass keywords, descriptions, or uncertain vendor part numbers."},
		},
		Required: []string{"stock_number"},
	}
}

func (t *ItemDetails) Execute(ctx context.Context, tc ToolCall) ToolResult {
	tr := ToolResult{
		Name:     tc.Function.Name,
		Response: "",
	}

	sku, ok := tc.Function.Arguments["stock_number"].(string)
	if !ok {
		tr.Error = errors.New("missing argument stock_number")
	}
	result, err := importer.SingleSearch(importer.InventoryLookUpArgs{
		Query:    sku,
		InStock:  false,
		Location: "1",
		Limit:    1,
	})
	if err != nil {
		tr.Error = fmt.Errorf("error: %v", err)
		return tr
	}
	b, err := json.MarshalIndent(result, "", "")
	if err != nil {
		tr.Error = errors.New("could not parse returning inventory data")
		return tr
	}
	tr.State = &WorkflowState{
		LastItemFound:  result.Item.StockNumber,
		LastItemSearch: sku,
		LastToolUsed:   tc.Function.Name,
	}
	tr.Response = string(b)
	return tr
}

type SelectSkill struct {
	Registry *SkillRegistry
}

func (t *SelectSkill) Name() string {
	return "select_skill"
}

func (t *SelectSkill) Description() string {
	return "Use this to select a skill from the skill catalog in the system prompt. You will need to enter the skill name and reason for using the skill in the parameters for calling this tool."
}

func (t *SelectSkill) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"skill_name":     {Type: "string", Description: "the name of the skill you want to call."},
			"entered_reason": {Type: "string", Description: "the reason this skill is necessary."},
		},
		Required: []string{"skill_name", "entered_reason"},
	}
}

func (t *SelectSkill) Execute(ctx context.Context, tc ToolCall) ToolResult {
	return ToolResult{}
}
