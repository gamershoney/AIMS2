package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	apiUrl       = "http://216.119.200.67:8080/api/"
	defMdoel     = "qwen3.5:9b"
	gemmaModel   = "gemma4:12b"
	defKeepAlive = "5m"

	systemPrompt = "You are a purchasing/sales assistant for a welding distributor. Your users will only be outside salespeople and purchasing managers. Frame all of your responses for that demographic. It is important that responses are not given with emojis and are styled professionally. Never answer price, cost, vendor, usage, received dates, sales dates, notes, or full item details from stock_check alone. stock_check is only for candidate discovery and availability. Use item_details for commercial or transaction fields before answering those questions."
)

type Messages struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	turn      int
}

type Chat struct {
	Messages      []Messages  `json:"messages"`
	Model         string      `json:"model"`
	Stream        bool        `json:"stream"`
	KeepAlive     string      `json:"keep_alive"`
	Tools         []ModelTool `json:"tools,omitempty"`
	workflowState *WorkflowState
	skillState    *SkillState
	currentTurn   int
	registry      *Registry
	skillregistry *SkillRegistry
	skillSession  *SkillSession
	logger        *ConversationLogger
	sendOnceFunc  func(*Chat) (*Answer, error)
}

type ToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Index     int64          `json:"index"`
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function,omitempty"`
}

type AIMessage struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Answer struct {
	Model   string    `json:"model"`
	Message AIMessage `json:"message"`
	Done    bool      `json:"done"`
}

func createMessage(m *Chat) ([]byte, error) {
	return json.Marshal(m)
}

func (m *Chat) SendMessage() (*Answer, error) {
	return m.sendMessageWithLimit(5)
}

func (m *Chat) sendMessageWithLimit(maxToolRounds int) (*Answer, error) {
	for round := 0; round < maxToolRounds; round++ {
		if m.skillSession != nil {
			m.skillSession.StepCount++
			if err := m.enforceSkillStepLimit(); err != nil {
				return nil, err
			}
		}

		ansr, err := m.sendOnce()
		if err != nil {
			if m.logger != nil {
				_ = m.logger.LogError("send_message", err.Error())
			}
			return nil, err
		}

		fmt.Printf(
			"Response: done:%t, model:%s, ToolCalls: %v\n\n",
			ansr.Done,
			ansr.Model,
			ansr.Message.ToolCalls,
		)

		// This is critical.
		// Save the assistant response WITH tool_calls.
		m.AppendAssistantMessage(ansr.Message)

		// No tool calls means this is the final answer for the user.
		if len(ansr.Message.ToolCalls) == 0 {
			fmt.Println("Finalizing Response to user")
			return ansr, nil
		}

		// Run tools, append tool results, then loop back and ask the model again.
		toolResults := m.RunToolCalls(ansr.Message)
		toolResults = append(toolResults, m.RunRequiredDetailFollowUps(toolResults)...)

		for _, tr := range toolResults {
			if tr.Error != nil {
				fmt.Println("error on tool results:", tr.Error)

				errResponse := fmt.Sprintf(`{"error":%q}`, tr.Error.Error())
				m.AppendMessage("tool", tr.Name, errResponse)
				continue
			}

			fmt.Println("Tool Result for tool:", tr.Name, tr.Response)
			m.AppendMessage("tool", tr.Name, tr.Response)
		}

		if m.HasActiveSkill() {
			return &Answer{
				Model: m.Model,
				Message: AIMessage{
					Role:    "assistant",
					Content: "Starting workflow skill.",
				},
				Done: true,
			}, nil
		}
	}

	if m.logger != nil {
		_ = m.logger.LogError("send_message", "too many tool call rounds")
	}
	return nil, fmt.Errorf("too many tool call rounds")
}

