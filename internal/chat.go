package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/alvinunreal/tmuxai/config"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/completion"
	"github.com/nyaosorg/go-readline-ny/keys"
	"github.com/nyaosorg/go-readline-ny/simplehistory"
)

// Message represents a chat message
type ChatMessage struct {
	Content   string
	FromUser  bool
	Timestamp time.Time
}

type CLIInterface struct {
	manager     *Manager
	initMessage string
}

func NewCLIInterface(manager *Manager) *CLIInterface {
	return &CLIInterface{
		manager:     manager,
		initMessage: "",
	}
}

// Start starts the CLI interface
func (c *CLIInterface) Start(initMessage string) error {
	c.printWelcomeMessage()

	// Initialize history
	history := simplehistory.New()
	historyFilePath := config.GetConfigFilePath("history")

	// Load history from file if it exists
	if historyData, err := os.ReadFile(historyFilePath); err == nil {
		for _, line := range strings.Split(string(historyData), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				history.Add(line)
			}
		}
	}

	// Initialize editor
	editor := &readline.Editor{
		PromptWriter: func(w io.Writer) (int, error) {
			return io.WriteString(w, c.manager.GetPrompt())
		},
		History:        history,
		HistoryCycling: true,
	}

	// Bind TAB key to completion
	editor.BindKey(keys.CtrlI, c.newCompleter())

	if initMessage != "" {
		fmt.Printf("%s%s\n", c.manager.GetPrompt(), initMessage)
		c.processInput(initMessage)
	}

	ctx := context.Background()

	for {
		line, err := editor.ReadLine(ctx)

		if err == readline.CtrlC {
			// Ctrl+C pressed, clear the line and continue
			continue
		} else if err == io.EOF {
			// Ctrl+D pressed, exit
			return nil
		} else if err != nil {
			return err
		}

		// Save history
		if line != "" {
			history.Add(line)

			// Build history data by iterating through all entries
			historyLines := make([]string, 0, history.Len())
			for i := 0; i < history.Len(); i++ {
				historyLines = append(historyLines, history.At(i))
			}
			historyData := strings.Join(historyLines, "\n")
			_ = os.WriteFile(historyFilePath, []byte(historyData), 0644)
		}

		// Process the input (preserving multiline content)
		input := line // Keep the original line including newlines

		// Check for exit/quit commands (only if it's the entire line content)
		trimmed := strings.TrimSpace(input)
		if trimmed == "exit" || trimmed == "quit" {
			return nil
		}
		if trimmed == "" {
			continue
		}

		c.processInput(input)
	}
}

// printWelcomeMessage prints a welcome message
func (c *CLIInterface) printWelcomeMessage() {
	fmt.Println()
	fmt.Println("Type '/help' for a list of commands, '/exit' to quit")
	fmt.Println()
}

func (c *CLIInterface) processInput(input string) {
	if c.manager.IsMessageSubcommand(input) {
		c.manager.ProcessSubCommand(input)
		return
	}

	// Set up signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Use a WaitGroup to wait for the processing to complete
	var wg sync.WaitGroup
	wg.Add(1)

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Launch a goroutine for the message processing
	go func() {
		defer wg.Done()
		defer func() {
			c.manager.Status = ""
		}()

		c.manager.Status = "running"
		c.manager.ProcessUserMessage(ctx, input)
	}()

	// Wait for either the processing to finish or for an interrupt signal
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-sigChan:
		// Ctrl+C was pressed
		fmt.Println("\nReceived interrupt signal, canceling operation...")
		cancel() // Signal the goroutine to stop
		<-done   // Wait for the goroutine to finish cleanup
		fmt.Println("Operation canceled.")
	case <-done:
		// Processing completed normally
	}

	// Clean up signal handling
	signal.Stop(sigChan)
	close(sigChan)
}

// newCompleter creates a completion handler for command completion
func (c *CLIInterface) newCompleter() *completion.CmdCompletionOrList2 {
	cmds := append([]string(nil), commands...)
	return &completion.CmdCompletionOrList2{
		Delimiter: " ",
		Postfix:   " ",
		Candidates: func(field []string) (forComp []string, forList []string) {
			// Handle top-level commands
			if len(field) == 0 || (len(field) == 1 && !strings.HasSuffix(field[0], " ")) {
				return cmds, cmds
			}

			// Handle /config subcommands
			if len(field) > 0 && field[0] == "/config" {
				if len(field) == 1 || (len(field) == 2 && !strings.HasSuffix(field[1], " ")) {
					return []string{"set", "get"}, []string{"set", "get"}
				} else if len(field) == 2 || (len(field) == 3 && !strings.HasSuffix(field[2], " ")) {
					return AllowedConfigKeys, AllowedConfigKeys
				}
			}

			// Handle /prepare subcommands
			if len(field) > 0 && field[0] == "/prepare" {
				if len(field) == 1 || (len(field) == 2 && !strings.HasSuffix(field[1], " ")) {
					return []string{"bash", "zsh", "fish"}, []string{"bash", "zsh", "fish"}
				}
			}
			return nil, nil
		},
	}
}
