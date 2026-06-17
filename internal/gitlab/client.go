package gitlab

import (
	"fmt"

	"github.com/sebrun/glpipe/internal/config"
	gl "gitlab.com/gitlab-org/api/client-go"
)

func NewClient(cfg *config.Config) (*gl.Client, error) {
	if cfg.GitLab.Token == "" {
		return nil, fmt.Errorf("GitLab token is required (set gitlab.token in config or GITLAB_TOKEN env var)")
	}
	client, err := gl.NewClient(cfg.GitLab.Token, gl.WithBaseURL(cfg.GitLab.URL))
	if err != nil {
		return nil, fmt.Errorf("creating GitLab client: %w", err)
	}
	return client, nil
}
