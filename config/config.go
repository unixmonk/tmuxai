package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Debug                 bool                  `mapstructure:"debug"`
	MaxCaptureLines       int                   `mapstructure:"max_capture_lines"`
	MaxContextSize        int                   `mapstructure:"max_context_size"`
	WaitInterval          int                   `mapstructure:"wait_interval"`
	SendKeysConfirm       bool                  `mapstructure:"send_keys_confirm"`
	PasteMultilineConfirm bool                  `mapstructure:"paste_multiline_confirm"`
	ExecConfirm           bool                  `mapstructure:"exec_confirm"`
	WhitelistPatterns     []string              `mapstructure:"whitelist_patterns"`
	BlacklistPatterns     []string              `mapstructure:"blacklist_patterns"`
	OpenRouter            OpenRouterConfig      `mapstructure:"openrouter"`
	OpenAI                OpenAIConfig          `mapstructure:"openai"`
	AzureOpenAI           AzureOpenAIConfig     `mapstructure:"azure_openai"`
	DefaultModel          string                 `mapstructure:"default_model"`
	Models                map[string]ModelConfig  `mapstructure:"models"`
	Prompts               PromptsConfig         `mapstructure:"prompts"`
	Personas              map[string]*Persona   `mapstructure:"personas"`
	PersonaRules          []PersonaRule         `mapstructure:"persona_rules"`
	DefaultPersona        string                `mapstructure:"default_persona"`
	ToolsManifestPath     string                `mapstructure:"tools_manifest_path"`
	KnowledgeBase         KnowledgeBaseConfig   `mapstructure:"knowledge_base"`
}

// OpenRouterConfig holds OpenRouter API configuration
type OpenRouterConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

// OpenAIConfig holds OpenAI API configuration
type OpenAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

// AzureOpenAIConfig holds Azure OpenAI API configuration
type AzureOpenAIConfig struct {
	APIKey         string `mapstructure:"api_key"`
	APIBase        string `mapstructure:"api_base"`
	APIVersion     string `mapstructure:"api_version"`
	DeploymentName string `mapstructure:"deployment_name"`
}


