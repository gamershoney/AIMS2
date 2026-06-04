package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	apiUrl       = "http://216.119.200.67:8080/api/"
	defMdoel     = "qwen3.5:9b"
	defKeepAlive = "5m"
)

type Messages struct {
	Role    string
	Content string
}

type Chat struct {
	Messages  []Messages
	Model     string
	Stream    bool
	KeepAlive string
}

type AIMessage struct {
	Content string
}
type Answer struct {
	Model   string
	Message AIMessage
	Done    bool
}

func (m *Chat) SendMessage() (*Answer, error) {
	b, err := createMessage(m)
	fmt.Printf("Message being sent %s\n", b)
	if err != nil {
		return nil, err
	}
	br := bytes.NewReader(b)
	resp, err := http.Post(apiUrl+"chat", "application/json", br)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error on sending message. StatusCode: %d\n response: %s", resp.StatusCode, body)
	}

	ansr := &Answer{}
	err = json.Unmarshal(body, ansr)
	fmt.Printf("Response: %s, done:%t, model:%s", ansr.Message.Content, ansr.Done, ansr.Model)
	if err != nil {
		return nil, fmt.Errorf("error : %s", err)
	}
	m.AppendMessage("assistant", ansr.Message.Content)
	return ansr, err
}

func (m *Chat) AppendMessage(role string, text string) {
	m.Messages = append(m.Messages, Messages{Role: role, Content: text})
}

func createMessage(m *Chat) ([]byte, error) {
	b, err := json.Marshal(m)
	return b, err
}
