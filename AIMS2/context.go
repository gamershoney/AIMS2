package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

const (
	systemPromptIndex = 0
	compactTrigger    = 3
)

func (m *Chat) Compact() {
	if m.currentTurn < compactTrigger {
		return
	}

	kept := make([]Messages, 0, len(m.Messages))

	for i, v := range m.Messages {
		if i == systemPromptIndex {
			kept = append(kept, v)
			continue
		}
		if v.turn == m.currentTurn {
			kept = append(kept, v)
			continue
		}
		if len(v.ToolCalls) == 0 && v.Role != "tool" {
			kept = append(kept, v)
			continue
		}

	}
	m.Messages = kept
}

type WorkflowState struct {
	LastToolUsed   string `json:"last_tool_used,omitempty"`
	LastItemSearch string `json:"last_item_searched,omitempty"`
	LastItemFound  string `json:"last_item_found,omitempty"`
	clear          bool
}

func (m *Chat) ApplyState(wf *WorkflowState) error {
	if wf == nil {
		return nil
	}
	state := m.workflowState
	if state == nil {
	} else if state.clear {
		wf.LastToolUsed = ""
		wf.LastItemSearch = ""
		wf.LastItemFound = ""
		wf.clear = false

	}
	m.workflowState = wf

	b, err := json.MarshalIndent(m.workflowState, "", "\t")
	if err != nil {
		return err
	}
	s := string(b)
	msg := Messages{
		Role:     "system",
		ToolName: "workflow_state",
		turn:     0,
		Content:  "Current Working State:\n" + s,
	}

	if m.replaceSystemSlot("workflow_state", msg) {
		return nil
	}
	if m.replaceEmptySystemSlot(msg) {
		return nil
	}
	m.Messages = slices.Insert(m.Messages, min(len(m.Messages), 1), msg)
	return nil
}

type ToolPermission string

const (
	PermissionNone     ToolPermission = "none"
	PermissionRequired ToolPermission = "required"
)

const (
	SkillStatusActive            = "active"
	SkillStatusWaitingPermission = "waiting_permission"
	SkillStatusCompleted         = "completed"
)

var JobID int = 0

type StateName string

const (
	StateStarted   StateName = "entered"
	StateWorking   StateName = "working"
	StateCompleted StateName = "completed"
)