// ModelConfig holds a single model configuration
type ModelConfig struct {
	Provider string `mapstructure:"provider"`
	Model   string `mapstructure:"model"`
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`

	// Azure-specific fields
	APIBase        string `mapstructure:"api_base"`
	APIVersion     string `mapstructure:"api_version"`
	DeploymentName string `mapstructure:"deployment_name"`
}

// PromptsConfig holds customizable prompt templates
type PromptsConfig struct {
	BaseSystem            string `mapstructure:"base_system"`
	ChatAssistant         string `mapstructure:"chat_assistant"`
	ChatAssistantPrepared string `mapstructure:"chat_assistant_prepared"`
	Watch                 string `mapstructure:"watch"`
}

// Persona represents a single persona configuration
type Persona struct {
	Prompt         string                 `yaml:"prompt"`
	Description    string                 `yaml:"description"`
	ToolsAvailable *PersonaToolsAvailable `yaml:"tools_available"`
}

// PersonaToolsAvailable describes an optional tools file to include in the persona prompt
type PersonaToolsAvailable struct {
	File string `yaml:"file"`
}

// PersonaRule defines rules for auto-selecting personas
type PersonaRule struct {
	Match   string `mapstructure:"match"`
	Persona string `mapstructure:"persona"`
}

// KnowledgeBaseConfig holds knowledge base configuration
type KnowledgeBaseConfig struct {
	AutoLoad []string `mapstructure:"auto_load"`
	Path     string   `mapstructure:"path"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	defaultPersonas := map[string]*Persona{
		"command_line_specialist": {
			Prompt: `You are TmuxAI assistant. You are an AI command-line specialist living inside the user's tmux window and can see every pane in that window.

Think of yourself as the user's command-line expert who sits beside them, watching the terminal exactly as they see it and taking action when it moves the work forward. You operate primarily in a fish shell environment—understand and respect fish conventions such as universal variables, concise function definitions, rich completions, and the default prompt. Detect when the active shell differs and adapt automatically, but default to fish-centric solutions when uncertain.

Your mandate:
- Observe pane content continuously, reason about state, and act through ExecCommand or TmuxSendKeys when it delivers progress.
- Prefer native shell pipelines, POSIX utilities, and fish-specific idioms over higher-level scripting unless unavoidable.
- Keep command sequences short, composable, and well-explained. When a task is larger than a single command, break it into safe, observable steps.
- Never assume availability of non-standard binaries unless declared in the tools manifest.

Collaboration guidelines:
- Use common sense before asking questions; infer intent whenever possible.
- Always address the user as "you" and keep responses crisp.
- Highlight the reasoning behind command choices when ambiguity exists.
- Address root causes instead of symptoms.

Safety and tooling:
- Before every tool call, state why it is required and follow the tool schema exactly.
- Never emit binary blobs, extremely long hashes, or meaningless output.
- After issuing ExecCommand or TmuxSendKeys, wait for observable confirmation before chaining risky actions.
- Respect the tmux exec pane; both you and the user can interact with it at any time.

Discipline:
- Be concise—verbosity wastes tokens and hides signal.
- Default to actionable steps. If you must pause for input, be explicit.
- Never append additional narration after tool tags.`,
			Description: "Specialist focused on terminal workflows in fish shell with emphasis on concise command-line execution.",
		},
	}

	return &Config{
		Debug:                 false,
		MaxCaptureLines:       200,
		MaxContextSize:        100000,
		WaitInterval:          5,
		SendKeysConfirm:       true,
		PasteMultilineConfirm: true,
		ExecConfirm:           true,
		WhitelistPatterns:     []string{},
		BlacklistPatterns:     []string{},
		OpenRouter: OpenRouterConfig{
			BaseURL: "https://openrouter.ai/api/v1",
			Model:   "google/gemini-2.5-flash-preview",
		},
		OpenAI: OpenAIConfig{
			BaseURL: "https://api.openai.com/v1",
		},
		AzureOpenAI: AzureOpenAIConfig{},
		DefaultModel: "",
	Models:       make(map[string]ModelConfig),
		Prompts: PromptsConfig{
			BaseSystem:    ``,
			ChatAssistant: ``,
		},
		Personas:          defaultPersonas,
		DefaultPersona:    "command_line_specialist",
		PersonaRules:      []PersonaRule{},
		ToolsManifestPath: "tools-available.md",
		KnowledgeBase: KnowledgeBaseConfig{
			AutoLoad: []string{},
			Path:     "",
		},
	}
}

// Load loads the configuration from file or environment variables
func Load() (*Config, error) {
	config := DefaultConfig()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	viper.AddConfigPath(".")

	configDir, err := GetConfigDir()
	if err == nil {
		viper.AddConfigPath(configDir)
	} else {
		viper.AddConfigPath(filepath.Join(homeDir, ".config", "tmuxai"))
	}

	// Environment variables
	viper.SetEnvPrefix("TMUXAI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Automatically bind all config keys to environment variables
	configType := reflect.TypeOf(*config)
	for _, key := range EnumerateConfigKeys(configType, "") {
		_ = viper.BindEnv(key)
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	ResolveEnvKeyInConfig(config)

	// Load personas from directory
	configDir, _ = GetConfigDir()
	personasDir := filepath.Join(configDir, "personas")
	if err := LoadPersonasFromDir(&config.Personas, personasDir); err != nil {
		fmt.Printf("Warning: Failed to load personas from directory: %v\n", err)
	}
	populatePersonasWithTools(config.Personas, personasDir, configDir)

	if config.ToolsManifestPath == "" {
		config.ToolsManifestPath = filepath.Join(configDir, "tools-available.md")
	} else if !filepath.IsAbs(config.ToolsManifestPath) {
		config.ToolsManifestPath = filepath.Join(configDir, config.ToolsManifestPath)
	}

	if err := ensureToolsManifestExists(config.ToolsManifestPath); err != nil {
		fmt.Printf("Warning: Failed to ensure tools manifest exists: %v\n", err)
	}

	return config, nil
}

// EnumerateConfigKeys returns all config keys (dot notation) for the given struct type.
func EnumerateConfigKeys(cfgType reflect.Type, prefix string) []string {
	var keys []string
	for i := 0; i < cfgType.NumField(); i++ {
		field := cfgType.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}
		key := tag
		if prefix != "" {
			key = prefix + "." + tag
		}
		if field.Type.Kind() == reflect.Struct {
			keys = append(keys, EnumerateConfigKeys(field.Type, key)...)
		} else {
			keys = append(keys, key)
		}
	}
	return keys
}

// GetConfigDir returns the path to the tmuxai config directory (~/.config/tmuxai)
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "tmuxai")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create personas subdirectory
	personasDir := filepath.Join(configDir, "personas")
	if err := os.MkdirAll(personasDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create personas directory: %w", err)
	}

	return configDir, nil
}