func (m *Chat) sendOnce() (*Answer, error) {
	if m.sendOnceFunc != nil {
		return m.sendOnceFunc(m)
	}

	b, err := createMessage(m)
	if err != nil {
		return nil, err
	}

	fmt.Printf("MODEL REQUEST SIZE: bytes=%d messages=%d\n", len(b), len(m.Messages))
	fmt.Println("MESSAGES SENT TO MODEL:")
	fmt.Println(string(b))

	if m.logger != nil {
		_ = m.logger.LogAPIRequest(m.Model)
	}

	br := bytes.NewReader(b)

	resp, err := http.Post(apiUrl+"chat", "application/json", br)
	if err != nil {
		if m.logger != nil {
			_ = m.logger.LogError("api_request", err.Error())
		}
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("error on sending message. StatusCode: %d\nresponse: %s", resp.StatusCode, body)
		if m.logger != nil {
			_ = m.logger.LogError("api_response", errMsg)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	fmt.Println(string(body))
	fmt.Println("")

	ansr := &Answer{}

	if err := json.Unmarshal(body, ansr); err != nil {
		return nil, fmt.Errorf("unmarshal chat response: %w", err)
	}

	if m.logger != nil {
		_ = m.logger.LogAPIResponse(ansr.Model, ansr.Done, ansr.Message.Content)
	}

	return ansr, nil
}

func (m *Chat) HasActiveSkill() bool {
	return m.skillSession != nil &&
		m.skillState != nil &&
		m.skillState.StateName != StateCompleted
}

func (m *Chat) RunActiveSkill(userText string) (*Answer, error) {
	if !m.HasActiveSkill() {
		return nil, errors.New("no active skill session")
	}
	resolvedPermission := false
	if m.skillState.Status == SkillStatusWaitingPermission {
		approved, handled := parsePermissionDecision(userText)
		if !handled {
			return &Answer{
				Model: m.Model,
				Message: AIMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("Permission is required before running %s. Please approve or deny this action.", m.skillState.PendingTool),
				},
				Done: true,
			}, nil
		}
		resolvedPermission = true

		result := m.resolvePendingPermission(approved)
		content := result.Response
		if result.Error != nil {
			content = fmt.Sprintf(`{"error":%q}`, result.Error.Error())
		}
		m.skillSession.Chat.AppendMessage("tool", result.Name, content)
		if !approved {
			return &Answer{
				Model: m.Model,
				Message: AIMessage{
					Role:    "assistant",
					Content: "Permission denied. I did not run the requested tool.",
				},
				Done: true,
			}, nil
		}
	}

	session := m.skillSession
	if !resolvedPermission && strings.TrimSpace(userText) != "" {
		session.Chat.currentTurn++
		session.Chat.AppendMessage("user", "", userText)
	}
	maxRounds := session.Skill.MaxSteps
	if maxRounds <= 0 {
		maxRounds = 5
	}

	ansr, err := session.Chat.sendMessageWithLimit(maxRounds)
	if session.Chat.workflowState != nil {
		_ = m.ApplyState(session.Chat.workflowState)
	}
	if session.Chat.skillState != nil {
		session.State = session.Chat.skillState
		_ = m.ApplySkillState(session.State)
	}
	if err != nil {
		return nil, err
	}

	if session.State.StateName == StateCompleted {
		summary := session.State.Summary
		if strings.TrimSpace(summary) == "" {
			summary = ansr.Message.Content
		}
		m.AppendMessage("system", "skill_summary", buildSkillSummary(session.State))
		m.skillSession = nil
		return &Answer{
			Model: ansr.Model,
			Message: AIMessage{
				Role:    "assistant",
				Content: summary,
			},
			Done: true,
		}, nil
	}

	return ansr, nil
}

func (m *Chat) resolvePendingPermission(approved bool) ToolResult {
	if m.skillState == nil || m.skillState.PendingTool == "" {
		return ToolResult{Name: "", Error: errors.New("no pending permission request")}
	}

	toolName := m.skillState.PendingTool
	args := map[string]any{}
	if m.skillState.Data != nil {
		if rawArgs, ok := m.skillState.Data["pending_tool_args"].(map[string]any); ok {
			args = rawArgs
		}
	}

	m.skillState.PendingTool = ""
	m.skillState.Status = SkillStatusActive
	m.skillState.StateName = StateWorking
	if m.skillState.Data != nil {
		delete(m.skillState.Data, "pending_tool_args")
	}

	if !approved {
		_ = m.ApplySkillState(m.skillState)
		response := fmt.Sprintf(`{"status":"permission_denied","tool":%q}`, toolName)
		return ToolResult{Name: toolName, Response: response, SkillState: m.skillState}
	}

	tool, err := m.toolForCall(toolName)
	if err != nil {
		return ToolResult{Name: toolName, Error: err, SkillState: m.skillState}
	}

	tc := ToolCall{}
	tc.Function.Name = toolName
	tc.Function.Arguments = args

	result := tool.Execute(context.TODO(), tc)
	if result.Name == "" {
		result.Name = toolName
	}
	result.Response = result.truncate(maxToolResponse)

	if result.State != nil {
		_ = m.ApplyState(result.State)
	}
	if result.SkillState != nil {
		_ = m.ApplySkillState(result.SkillState)
	} else {
		result.SkillState = m.skillState
		_ = m.ApplySkillState(m.skillState)
	}
	return result
}

func parsePermissionDecision(text string) (bool, bool) {
	normalized := strings.TrimSpace(strings.ToLower(text))
	if normalized == "" {
		return false, false
	}
	approveTerms := []string{"approve", "approved", "allow", "allowed", "yes", "y", "go ahead", "run it"}
	for _, term := range approveTerms {
		if normalized == term || strings.Contains(normalized, term) {
			return true, true
		}
	}
	denyTerms := []string{"deny", "denied", "do not", "don't", "no", "n", "cancel", "stop"}
	for _, term := range denyTerms {
		if normalized == term || strings.Contains(normalized, term) {
			return false, true
		}
	}
	return false, false
}

func (m *Chat) enforceSkillStepLimit() error {
	if m.skillSession == nil || m.skillSession.Skill == nil {
		return nil
	}
	maxSteps := m.skillSession.Skill.MaxSteps
	if maxSteps <= 0 || m.skillSession.StepCount <= maxSteps {
		return nil
	}
	state := m.skillSession.State
	if state != nil {
		state.StateName = StateCompleted
		state.Status = SkillStatusCompleted
		state.Warnings = append(state.Warnings, fmt.Sprintf("Skill stopped after reaching max_steps=%d.", maxSteps))
		state.Summary = "Skill stopped because it reached the configured maximum number of steps."
		_ = m.ApplySkillState(state)
	}
	return fmt.Errorf("skill %q exceeded max_steps=%d", m.skillSession.Skill.Name, maxSteps)
}

func buildSkillSummary(state *SkillState) string {
	if state == nil {
		return ""
	}
	b, err := json.Marshal(state)
	if err != nil {
		return state.Summary
	}
	return "Completed Skill State:\n" + string(b)
}

func (m *Chat) RunToolCalls(ai AIMessage) []ToolResult {
	ch := make(chan ToolResult, len(ai.ToolCalls))
	ctx := context.TODO()

	for _, v := range ai.ToolCalls {
		tool, err := m.toolForCall(v.Function.Name)
		if err != nil {
			if m.logger != nil {
				_ = m.logger.LogError("tool_call", err.Error())
			}
			ch <- ToolResult{
				Name:  v.Function.Name,
				Error: err,
			}
			continue
		}

		if m.logger != nil {
			_ = m.logger.LogToolCall(v.Function.Name, v.Function.Arguments)
		}

		if blocked := m.permissionGateResult(tool, v); blocked != nil {
			ch <- *blocked
			continue
		}

		go func(tool Tool, t ToolCall) {
			result := tool.Execute(ctx, t)

			// Make sure the result has the tool name.
			if result.Name == "" {
				result.Name = t.Function.Name
			}

			if m.logger != nil {
				_ = m.logger.LogToolResult(result.Name, result.Response, result.Error)
			}

			ch <- result
		}(tool, v)
	}

	results := make([]ToolResult, 0, len(ai.ToolCalls))

	for range ai.ToolCalls {
		result := <-ch
		result.Response = result.truncate(maxToolResponse)
		if result.State != nil {
			m.workflowState = result.State
			err := m.ApplyState(result.State)
			if err != nil {
				continue
			}

		}
		if result.SkillState != nil {
			if err := m.ApplySkillState(result.SkillState); err != nil {
				if m.logger != nil {
					_ = m.logger.LogError("skill_state", err.Error())
				}
				continue
			}
		}
		results = append(results, result)
	}

	return results
}

func (m *Chat) RunRequiredDetailFollowUps(results []ToolResult) []ToolResult {
	if !asksForItemDetails(m.lastUserMessage()) {
		return nil
	}
	if toolResultsInclude(results, "item_details") {
		return nil
	}

	stockNumber := exactSingleStockCheckResult(results)
	if stockNumber == "" {
		return nil
	}

	return []ToolResult{m.runToolByName("item_details", map[string]any{
		"stock_number": stockNumber,
	})}
}

func (m *Chat) runToolByName(name string, args map[string]any) ToolResult {
	tool, err := m.toolForCall(name)
	if err != nil {
		return ToolResult{Name: name, Error: err}
	}

	if m.logger != nil {
		_ = m.logger.LogToolCall(name, args)
	}

	tc := ToolCall{}
	tc.Function.Name = name
	tc.Function.Arguments = args

	if blocked := m.permissionGateResult(tool, tc); blocked != nil {
		return *blocked
	}

	result := tool.Execute(context.TODO(), tc)
	if result.Name == "" {
		result.Name = name
	}
	result.Response = result.truncate(maxToolResponse)

	if result.State != nil {
		if err := m.ApplyState(result.State); err != nil && m.logger != nil {
			_ = m.logger.LogError("workflow_state", err.Error())
		}
	}
	if result.SkillState != nil {
		if err := m.ApplySkillState(result.SkillState); err != nil && m.logger != nil {
			_ = m.logger.LogError("skill_state", err.Error())
		}
	}

	if m.logger != nil {
		_ = m.logger.LogToolResult(result.Name, result.Response, result.Error)
	}

	return result
}

func (m *Chat) toolForCall(name string) (Tool, error) {
	if m.skillSession != nil && m.skillState != nil && m.skillState.StateName != StateCompleted {
		tool, ok := m.skillSession.AllowedTools[name]
		if !ok {
			return nil, fmt.Errorf("tool %q is not allowed while skill %q is active", name, m.skillState.SkillName)
		}
		return tool, nil
	}

	if m.registry == nil || m.registry.tools == nil {
		return nil, errors.New("tool registry is not initialized")
	}

	tool, ok := m.registry.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

func (m *Chat) permissionGateResult(tool Tool, tc ToolCall) *ToolResult {
	if tool.Permission() != PermissionRequired {
		return nil
	}

	state := m.skillState
	if state == nil {
		state = &SkillState{
			StateName: StateWorking,
			Status:    SkillStatusWaitingPermission,
		}
	} else {
		state.StateName = StateWorking
		state.Status = SkillStatusWaitingPermission
	}
	state.PendingTool = tc.Function.Name
	if state.Data == nil {
		state.Data = make(map[string]any)
	}
	state.Data["pending_tool_args"] = tc.Function.Arguments

	if err := m.ApplySkillState(state); err != nil && m.logger != nil {
		_ = m.logger.LogError("skill_state", err.Error())
	}

	response := map[string]any{
		"status":  "permission_required",
		"tool":    tc.Function.Name,
		"message": "This tool requires user permission before it can run. Ask the user for approval.",
	}
	b, err := json.Marshal(response)
	if err != nil {
		return &ToolResult{Name: tc.Function.Name, Error: err, SkillState: state}
	}
	return &ToolResult{Name: tc.Function.Name, Response: string(b), SkillState: state}
}

func (m *Chat) lastUserMessage() string {
	for i := len(m.Messages) - 1; i >= 0; i-- {
		if m.Messages[i].Role == "user" {
			return m.Messages[i].Content
		}
	}
	return ""
}

func asksForItemDetails(text string) bool {
	text = strings.ToLower(text)
	terms := []string{
		"cost",
		"price",
		"retail",
		"selling price",
		"sell price",
		"vendor",
		"usage",
		"received",
		"receive",
		"last sale",
		"last shipment",
		"shipment",
		"note",
		"minimum",
		"target",
		"stocking",
		"full detail",
		"details",
	}
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func toolResultsInclude(results []ToolResult, name string) bool {
	for _, result := range results {
		if result.Name == name && result.Error == nil {
			return true
		}
	}
	return false
}

func exactSingleStockCheckResult(results []ToolResult) string {
	for _, result := range results {
		if result.Name != "stock_check" || result.Error != nil {
			continue
		}

		var response StockCheckResponse
		if err := json.Unmarshal([]byte(result.Response), &response); err != nil {
			continue
		}
		if response.Count != 1 || len(response.Results) != 1 {
			continue
		}
		return response.Results[0].StockNumber
	}
	return ""
}

func (m *Chat) AppendMessage(role string, toolname string, text string) {
	msg := Messages{
		Role:    role,
		Content: text,
		turn:    m.currentTurn,
	}

	if toolname != "" {
		msg.ToolName = toolname
	}

	m.Messages = append(m.Messages, msg)

	if m.logger != nil {
		_ = m.logger.LogMessage(role, toolname, text)
	}
}

func (m *Chat) AppendAssistantMessage(ai AIMessage) {
	m.Messages = append(m.Messages, Messages{
		Role:      "assistant",
		Content:   ai.Content,
		ToolCalls: ai.ToolCalls,
		turn:      m.currentTurn,
	})
}

func (m *Chat) SetSystemMessage() {
	m.AppendMessage("system", "", systemPrompt)
	if m.skillregistry != nil {
		m.AppendMessage("system", "skill_catalog", m.skillregistry.MakeCatalog())
	}
	m.AppendMessage("system", "workflow_state", "")
	m.AppendMessage("system", "skill_state", "")
}
