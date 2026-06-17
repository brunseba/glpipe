package cmd

import (
	"fmt"
	"os"

	"github.com/sebrun/glpipe/internal/config"
	"github.com/sebrun/glpipe/internal/gitlab"
	"github.com/sebrun/glpipe/internal/ui"
	"github.com/spf13/cobra"
	gl "gitlab.com/gitlab-org/api/client-go"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the latest pipeline for every configured project",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}

		t := ui.NewTable(os.Stdout, []string{"Project", "Pipeline", "Branch", "Status", "Jobs", "Duration", "Created"})

		for _, p := range cfg.Projects {
			row := projectStatusRow(client, p)
			_ = t.Append(row)
		}

		return t.Render()
	},
}

func projectStatusRow(client *gl.Client, p config.Project) []string {
	label := p.Alias
	if label == "" {
		label = p.ID
	}

	pipelines, _, err := client.Pipelines.ListProjectPipelines(p.ID, &gl.ListProjectPipelinesOptions{
		ListOptions: gl.ListOptions{PerPage: 1},
	})
	if err != nil || len(pipelines) == 0 {
		errMsg := "no pipelines"
		if err != nil {
			errMsg = err.Error()
		}
		return []string{label, "-", "-", errMsg, "-", "-", "-"}
	}

	pipe := pipelines[0]

	// count jobs by status
	jobs, _, _ := client.Jobs.ListPipelineJobs(p.ID, pipe.ID, &gl.ListJobsOptions{})
	jobSummary := summarizeJobs(jobs)

	return []string{
		label,
		fmt.Sprintf("%d", pipe.ID),
		pipe.Ref,
		ui.ColorStatus(pipe.Status),
		jobSummary,
		ui.FormatDuration(pipe.CreatedAt, pipe.UpdatedAt),
		pipe.CreatedAt.Format("2006-01-02 15:04"),
	}
}

func summarizeJobs(jobs []*gl.Job) string {
	counts := map[string]int{}
	for _, j := range jobs {
		counts[j.Status]++
	}
	if len(counts) == 0 {
		return "-"
	}
	order := []string{"running", "pending", "manual", "failed", "success", "canceled", "skipped"}
	out := ""
	for _, s := range order {
		if n := counts[s]; n > 0 {
			out += fmt.Sprintf("%s:%d ", ui.ColorStatus(s), n)
		}
	}
	return out
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