// LoadPersonasFromDir loads persona files from the specified directory
func LoadPersonasFromDir(personas *map[string]*Persona, dir string) error {
	if personas == nil {
		return fmt.Errorf("personas map pointer is nil")
	}
	if *personas == nil {
		*personas = make(map[string]*Persona)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Dir doesn't exist yet, no error
		}
		return fmt.Errorf("failed to read personas directory: %w", err)
	}

	loaded := 0
	for _, file := range files {
		if file.IsDir() || (!strings.HasSuffix(file.Name(), ".yaml") && !strings.HasSuffix(file.Name(), ".yml")) {
			continue
		}

		filename := strings.TrimSuffix(file.Name(), ".yaml")
		if strings.HasSuffix(file.Name(), ".yml") {
			filename = strings.TrimSuffix(filename, ".yml")
		}

		if len(*personas) >= 50 { // Limit to prevent overload
			fmt.Printf("Warning: Skipping %s - max 50 personas loaded\n", file.Name())
			continue
		}

		filePath := filepath.Join(dir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Warning: Failed to read %s: %v\n", file.Name(), err)
			continue
		}

		var persona Persona
		if err := yaml.Unmarshal(data, &persona); err != nil {
			fmt.Printf("Warning: Failed to parse %s: %v\n", file.Name(), err)
			continue
		}

		if persona.Prompt == "" {
			fmt.Printf("Warning: %s has no prompt, skipping\n", file.Name())
			continue
		}

		loadPersonaTools(&persona, dir)
		(*personas)[filename] = &persona
		loaded++
	}

	if loaded > 0 {
		fmt.Printf("Loaded %d personas from %s\n", loaded, dir)
	}

	return nil
}

func loadPersonaTools(persona *Persona, baseDirs ...string) {
	if persona == nil || persona.ToolsAvailable == nil || persona.ToolsAvailable.File == "" {
		return
	}

	pathsTried := make(map[string]struct{})
	for _, baseDir := range baseDirs {
		path := persona.ToolsAvailable.File
		if baseDir != "" && !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		if _, seen := pathsTried[path]; seen {
			continue
		}
		pathsTried[path] = struct{}{}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		persona.Prompt = strings.TrimSpace(persona.Prompt + "\n\n" + string(data))
		return
	}

	if filepath.IsAbs(persona.ToolsAvailable.File) {
		if _, seen := pathsTried[persona.ToolsAvailable.File]; !seen {
			data, err := os.ReadFile(persona.ToolsAvailable.File)
			if err == nil {
				persona.Prompt = strings.TrimSpace(persona.Prompt + "\n\n" + string(data))
				return
			}
		}
	}

	fmt.Printf("Warning: Failed to load tools file %s\n", persona.ToolsAvailable.File)
}

