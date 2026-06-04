package main

import (
	"context"
	"fmt"
)

// App struct
type App struct {
	ctx    context.Context
	Thread *Chat
}

// NewApp creates a new App application struct
func NewApp() *App {
	newChat := &Chat{Model: defMdoel, KeepAlive: defKeepAlive, Stream: false}
	return &App{Thread: newChat}
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
	msg.AppendMessage("user", text)
	ansr, err := msg.SendMessage()
	if err != nil {
		fmt.Println(fmt.Errorf("error: %s", err))
		return ""
	}
	return ansr.Message.Content
}
