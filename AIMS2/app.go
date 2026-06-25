package main

import (
	"context"
	"fmt"
)

// App struct
type App struct {
	ctx          context.Context
	Thread       *Chat
	ToolRegistry *Registry
}

// NewApp creates a new App application struct
func NewApp() *App {
	logger := NewConversationLogger("")
	newChat := &Chat{Model: gemmaModel, KeepAlive: defKeepAlive, Stream: false, logger: logger}
	Reg := InitializeTools()
	newChat.Tools = Reg.ModelTools()
	newChat.registry = Reg
	newChat.SetSystemMessage()
	return &App{Thread: newChat, ToolRegistry: Reg}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) SendMsg(text string) string {
	msg := a.Thread
	msg.AppendMessage("user", "", text)
	ansr, err := msg.SendMessage()
	if err != nil {
		fmt.Println(fmt.Errorf("error: %s", err))
		return ""
	}
	return ansr.Message.Content
}

func (a *App) ClearChat() {
	a.Thread = &Chat{Model: defMdoel, KeepAlive: defKeepAlive, Stream: false}
}