func populatePersonasWithTools(personas map[string]*Persona, baseDirs ...string) {
	for _, persona := range personas {
		loadPersonaTools(persona, baseDirs...)
	}
}

func ensureToolsManifestExists(path string) error {
	if path == "" {
		return nil
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	defaultContent := "# Tools Available for TmuxAI Personas\n\n" +
		"The following non-standard tools are installed and may be used when appropriate. When a tool is unavailable, fall back to core POSIX utilities.\n\n" +
		"## Search & Navigation\n\n" +
		"- `rg` *(ripgrep)* – project-wide regex search respecting `.gitignore`.\n" +
		"- `fd` – intuitive alternative to `find`; supports globbing and excludes.\n\n" +
		"## Editing & Formatting\n\n" +
		"- `nvim` – modal editor; automation via `nvim --headless` and `:lua` scripts.\n\n" +
		"## Data Processing & Scripting\n\n" +
		"- `jq` – JSON query and transformation.\n" +
		"- `xh` – http client (curl alternative) with JSON processing.\n\n" +
		"> Keep commands concise. Confirm tool availability with `command -v <name>`.\n"

	return os.WriteFile(path, []byte(defaultContent), 0o644)
}

func GetConfigFilePath(filename string) string {
	configDir, _ := GetConfigDir()
	return filepath.Join(configDir, filename)
}

// GetKBDir returns the path to the knowledge base directory
func GetKBDir() string {
	// Try to load config to check for custom path
	cfg, err := Load()
	if err == nil && cfg.KnowledgeBase.Path != "" {
		// Use custom path if specified
		return cfg.KnowledgeBase.Path
	}

	// Default to ~/.config/tmuxai/kb/
	configDir, _ := GetConfigDir()
	kbDir := filepath.Join(configDir, "kb")

	// Create KB directory if it doesn't exist
	_ = os.MkdirAll(kbDir, 0o755)

	return kbDir
}

func TryInferType(key, value string) any {
	if key == "" {
		return value
	}

	parts := strings.Split(key, ".")
	if converted, ok := inferConfigFieldType(reflect.TypeOf(Config{}), parts, value); ok {
		return converted
	}

	return value
}

func inferConfigFieldType(t reflect.Type, parts []string, value string) (any, bool) {
	if len(parts) == 0 {
		return value, false
	}

	part := parts[0]

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}
		if tag != part {
			continue
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		if len(parts) > 1 {
			if fieldType.Kind() == reflect.Struct {
				return inferConfigFieldType(fieldType, parts[1:], value)
			}
			return value, false
		}

		switch fieldType.Kind() {
		case reflect.Bool:
			switch strings.ToLower(value) {
			case "true", "1", "yes", "on":
				return true, true
			case "false", "0", "no", "off":
				return false, true
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if parsed, err := strconv.ParseInt(value, 10, int(fieldType.Bits())); err == nil {
				switch fieldType.Kind() {
				case reflect.Int:
					return int(parsed), true
				case reflect.Int8:
					return int8(parsed), true
				case reflect.Int16:
					return int16(parsed), true
				case reflect.Int32:
					return int32(parsed), true
				case reflect.Int64:
					return int64(parsed), true
				}
			}
		}

		return value, false
	}

	return value, false
}

// ResolveEnvKeyInConfig recursively expands environment variables in all string fields of the config struct.
func ResolveEnvKeyInConfig(cfg *Config) {
	val := reflect.ValueOf(cfg).Elem()
	resolveEnvKeyReferenceInValue(val)
}

func resolveEnvKeyReferenceInValue(val reflect.Value) {
	switch val.Kind() {
	case reflect.String:
		val.SetString(os.ExpandEnv(val.String()))
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			resolveEnvKeyReferenceInValue(val.Field(i))
		}
	case reflect.Ptr:
		if !val.IsNil() {
			resolveEnvKeyReferenceInValue(val.Elem())
		}
	}
}
