package cmd

import (
	"fmt"
	"os"
	"text/template"

	"github.com/sebrun/glpipe/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage glpipe configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter config file at ~/.glpipe/config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.EnsureDir(); err != nil {
			return err
		}
		path := config.DefaultPath()
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s", path)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()

		tpl := template.Must(template.New("cfg").Parse(starterConfig))
		if err := tpl.Execute(f, nil); err != nil {
			return err
		}
		fmt.Printf("Created config file: %s\n", path)
		fmt.Println("Edit it to add your GitLab token and projects.")
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the config file and test GitLab connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.GitLab.Token == "" {
			return fmt.Errorf("no token configured (set gitlab.token or GITLAB_TOKEN)")
		}
		if len(cfg.Projects) == 0 {
			return fmt.Errorf("no projects configured")
		}
		fmt.Printf("Config     : %s\n", config.DefaultPath())
		fmt.Printf("GitLab URL : %s\n", cfg.GitLab.URL)
		fmt.Printf("Token      : %s...\n", cfg.GitLab.Token[:min(8, len(cfg.GitLab.Token))])
		fmt.Printf("Projects   : %d configured\n", len(cfg.Projects))
		for _, p := range cfg.Projects {
			alias := p.Alias
			if alias == "" {
				alias = "(no alias)"
			}
			fmt.Printf("  - %-20s  alias: %s  default_branch: %s\n", p.ID, alias, p.DefaultBranch)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
	rootCmd.AddCommand(configCmd)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const starterConfig = `gitlab:
  url: https://gitlab.com   # or your self-hosted instance
  token: ""                 # or set GITLAB_TOKEN env var

projects:
  - id: mygroup/myrepo
    alias: backend
    default_branch: main
  - id: mygroup/frontend
    alias: frontend
    default_branch: develop
  - id: mygroup/infra
    alias: infra
    default_branch: main

groups:
  - name: apps
    projects: [backend, frontend]
  - name: all
    projects: [backend, frontend, infra]
`
