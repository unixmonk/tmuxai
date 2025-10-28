package cli

import (
	"errors"
	"fmt"

	"github.com/alvinunreal/tmuxai/config"
	"github.com/alvinunreal/tmuxai/internal"
	"github.com/alvinunreal/tmuxai/logger"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newToolsCmd())
}

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "tools",
		Short:  "Manage the tools manifest",
		Long:   "Add, remove, or list entries in the tools manifest used by personas.",
		Hidden: true,
	}

	cmd.AddCommand(newToolsAddCmd())
	cmd.AddCommand(newToolsRemoveCmd())
	cmd.AddCommand(newToolsListCmd())

	return cmd
}

func loadConfigForTools() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.ToolsManifestPath == "" {
		return nil, errors.New("tools manifest path is not configured")
	}
	return cfg, nil
}

func newToolsAddCmd() *cobra.Command {
	var section string
	var description string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a tool entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if section == "" {
				return errors.New("--section is required")
			}

			cfg, err := loadConfigForTools()
			if err != nil {
				return err
			}

			change, modified, err := internal.AddToolToManifest(cfg.ToolsManifestPath, section, name, description)
			if err != nil {
				return err
			}

			if modified {
				logger.Info("Tool %s %s in section %s", change.Name, change.Action, change.Section)
				fmt.Fprintf(cmd.OutOrStdout(), "Tool %s %s in section %s\n", change.Name, change.Action, change.Section)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Tool %s already up to date in section %s\n", name, section)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&section, "section", "", "Section heading to place the tool under")
	cmd.Flags().StringVar(&description, "description", "", "Optional description for the tool")

	return cmd
}

func newToolsRemoveCmd() *cobra.Command {
	var section string

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a tool entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if section == "" {
				return errors.New("--section is required")
			}

			cfg, err := loadConfigForTools()
			if err != nil {
				return err
			}

			change, modified, err := internal.RemoveToolFromManifest(cfg.ToolsManifestPath, section, name)
			if err != nil {
				return err
			}

			if modified {
				logger.Info("Tool %s removed from section %s", change.Name, change.Section)
				fmt.Fprintf(cmd.OutOrStdout(), "Tool %s removed from section %s\n", change.Name, change.Section)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Tool %s not found in section %s\n", name, section)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&section, "section", "", "Section heading to remove the tool from")

	return cmd
}

func newToolsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print the current tools manifest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigForTools()
			if err != nil {
				return err
			}

			content, err := internal.ListToolsInManifest(cfg.ToolsManifestPath)
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), content)
			return nil
		},
	}

	return cmd
}
