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

type Config struct {
	GitLab   GitLab    `mapstructure:"gitlab"`
	Projects []Project `mapstructure:"projects"`
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

// FindProject returns a project by alias or by ID (partial match).
func (c *Config) FindProject(ref string) (*Project, error) {
	for i := range c.Projects {
		p := &c.Projects[i]
		if p.Alias == ref || p.ID == ref {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project %q not found in config", ref)
}
