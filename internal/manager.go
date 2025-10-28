package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/alvinunreal/tmuxai/config"
	"github.com/alvinunreal/tmuxai/logger"
	"github.com/alvinunreal/tmuxai/system"
	"github.com/fatih/color"
)

const reflectionLogLimit = 500

type AIResponse struct {
	Message                string
	SendKeys               []string
	ExecCommand            []string
	PasteMultilineContent  string
	RequestAccomplished    bool
	ExecPaneSeemsBusy      bool
	WaitingForUserResponse bool
	NoComment              bool
}

// AiClientInterface defines the interface for AI clients to make testing easier
type AiClientInterface interface {
	GetResponseFromChatMessages(ctx context.Context, messages []ChatMessage, model string) (string, error)
	ChatCompletion(ctx context.Context, messages []Message, model string) (string, error)
}

// Parsed only when pane is prepared
type CommandExecHistory struct {
	Command string
	Output  string
	Code    int
}

type CommandReflection struct {
	Command              string
	Output               string
	ExitCode             int
	LessonsLearned       string
	Alternative          string
	AlternativeRationale string
	SuggestedActions     []SuggestedToolAction
	Timestamp            time.Time
}

type SuggestedToolAction struct {
	Action      string
	Name        string
	Section     string
	Description string
	Reason      string
	Outcome     string
}

type ReflectionTask struct {
	History CommandExecHistory
}

// Manager represents the TmuxAI manager agent
type Manager struct {
	Config             *config.Config
	AiClient           AiClientInterface
	Status             string // running, waiting, done
	PaneId             string
	ExecPane           *system.TmuxPaneDetails
	Messages           []ChatMessage
	ExecHistory        []CommandExecHistory
	ReflectionLog      []CommandReflection
	pendingReflections []ReflectionTask
	WatchMode          bool
	OS                 string
	CurrentPersona     string
	SessionOverrides   map[string]interface{} // session-only config overrides

	// Functions for mocking
	confirmedToExec   func(command string, prompt string, edit bool) (bool, string)
	getTmuxPanesInXml func(config *config.Config) string
}

// NewManager creates a new manager agent
func NewManager(cfg *config.Config) (*Manager, error) {
	if cfg.OpenRouter.APIKey == "" && cfg.AzureOpenAI.APIKey == "" {
		fmt.Println("An API key is required. Set OpenRouter or Azure OpenAI credentials in the config file or environment variables.")
		return nil, fmt.Errorf("API key required")
	}

	paneId, err := system.TmuxCurrentPaneId()
	if err != nil {
		// If we're not in a tmux session, start a new session and execute the same command
		paneId, err = system.TmuxCreateSession()
		if err != nil {
			return nil, fmt.Errorf("system.TmuxCreateSession failed: %w", err)
		}
		args := strings.Join(os.Args[1:], " ")

		_ = system.TmuxSendCommandToPane(paneId, "tmuxai "+args, true)
		// shell initialization may take some time
		time.Sleep(1 * time.Second)
		_ = system.TmuxSendCommandToPane(paneId, "Enter", false)
		err = system.TmuxAttachSession(paneId)
		if err != nil {
			return nil, fmt.Errorf("system.TmuxAttachSession failed: %w", err)
		}
		os.Exit(0)
	}

	aiClient := NewAiClient(cfg)
	os := system.GetOSDetails()

	manager := &Manager{
		Config:           cfg,
		AiClient:         aiClient,
		PaneId:           paneId,
		Messages:         []ChatMessage{},
		ExecPane:         &system.TmuxPaneDetails{},
		OS:               os,
		SessionOverrides: make(map[string]interface{}),
	}

	manager.confirmedToExec = manager.confirmedToExecFn
	manager.getTmuxPanesInXml = manager.getTmuxPanesInXmlFn

	manager.CurrentPersona = manager.selectPersona()
	logger.Debug("Selected persona: %s", manager.CurrentPersona)
	manager.InitExecPane()
	manager.loadReflectionLog()
	return manager, nil
}

func (m *Manager) enqueueReflection(history CommandExecHistory) {
	if strings.TrimSpace(history.Command) == "" {
		return
	}
	m.pendingReflections = append(m.pendingReflections, ReflectionTask{History: history})
}

func (m *Manager) processPendingReflections(ctx context.Context) {
	for len(m.pendingReflections) > 0 {
		var task ReflectionTask
		task, m.pendingReflections = m.pendingReflections[0], m.pendingReflections[1:]

		reflection, err := m.runReflection(ctx, task)
		if err != nil {
			logger.Warn("Reflection failed for command '%s': %v", task.History.Command, err)
			continue
		}

		reflection.SuggestedActions = m.applyToolActions(reflection.SuggestedActions)
		m.ReflectionLog = append(m.ReflectionLog, reflection)
		m.pruneReflectionLog()
		m.persistReflectionLog()

		summary := formatReflectionSummary(reflection)
		m.Messages = append(m.Messages, ChatMessage{Content: summary, FromUser: false, Timestamp: time.Now()})
		m.Println(summary)
	}
}

