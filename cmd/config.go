package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/fatih/color"
	"github.com/sebrun/glpipe/internal/config"
	"github.com/sebrun/glpipe/internal/gitlab"
	"github.com/spf13/cobra"
	gl "gitlab.com/gitlab-org/api/client-go"
	"go.yaml.in/yaml/v3"
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

// ── sync-group ────────────────────────────────────────────────────────────────

var (
	syncGroupCreateGroup bool
	syncGroupGroupName   string
	syncGroupDryRun      bool
)

var configSyncGroupCmd = &cobra.Command{
	Use:   "sync-group <gitlab-group-path>",
	Short: "Fetch projects from a GitLab group and add them to config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		groupPath := args[0]

		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}

		// Fetch all projects in the GitLab group (paginate).
		var glProjects []*gl.Project
		listOpts := &gl.ListGroupProjectsOptions{
			ListOptions:      gl.ListOptions{PerPage: 100},
			IncludeSubGroups: gl.Ptr(true),
		}
		for {
			page, resp, err := client.Groups.ListGroupProjects(groupPath, listOpts)
			if err != nil {
				return fmt.Errorf("fetching group %q: %w", groupPath, err)
			}
			glProjects = append(glProjects, page...)
			if resp.NextPage == 0 {
				break
			}
			listOpts.Page = resp.NextPage
		}

		if len(glProjects) == 0 {
			return fmt.Errorf("no projects found in group %q", groupPath)
		}

		// Build set of existing project IDs to skip duplicates.
		existing := map[string]bool{}
		for _, p := range cfg.Projects {
			existing[strings.ToLower(p.ID)] = true
		}

		var added []config.Project
		for _, gp := range glProjects {
			id := gp.PathWithNamespace
			if existing[strings.ToLower(id)] {
				color.Yellow("  skip  %s (already in config)", id)
				continue
			}
			p := config.Project{
				ID:            id,
				Alias:         gp.Path,
				DefaultBranch: gp.DefaultBranch,
			}
			if p.DefaultBranch == "" {
				p.DefaultBranch = "main"
			}
			added = append(added, p)
			color.Green("  + %s  alias: %s  branch: %s", p.ID, p.Alias, p.DefaultBranch)
		}

		if len(added) == 0 {
			fmt.Println("Nothing to add — all projects already in config.")
			return nil
		}

		if syncGroupDryRun {
			fmt.Printf("\n(dry-run) would add %d project(s)\n", len(added))
			return nil
		}

		// Build the updated config.
		newCfg := *cfg
		newCfg.Projects = append(newCfg.Projects, added...)

		// Optionally add a config group entry.
		if syncGroupCreateGroup {
			name := syncGroupGroupName
			if name == "" {
				name = groupPath[strings.LastIndex(groupPath, "/")+1:]
			}
			aliases := make([]string, 0, len(added))
			for _, p := range added {
				aliases = append(aliases, p.Alias)
			}
			// Merge into existing group if name already exists.
			found := false
			for i, g := range newCfg.Groups {
				if g.Name == name {
					newCfg.Groups[i].Projects = append(newCfg.Groups[i].Projects, aliases...)
					found = true
					break
				}
			}
			if !found {
				newCfg.Groups = append(newCfg.Groups, config.Group{
					Name:     name,
					Projects: aliases,
				})
			}
			color.Green("  + group %q with %d project(s)", name, len(aliases))
		}

		// Write back to config file.
		path := config.DefaultPath()
		data, err := yaml.Marshal(&newCfg)
		if err != nil {
			return fmt.Errorf("marshalling config: %w", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}

		fmt.Printf("\n%d project(s) added to %s\n", len(added), path)
		return nil
	},
}

func init() {
	configSyncGroupCmd.Flags().BoolVar(&syncGroupCreateGroup, "create-group", false, "also create a config group entry for the synced projects")
	configSyncGroupCmd.Flags().StringVar(&syncGroupGroupName, "group-name", "", "name for the config group (default: last segment of gitlab group path)")
	configSyncGroupCmd.Flags().BoolVar(&syncGroupDryRun, "dry-run", false, "print what would be added without modifying the config")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configSyncGroupCmd)
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
