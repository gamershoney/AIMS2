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
	config       Config
}

// PermissionRequest is the frontend-safe description of a tool call that is
// paused until the user explicitly approves or denies it.
type PermissionRequest struct {
	Tool        string         `json:"tool"`
	Description string         `json:"description"`
	SkillName   string         `json:"skill_name,omitempty"`
	JobID       int            `json:"job_id,omitempty"`
	Arguments   map[string]any `json:"arguments,omitempty"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	config := LoadConfig()
	newChat := newChatFromConfig(config)
	Reg := InitializeTools(config)

	newChat.skillregistry = NewSkillRegistry(Reg.AllTools())
	if err := newChat.skillregistry.ParseSkills("./skills"); err != nil {
		fmt.Println("error parsing skills:", err)
	}
	Reg.Register(&SelectSkill{Registry: newChat.skillregistry, Chat: newChat})
	newChat.Tools = Reg.ModelTools()
	newChat.registry = Reg
	newChat.SetSystemMessage()
	return &App{Thread: newChat, ToolRegistry: Reg, config: config}
}

func newChatFromConfig(config Config) *Chat {
	return &Chat{
		Model:         config.Model,
		KeepAlive:     config.KeepAlive,
		Stream:        false,
		APIURL:        config.ModelAPIURL,
		HTTPClient:    config.httpClient(),
		DebugPayloads: config.DebugModelPayloads,
		logger:        config.conversationLogger(),
	}
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
	msg.currentTurn++

	if msg.HasActiveSkill() {
		ansr, err := msg.RunActiveSkill(text)
		if err != nil {
			fmt.Println(fmt.Errorf("error: %s", err))
			return ""
		}
		msg.Compact()
		return ansr.Message.Content
	}

	msg.AppendMessage("user", "", text)
	msg.Compact()
	ansr, err := msg.SendMessage()
	if err != nil {
		fmt.Println(fmt.Errorf("error: %s", err))
		return ""
	}

	if msg.HasActiveSkill() {
		ansr, err = msg.RunActiveSkill(text)
		if err != nil {
			fmt.Println(fmt.Errorf("error: %s", err))
			return ""
		}
	}

	msg.Compact()
	return ansr.Message.Content
}

// GetPendingPermission returns the currently paused tool call, if one exists.
// It does not execute or alter the pending request.
func (a *App) GetPendingPermission() *PermissionRequest {
	if a.Thread == nil || a.Thread.skillState == nil {
		return nil
	}
	state := a.Thread.skillState
	if state.Status != SkillStatusWaitingPermission || state.PendingTool == "" {
		return nil
	}

	request := &PermissionRequest{
		Tool:      state.PendingTool,
		SkillName: state.SkillName,
		JobID:     state.JobID,
	}
	if state.Data != nil {
		if args, ok := state.Data["pending_tool_args"].(map[string]any); ok {
			request.Arguments = args
		}
	}
	if tool, err := a.Thread.toolForCall(state.PendingTool); err == nil {
		request.Description = tool.Description()
	}
	return request
}

// ResolveToolPermission resumes a paused tool call using the user's explicit
// decision. Permission decisions stay within the current turn and are not
// interpreted from free-form chat text.
func (a *App) ResolveToolPermission(approved bool) (string, error) {
	msg := a.Thread
	if msg == nil || msg.skillState == nil || msg.skillState.Status != SkillStatusWaitingPermission {
		return "", fmt.Errorf("no tool is waiting for permission")
	}

	if msg.HasActiveSkill() {
		decision := "deny"
		if approved {
			decision = "approve"
		}
		ansr, err := msg.RunActiveSkill(decision)
		if err != nil {
			return "", err
		}
		msg.Compact()
		return ansr.Message.Content, nil
	}

	result := msg.resolvePendingPermission(approved)
	content := result.Response
	if result.Error != nil {
		content = fmt.Sprintf(`{"error":%q}`, result.Error.Error())
	}
	msg.AppendMessage("tool", result.Name, content)
	if !approved {
		msg.Compact()
		return "Permission denied. I did not run the requested tool.", nil
	}

	ansr, err := msg.SendMessage()
	if err != nil {
		return "", err
	}
	msg.Compact()
	return ansr.Message.Content, nil
}

func (a *App) ClearChat() {
	var skillRegistry *SkillRegistry
	if a.Thread != nil {
		skillRegistry = a.Thread.skillregistry
	}

	config := a.config
	if config.Model == "" {
		config = LoadConfig()
	}
	a.Thread = newChatFromConfig(config)
	a.Thread.registry = a.ToolRegistry
	a.Thread.skillregistry = skillRegistry
	if a.ToolRegistry != nil {
		if selectSkill, ok := a.ToolRegistry.tools["select_skill"].(*SelectSkill); ok {
			selectSkill.Chat = a.Thread
		}
		a.Thread.Tools = a.ToolRegistry.ModelTools()
	}
	a.Thread.SetSystemMessage()
}