type SkillState struct {
	JobID         int            `json:"job_id"`
	SkillName     string         `json:"skill_name"`
	StateName     StateName      `json:"state_name"`
	EnteredReason string         `json:"entered_reason"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	PendingTool   string         `json:"pending_tool,omitempty"`
}

type ToolMap map[string]Tool

type SkillSession struct {
	State        *SkillState
	Chat         *Chat
	AllowedTools ToolMap
	Skill        *Skill
	StepCount    int
}

func CreateSession(skill *Skill, tools ToolMap, reason string) *SkillSession {
	session := &SkillSession{}
	JobID += 1
	state := &SkillState{}
	state.JobID = JobID
	state.SkillName = skill.Name
	state.StateName = StateStarted
	state.Status = SkillStatusActive
	state.EnteredReason = reason
	session.State = state
	session.Skill = skill

	session.AllowedTools = filterAllowedTools(skill, tools)
	session.Chat = newSkillChat(skill, session.AllowedTools, state)
	return session
}

func newSkillChat(skill *Skill, allowedTools ToolMap, state *SkillState) *Chat {
	config := LoadConfig()
	registry := registryFromToolMap(allowedTools)
	registry.Register(&UpdateSkillState{State: state})
	registry.Register(&CompleteSkill{State: state})
	chat := &Chat{
		Model:      config.Model,
		KeepAlive:  config.KeepAlive,
		Stream:     false,
		APIURL:     config.ModelAPIURL,
		HTTPClient: config.httpClient(),
		registry:   registry,
		Tools:      registry.ModelTools(),
		skillState: state,
	}
	chat.Messages = []Messages{
		{
			Role:    "system",
			Content: buildSkillSystemPrompt(skill, allowedTools),
		},
		{
			Role:     "system",
			ToolName: "skill_state",
			Content:  "",
		},
	}
	_ = chat.ApplySkillState(state)
	return chat
}

func registryFromToolMap(tools ToolMap) *Registry {
	registry := &Registry{tools: make(map[string]Tool)}
	for _, tool := range tools {
		registry.Register(tool)
	}
	return registry
}

func buildSkillSystemPrompt(skill *Skill, allowedTools ToolMap) string {
	names := make([]string, 0, len(allowedTools))
	for name := range allowedTools {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("You are running an isolated workflow skill.\n")
	b.WriteString("Skill: ")
	b.WriteString(skill.Name)
	b.WriteString("\n")
	if len(names) > 0 {
		b.WriteString("Allowed tools: ")
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n")
	} else {
		b.WriteString("Allowed tools: none\n")
	}
	b.WriteString("Use only the allowed tools for this skill. Return concise progress summaries and update skill state when tool results provide it.\n\n")
	b.WriteString("Skill instructions:\n")
	b.WriteString(skill.Body)
	return b.String()
}

func filterAllowedTools(skill *Skill, tools ToolMap) ToolMap {
	allowed := make(ToolMap)
	for _, name := range skill.AllowedTools {
		if tool, ok := tools[name]; ok {
			allowed[name] = tool
		}
	}
	return allowed
}

type SkillRegistry struct {
	Registry map[string]*Skill
	Tools    ToolMap
}

func NewSkillRegistry(stools []Tool) *SkillRegistry {
	reg := make(map[string]*Skill)
	tools := make(map[string]Tool)
	for _, v := range stools {
		tools[v.Name()] = v
	}

	return &SkillRegistry{Registry: reg, Tools: tools}
}

type Skill struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	AllowedTools    []string `yaml:"allowed_tools"`
	RestrictedTools []string `yaml:"requires_approval_for"`
	MaxSteps        int      `yaml:"max_steps"`
	Body            string
	Path            string
}

func ReadSkill(path string) (*Skill, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	skill := &Skill{}
	err = yaml.Unmarshal(file, skill)
	if err != nil {
		return nil, err
	}
	md := string(file)

	md = strings.TrimPrefix(md, "\ufeff")

	if strings.Count(md, "---\n") != 2 {
		return nil, errors.New("error malformed line breaks in skill")
	}
	split := strings.SplitAfter(md, "---\n")
	if len(split) < 2 {
		return nil, errors.New("error: could not parse yaml")
	}
	md = split[len(split)-1]

	if md == "" {
		return nil, errors.New("error: empty frontmatter")
	}
	skill.Body = md
	skill.Path = path
	return skill, err
}

func (sr *SkillRegistry) ParseSkills(dir string) error {
	fldr, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, item := range fldr {
		if item.IsDir() {
			skillpath := filepath.Join(dir, item.Name(), "SKILL.md")
			skill, err := ReadSkill(skillpath)
			if err != nil {
				return err
			}
			sr.RegisterSkill(skill)
		}
	}
	return err
}

func (sr *SkillRegistry) RegisterSkill(skill *Skill) {
	sr.Registry[skill.Name] = skill
}

func (sr *SkillRegistry) MakeCatalog() string {
	if sr == nil || len(sr.Registry) == 0 {
		return "Available workflow skills: none"
	}

	names := make([]string, 0, len(sr.Registry))
	for name := range sr.Registry {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Available workflow skills:\n")
	for _, name := range names {
		skill := sr.Registry[name]
		b.WriteString("- ")
		b.WriteString(skill.Name)
		b.WriteString(": ")
		b.WriteString(skill.Description)
		if len(skill.AllowedTools) > 0 {
			b.WriteString(" Allowed tools: ")
			b.WriteString(strings.Join(skill.AllowedTools, ", "))
			b.WriteString(".")
		}
		b.WriteString("\n")
	}
	b.WriteString("If a user request matches a workflow skill, call select_skill with the skill name and reason.")
	return b.String()
}

func (sr *SkillRegistry) RunSkill(skill *Skill, reason string) *SkillSession {
	return CreateSession(skill, sr.Tools, reason)
}

func (m *Chat) ApplySkillState(state *SkillState) error {
	if state == nil {
		return nil
	}
	m.skillState = state

	b, err := json.MarshalIndent(m.skillState, "", "\t")
	if err != nil {
		return err
	}

	msg := Messages{
		Role:     "system",
		ToolName: "skill_state",
		turn:     0,
		Content:  "Current Skill State:\n" + string(b),
	}

	if m.replaceSystemSlot("skill_state", msg) {
		return nil
	}
	if m.replaceEmptySystemSlot(msg) {
		return nil
	}
	m.Messages = slices.Insert(m.Messages, min(len(m.Messages), 2), msg)
	return nil
}

func (m *Chat) replaceSystemSlot(toolName string, msg Messages) bool {
	for i, existing := range m.Messages {
		if existing.Role == "system" && existing.ToolName == toolName {
			m.Messages[i] = msg
			return true
		}
	}
	return false
}

func (m *Chat) replaceEmptySystemSlot(msg Messages) bool {
	for i := 1; i < len(m.Messages); i++ {
		existing := m.Messages[i]
		if existing.Role == "system" && existing.ToolName == "" && existing.Content == "" {
			m.Messages[i] = msg
			return true
		}
	}
	return false
}
