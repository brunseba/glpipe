package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/sebrun/glpipe/internal/gitlab"
	"github.com/sebrun/glpipe/internal/ui"
	"github.com/spf13/cobra"
	gl "gitlab.com/gitlab-org/api/client-go"
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage pipeline jobs",
}

// ── list ──────────────────────────────────────────────────────────────────────

var (
	jobListProject    string
	jobListPipelineID int64
	jobListScope      []string
)

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs of a pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		p, err := cfg.FindProject(jobListProject)
		if err != nil {
			return err
		}

		opts := &gl.ListJobsOptions{}
		if len(jobListScope) > 0 {
			scopes := make([]gl.BuildStateValue, 0, len(jobListScope))
			for _, s := range jobListScope {
				scopes = append(scopes, gl.BuildStateValue(s))
			}
			opts.Scope = &scopes
		}

		jobs, _, err := client.Jobs.ListPipelineJobs(p.ID, jobListPipelineID, opts)
		if err != nil {
			return fmt.Errorf("listing jobs: %w", err)
		}

		t := ui.NewTable(os.Stdout, []string{"ID", "Name", "Stage", "Status", "Duration", "Runner"})
		for _, j := range jobs {
			runner := "-"
			if j.Runner.ID != 0 {
				runner = j.Runner.Description
			}
			_ = t.Append([]string{
				fmt.Sprintf("%d", j.ID),
				j.Name,
				j.Stage,
				ui.ColorStatus(j.Status),
				ui.FormatDuration(j.StartedAt, j.FinishedAt),
				runner,
			})
		}
		return t.Render()
	},
}

// ── play (manual trigger) ─────────────────────────────────────────────────────

var (
	jobPlayProject string
	jobPlayJobID   int64
)

var jobPlayCmd = &cobra.Command{
	Use:   "play",
	Short: "Trigger a manual job",
	Long: `Play (trigger) a job that is in 'manual' state.
Use 'glpipe job list' to find the job ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		p, err := cfg.FindProject(jobPlayProject)
		if err != nil {
			return err
		}

		job, _, err := client.Jobs.PlayJob(p.ID, jobPlayJobID, nil)
		if err != nil {
			return fmt.Errorf("playing job %d: %w", jobPlayJobID, err)
		}

		color.Green("Job #%d (%s) triggered — status: %s", job.ID, job.Name, job.Status)
		return nil
	},
}

// ── retry ─────────────────────────────────────────────────────────────────────

var (
	jobRetryProject string
	jobRetryJobID   int64
)

var jobRetryCmd = &cobra.Command{
	Use:   "retry",
	Short: "Retry a failed or canceled job",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		p, err := cfg.FindProject(jobRetryProject)
		if err != nil {
			return err
		}

		job, _, err := client.Jobs.RetryJob(p.ID, jobRetryJobID)
		if err != nil {
			return fmt.Errorf("retrying job %d: %w", jobRetryJobID, err)
		}

		color.Yellow("Job #%d (%s) retried — new status: %s", job.ID, job.Name, job.Status)
		return nil
	},
}

// ── logs ──────────────────────────────────────────────────────────────────────

var (
	jobLogsProject string
	jobLogsJobID   int64
	jobLogsFollow  bool
)

var jobLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Print (or follow) the logs of a job",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		p, err := cfg.FindProject(jobLogsProject)
		if err != nil {
			return err
		}

		if !jobLogsFollow {
			return printLogs(client, p.ID, jobLogsJobID)
		}

		// follow mode: poll until job is no longer running
		var offset int
		for {
			job, _, err := client.Jobs.GetJob(p.ID, jobLogsJobID)
			if err != nil {
				return err
			}
			log, _, err := client.Jobs.GetTraceFile(p.ID, jobLogsJobID)
			if err != nil {
				return err
			}
			data, _ := io.ReadAll(log)
			chunk := string(data)
			if len(chunk) > offset {
				fmt.Print(chunk[offset:])
				offset = len(chunk)
			}
			terminal := []string{"success", "failed", "canceled", "skipped"}
			done := false
			for _, s := range terminal {
				if job.Status == s {
					done = true
				}
			}
			if done {
				break
			}
			time.Sleep(3 * time.Second)
		}
		return nil
	},
}

func printLogs(client *gl.Client, projectID string, jobID int64) error {
	log, _, err := client.Jobs.GetTraceFile(projectID, jobID)
	if err != nil {
		return fmt.Errorf("fetching logs for job %d: %w", jobID, err)
	}
	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	return scanner.Err()
}

func init() {
	// list
	jobListCmd.Flags().StringVarP(&jobListProject, "project", "p", "", "project alias or ID")
	jobListCmd.Flags().Int64VarP(&jobListPipelineID, "pipeline", "i", 0, "pipeline ID")
	jobListCmd.Flags().StringArrayVarP(&jobListScope, "scope", "s", nil, "filter by scope (running, pending, success, failed, canceled, manual)")
	_ = jobListCmd.MarkFlagRequired("project")
	_ = jobListCmd.MarkFlagRequired("pipeline")

	// play
	jobPlayCmd.Flags().StringVarP(&jobPlayProject, "project", "p", "", "project alias or ID")
	jobPlayCmd.Flags().Int64VarP(&jobPlayJobID, "id", "i", 0, "job ID")
	_ = jobPlayCmd.MarkFlagRequired("project")
	_ = jobPlayCmd.MarkFlagRequired("id")

	// retry
	jobRetryCmd.Flags().StringVarP(&jobRetryProject, "project", "p", "", "project alias or ID")
	jobRetryCmd.Flags().Int64VarP(&jobRetryJobID, "id", "i", 0, "job ID")
	_ = jobRetryCmd.MarkFlagRequired("project")
	_ = jobRetryCmd.MarkFlagRequired("id")

	// logs
	jobLogsCmd.Flags().StringVarP(&jobLogsProject, "project", "p", "", "project alias or ID")
	jobLogsCmd.Flags().Int64VarP(&jobLogsJobID, "id", "i", 0, "job ID")
	jobLogsCmd.Flags().BoolVarP(&jobLogsFollow, "follow", "f", false, "stream logs until job finishes")
	_ = jobLogsCmd.MarkFlagRequired("project")
	_ = jobLogsCmd.MarkFlagRequired("id")

	jobCmd.AddCommand(jobListCmd, jobPlayCmd, jobRetryCmd, jobLogsCmd)
	rootCmd.AddCommand(jobCmd)
}