func (m *Manager) reflectionLogPath() string {
	return config.GetConfigFilePath("lessons-learned.json")
}

func (m *Manager) loadReflectionLog() {
	path := m.reflectionLogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		logger.Warn("Failed to read reflection log: %v", err)
		return
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return
	}
	var reflections []CommandReflection
	if err := json.Unmarshal([]byte(trimmed), &reflections); err != nil {
		logger.Warn("Failed to parse reflection log: %v", err)
		return
	}
	m.ReflectionLog = append(m.ReflectionLog, reflections...)
	m.pruneReflectionLog()
}

func (m *Manager) persistReflectionLog() {
	m.pruneReflectionLog()
	path := m.reflectionLogPath()
	data, err := json.MarshalIndent(m.ReflectionLog, "", "  ")
	if err != nil {
		logger.Warn("Failed to encode reflection log: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		logger.Warn("Failed to write reflection log: %v", err)
	}
}

func (m *Manager) pruneReflectionLog() {
	if reflectionLogLimit <= 0 {
		return
	}
	if len(m.ReflectionLog) <= reflectionLogLimit {
		return
	}
	dropped := len(m.ReflectionLog) - reflectionLogLimit
	m.ReflectionLog = append([]CommandReflection{}, m.ReflectionLog[len(m.ReflectionLog)-reflectionLogLimit:]...)
	logger.Info("Reflection log trimmed, removed %d older entries", dropped)
}

func (m *Manager) runReflection(ctx context.Context, task ReflectionTask) (CommandReflection, error) {
	manifestPath := m.GetToolsManifestPath()
	payload := reflectionRequest{
		Command:           task.History.Command,
		ExitCode:          task.History.Code,
		Output:            truncateForReflection(task.History.Output, 4000),
		ToolsManifestPath: manifestPath,
	}

	systemPrompt := `You are an autonomous CLI specialist. Analyze the provided command execution, derive improvements, and respond with strict JSON matching this schema:
{
  "lessons": "string",
  "alternative": {
    "command": "string",
    "reason": "string"
  },
  "tools": [
    {
      "action": "add" | "remove" | "update" | "skip",
      "name": "string",
      "section": "string",
      "description": "string",
      "reason": "string"
    }
  ]
}
Return only JSON with no code fences.`

	userBytes, err := json.Marshal(payload)
	if err != nil {
		return CommandReflection{}, err
	}

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(userBytes)},
	}

	model := m.GetOpenRouterModel()
	resp, err := m.AiClient.ChatCompletion(ctx, messages, model)
	if err != nil {
		return CommandReflection{}, err
	}

	resp = sanitizeJSONResponse(resp)
	var parsed reflectionResponse
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return CommandReflection{}, fmt.Errorf("failed to parse reflection JSON: %w", err)
	}

	reflection := CommandReflection{
		Command:              task.History.Command,
		Output:               task.History.Output,
		ExitCode:             task.History.Code,
		LessonsLearned:       strings.TrimSpace(parsed.Lessons),
		Alternative:          strings.TrimSpace(parsed.Alternative.Command),
		AlternativeRationale: strings.TrimSpace(parsed.Alternative.Reason),
		Timestamp:            time.Now(),
	}

	for _, tool := range parsed.Tools {
		reflection.SuggestedActions = append(reflection.SuggestedActions, SuggestedToolAction{
			Action:      strings.ToLower(strings.TrimSpace(tool.Action)),
			Name:        strings.TrimSpace(tool.Name),
			Section:     strings.TrimSpace(tool.Section),
			Description: strings.TrimSpace(tool.Description),
			Reason:      strings.TrimSpace(tool.Reason),
		})
	}

	return reflection, nil
}

func (m *Manager) applyToolActions(actions []SuggestedToolAction) []SuggestedToolAction {
	path := m.GetToolsManifestPath()
	if path == "" {
		for i := range actions {
			actions[i].Outcome = "skipped: manifest path not configured"
		}
		return actions
	}

	for i := range actions {
		action := strings.ToLower(actions[i].Action)
		switch action {
		case "add", "update":
			if actions[i].Section == "" {
				actions[i].Outcome = "skipped: section required"
				continue
			}
			change, modified, err := AddToolToManifest(path, actions[i].Section, actions[i].Name, actions[i].Description)
			if err != nil {
				actions[i].Outcome = fmt.Sprintf("error: %v", err)
				continue
			}
			if modified {
				actions[i].Outcome = change.Action
			} else {
				actions[i].Outcome = "unchanged"
			}
		case "remove":
			if actions[i].Section == "" {
				actions[i].Outcome = "skipped: section required"
				continue
			}
			_, modified, err := RemoveToolFromManifest(path, actions[i].Section, actions[i].Name)
			if err != nil {
				actions[i].Outcome = fmt.Sprintf("error: %v", err)
				continue
			}
			if modified {
				actions[i].Outcome = "removed"
			} else {
				actions[i].Outcome = "not-found"
			}
		default:
			actions[i].Outcome = "skipped: no-op"
		}
	}

	return actions
}

