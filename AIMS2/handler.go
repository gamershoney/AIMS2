package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	apiUrl       = "http://216.119.200.67:8080/api/"
	defMdoel     = "qwen3.5:9b"
	gemmaModel   = "gemma4:12b"
	defKeepAlive = "5m"
	systemPrompt = "You are a purchasing/sales assistant for a welding distrubutor. Your users will only be outside salespeople and purchasing managers. Frame all of your responses for that demographic. It is important that responses are not given with emojis and are styled professionally."
)

type Messages struct {
	Role     string
	ToolName string `json:"tool_name,omitempty"`
	Content  string
}

type Chat struct {
	Messages  []Messages
	Model     string
	Stream    bool
	KeepAlive string
	Tools     []ModelTool `json:"tools,omitempty"`
	registry  *Registry
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
	Content   string
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}
type Answer struct {
	Model   string
	Message AIMessage
	Done    bool
}

func (m *Chat) SendMessage() (*Answer, error) {
	b, err := createMessage(m)
	if err != nil {
		return nil, err
	}
	br := bytes.NewReader(b)
	resp, err := http.Post(apiUrl+"chat", "application/json", br)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error on sending message. StatusCode: %d\n response: %s", resp.StatusCode, body)
	}

	ansr := &Answer{}
	fmt.Println(string(body))
	fmt.Println("")
	err = json.Unmarshal(body, ansr)
	if err != nil {
		return nil, fmt.Errorf("error : %s", err)
	}
	fmt.Printf("Response: done:%t, model:%s, ToolCalls: %v \n\n", ansr.Done, ansr.Model, ansr.Message.ToolCalls)
	m.AppendMessage("assistant", "", ansr.Message.Content)

	if len(ansr.Message.ToolCalls) > 0 {
		ansr := m.RunToolCalls(ansr.Message)
		return ansr, err
	}
	fmt.Println("Finalizing Response to user")
	return ansr, err
}

func (m *Chat) RunToolCalls(ai AIMessage) *Answer {
	ch := make(chan ToolResult, len(ai.ToolCalls))
	ctx := context.TODO()

	for _, v := range ai.ToolCalls {
		_, ok := m.registry.tools[v.Function.Name]
		if !ok {
			ch <- ToolResult{Name: v.Function.Name, Error: errors.New("Tool not found")}
			continue
		}
		go func(t ToolCall) {
			results := m.registry.tools[t.Function.Name].Execute(ctx, t)
			ch <- results
		}(v)
	}
	var tr []ToolResult
	for range ai.ToolCalls {
		trindex := <-ch
		tr = append(tr, trindex)
		if trindex.Error != nil {
			fmt.Println("error on tool results: ", trindex.Error)
			continue
		}
		fmt.Println("Tool Result for tool: ", trindex.Name)
		m.AppendMessage("tool", trindex.Name, trindex.Response)
	}
	ansr, err := m.SendMessage()
	if err != nil {
		fmt.Println("error: ", err)
	}

	return ansr
}

func (m *Chat) AppendMessage(role string, toolname string, text string) {
	msg := Messages{Role: role, Content: text}
	if toolname != "" {
		msg.ToolName = toolname
	}
	m.Messages = append(m.Messages, msg)
}

func createMessage(m *Chat) ([]byte, error) {
	b, err := json.Marshal(m)
	return b, err
}

func (m *Chat) SetSystemMessage() {
	m.AppendMessage("system", "", systemPrompt)
}
