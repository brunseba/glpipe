package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Project struct {
	ID            string `mapstructure:"id"`
	Alias         string `mapstructure:"alias"`
	DefaultBranch string `mapstructure:"default_branch"`
}

type GitLab struct {
	URL   string `mapstructure:"url"`
	Token string `mapstructure:"token"`
}

type Group struct {
	Name     string   `mapstructure:"name"`
	Projects []string `mapstructure:"projects"` // list of project aliases or IDs
}

type Config struct {
	GitLab   GitLab    `mapstructure:"gitlab"`
	Projects []Project `mapstructure:"projects"`
	Groups   []Group   `mapstructure:"groups"`
}

// DefaultDir returns ~/.glpipe
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".glpipe")
}

// DefaultPath returns ~/.glpipe/config.yaml
func DefaultPath() string {
	return filepath.Join(DefaultDir(), "config.yaml")
}

// EnsureDir creates ~/.glpipe with permissions 0700 if it does not exist.
func EnsureDir() error {
	dir := DefaultDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir %s: %w", dir, err)
	}
	return nil
}

// Load reads config from file then overrides with env vars:
//
//	GITLAB_URL, GITLAB_TOKEN
func Load(cfgFile string) (*Config, error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(DefaultDir())
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetDefault("gitlab.url", "https://gitlab.com")

	// env overrides
	viper.SetEnvPrefix("")
	_ = viper.BindEnv("gitlab.url", "GITLAB_URL")
	_ = viper.BindEnv("gitlab.token", "GITLAB_TOKEN")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// FindProject returns a project by alias or ID.
func (c *Config) FindProject(ref string) (*Project, error) {
	for i := range c.Projects {
		p := &c.Projects[i]
		if p.Alias == ref || p.ID == ref {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project %q not found in config", ref)
}

// FindGroup returns the projects belonging to a named group.
func (c *Config) FindGroup(name string) ([]Project, error) {
	for _, g := range c.Groups {
		if g.Name != name {
			continue
		}
		projects := make([]Project, 0, len(g.Projects))
		for _, ref := range g.Projects {
			p, err := c.FindProject(ref)
			if err != nil {
				return nil, fmt.Errorf("group %q: %w", name, err)
			}
			projects = append(projects, *p)
		}
		return projects, nil
	}
	return nil, fmt.Errorf("group %q not found in config", name)
}

// ResolveProjectsOrGroup returns the project list for --project or --group.
// Exactly one of project/group must be non-empty, or neither (returns all projects).
func (c *Config) ResolveTargets(project, group string) ([]Project, error) {
	switch {
	case project != "" && group != "":
		return nil, fmt.Errorf("--project and --group are mutually exclusive")
	case project != "":
		p, err := c.FindProject(project)
		if err != nil {
			return nil, err
		}
		return []Project{*p}, nil
	case group != "":
		return c.FindGroup(group)
	default:
		return c.Projects, nil
	}
}