func formatReflectionSummary(reflection CommandReflection) string {
	var builder strings.Builder
	builder.WriteString("Reflection Summary\n")
	builder.WriteString(fmt.Sprintf("Command: %s\n", reflection.Command))
	builder.WriteString(fmt.Sprintf("Exit Code: %d\n", reflection.ExitCode))
	if reflection.LessonsLearned != "" {
		builder.WriteString(fmt.Sprintf("Lessons Learned: %s\n", reflection.LessonsLearned))
	}
	if reflection.Alternative != "" {
		builder.WriteString(fmt.Sprintf("Proposed Alternative: %s\n", reflection.Alternative))
		if reflection.AlternativeRationale != "" {
			builder.WriteString(fmt.Sprintf("Rationale: %s\n", reflection.AlternativeRationale))
		}
	}
	if len(reflection.SuggestedActions) > 0 {
		builder.WriteString("Tool Actions:\n")
		for _, action := range reflection.SuggestedActions {
			builder.WriteString(fmt.Sprintf("- %s %s in section %s (%s): %s\n", strings.Title(action.Action), action.Name, action.Section, action.Outcome, action.Reason))
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func truncateForReflection(content string, max int) string {
	runes := []rune(content)
	if len(runes) <= max {
		return content
	}
	return string(runes[:max]) + "\n[truncated]"
}

func sanitizeJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		resp = strings.TrimPrefix(resp, "```json")
		resp = strings.TrimPrefix(resp, "```")
		resp = strings.TrimSuffix(resp, "```")
	}
	return strings.TrimSpace(resp)
}

type reflectionRequest struct {
	Command           string `json:"command"`
	ExitCode          int    `json:"exit_code"`
	Output            string `json:"output"`
	ToolsManifestPath string `json:"tools_manifest_path"`
}

type reflectionResponse struct {
	Lessons     string               `json:"lessons"`
	Alternative reflectionAlt        `json:"alternative"`
	Tools       []reflectionToolItem `json:"tools"`
}

type reflectionAlt struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type reflectionToolItem struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Section     string `json:"section"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
}

// selectPersona selects the appropriate persona based on rules or defaults
func (m *Manager) selectPersona() string {
	currentWindow, err := system.TmuxWindowName()
	if err != nil {
		logger.Error("Failed to get tmux window name: %v", err)
		return m.Config.DefaultPersona
	}

	// Check persona rules in order
	for _, rule := range m.Config.PersonaRules {
		matched, err := regexp.MatchString(rule.Match, currentWindow)
		if err != nil {
			logger.Error("Invalid regex pattern %q: %v", rule.Match, err)
			continue
		}
		if matched {
			return rule.Persona
		}
	}

	// Fallback to default persona if no matches
	return m.Config.DefaultPersona
}

// Start starts the manager agent
func (m *Manager) Start(initMessage string) error {
	cliInterface := NewCLIInterface(m)
	if initMessage != "" {
		logger.Info("Initial task provided: %s", initMessage)
	}
	if err := cliInterface.Start(initMessage); err != nil {
		logger.Error("Failed to start CLI interface: %v", err)
		return err
	}

	return nil
}

func (m *Manager) Println(msg string) {
	fmt.Println(m.GetPrompt() + msg)
}

func (m *Manager) GetConfig() *config.Config {
	return m.Config
}

// getPrompt returns the prompt string with color
func (m *Manager) GetPrompt() string {
	tmuxaiColor := color.New(color.FgGreen, color.Bold)
	arrowColor := color.New(color.FgYellow, color.Bold)
	stateColor := color.New(color.FgMagenta, color.Bold)

	var stateSymbol string
	switch m.Status {
	case "running":
		stateSymbol = "▶"
	case "waiting":
		stateSymbol = "?"
	case "done":
		stateSymbol = "✓"
	default:
		stateSymbol = ""
	}
	if m.WatchMode {
		stateSymbol = "∞"
	}

	prompt := tmuxaiColor.Sprint("TmuxAI")
	if stateSymbol != "" {
		prompt += " " + stateColor.Sprint("["+stateSymbol+"]")
	}
	prompt += arrowColor.Sprint(" » ")
	return prompt
}

func (ai *AIResponse) String() string {
	return fmt.Sprintf(`
	Message: %s
	SendKeys: %v
	ExecCommand: %v
	PasteMultilineContent: %s
	RequestAccomplished: %v
	ExecPaneSeemsBusy: %v
	WaitingForUserResponse: %v
	NoComment: %v
`,
		ai.Message,
		ai.SendKeys,
		ai.ExecCommand,
		ai.PasteMultilineContent,
		ai.RequestAccomplished,
		ai.ExecPaneSeemsBusy,
		ai.WaitingForUserResponse,
		ai.NoComment,
	)
}
