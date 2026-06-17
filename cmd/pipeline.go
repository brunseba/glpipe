package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/sebrun/glpipe/internal/config"
	"github.com/sebrun/glpipe/internal/gitlab"
	"github.com/sebrun/glpipe/internal/ui"
	"github.com/spf13/cobra"
	gl "gitlab.com/gitlab-org/api/client-go"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage pipelines",
}

// ── list ──────────────────────────────────────────────────────────────────────

var (
	listProject string
	listLimit   int
	listStatus  string
)

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent pipelines (all projects or a specific one)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}

		projects := cfg.Projects
		if listProject != "" {
			p, err := cfg.FindProject(listProject)
			if err != nil {
				return err
			}
			projects = []config.Project{*p}
		}

		t := ui.NewTable(os.Stdout, []string{"Project", "ID", "Branch", "Status", "Triggered by", "Duration", "Created"})

		for _, p := range projects {
			opts := &gl.ListProjectPipelinesOptions{
				ListOptions: gl.ListOptions{PerPage: int64(listLimit)},
			}
			if listStatus != "" {
				s := gl.BuildStateValue(listStatus)
				opts.Status = &s
			}
			pipelines, _, err := client.Pipelines.ListProjectPipelines(p.ID, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: %s: %v\n", p.ID, err)
				continue
			}
			label := p.Alias
			if label == "" {
				label = p.ID
			}
			for _, pipe := range pipelines {
				_ = t.Append([]string{
					label,
					fmt.Sprintf("%d", pipe.ID),
					pipe.Ref,
					ui.ColorStatus(pipe.Status),
					"-",
					ui.FormatDuration(pipe.CreatedAt, pipe.UpdatedAt),
					pipe.CreatedAt.Format("2006-01-02 15:04"),
				})
			}
		}
		return t.Render()
	},
}

// ── trigger ───────────────────────────────────────────────────────────────────

var (
	triggerProject string
	triggerRef     string
	triggerVars    []string
)

var pipelineTriggerCmd = &cobra.Command{
	Use:   "trigger",
	Short: "Trigger a new pipeline on a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		p, err := cfg.FindProject(triggerProject)
		if err != nil {
			return err
		}
		ref := triggerRef
		if ref == "" {
			ref = p.DefaultBranch
		}

		vars := parseVars(triggerVars)
		glVars := buildGLVars(vars)
		opts := &gl.CreatePipelineOptions{
			Ref:       gl.Ptr(ref),
			Variables: &glVars,
		}

		pipe, _, err := client.Pipelines.CreatePipeline(p.ID, opts)
		if err != nil {
			return fmt.Errorf("triggering pipeline: %w", err)
		}

		color.Green("Pipeline #%d created on %s @ %s", pipe.ID, p.ID, ref)
		fmt.Printf("URL: %s\n", pipe.WebURL)
		return nil
	},
}

// ── cancel ────────────────────────────────────────────────────────────────────

var (
	cancelProject    string
	cancelPipelineID int64
)

var pipelineCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a running pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		p, err := cfg.FindProject(cancelProject)
		if err != nil {
			return err
		}
		_, _, err = client.Pipelines.CancelPipelineBuild(p.ID, cancelPipelineID)
		if err != nil {
			return fmt.Errorf("canceling pipeline: %w", err)
		}
		color.Yellow("Pipeline #%d canceled.", cancelPipelineID)
		return nil
	},
}

// ── watch ─────────────────────────────────────────────────────────────────────

var (
	watchProject    string
	watchPipelineID int64
	watchInterval   int
	watchPlayManual bool
)

var pipelineWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch a pipeline until it completes (Ctrl+C to stop)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		p, err := cfg.FindProject(watchProject)
		if err != nil {
			return err
		}

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		tick := time.NewTicker(time.Duration(watchInterval) * time.Second)
		defer tick.Stop()

		played := map[int64]bool{} // jobs already played, avoid double-trigger

		fmt.Printf("Watching pipeline #%d on %s (interval %ds", watchPipelineID, p.ID, watchInterval)
		if watchPlayManual {
			fmt.Print(", auto-play manual jobs")
		}
		fmt.Println(")...")

		for {
			select {
			case <-sig:
				fmt.Println("\nStopped.")
				return nil
			case <-tick.C:
				pipe, _, err := client.Pipelines.GetPipeline(p.ID, watchPipelineID)
				if err != nil {
					return err
				}
				jobs, _, _ := client.Jobs.ListPipelineJobs(p.ID, watchPipelineID, &gl.ListJobsOptions{})

				if watchPlayManual {
					playManualJobs(client, p.ID, jobs, played)
				}

				printWatchLine(pipe, jobs)

				terminal := []string{"success", "failed", "canceled", "skipped"}
				for _, s := range terminal {
					if pipe.Status == s {
						fmt.Printf("\nPipeline finished: %s\n", ui.ColorStatus(pipe.Status))
						return nil
					}
				}
			}
		}
	},
}

// playManualJobs triggers any job in 'manual' state that hasn't been played yet.
func playManualJobs(client *gl.Client, projectID string, jobs []*gl.Job, played map[int64]bool) {
	for _, j := range jobs {
		if j.Status != "manual" || played[j.ID] {
			continue
		}
		played[j.ID] = true
		_, _, err := client.Jobs.PlayJob(projectID, j.ID, nil)
		if err != nil {
			fmt.Printf("\n[auto-play] job #%d (%s): error: %v\n", j.ID, j.Name, err)
		} else {
			fmt.Printf("\n[auto-play] job #%d (%s) triggered\n", j.ID, j.Name)
		}
	}
}

func printWatchLine(pipe *gl.Pipeline, jobs []*gl.Job) {
	fmt.Printf("\r[%s] Pipeline #%d — %s   ",
		time.Now().Format("15:04:05"),
		pipe.ID,
		ui.ColorStatus(pipe.Status),
	)
	for _, j := range jobs {
		if j.Status == "running" || j.Status == "manual" || j.Status == "failed" {
			fmt.Printf("  %s:%s", j.Name, ui.ColorStatus(j.Status))
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseVars(raw []string) map[string]string {
	out := map[string]string{}
	for _, kv := range raw {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

func buildGLVars(vars map[string]string) []*gl.PipelineVariableOptions {
	out := make([]*gl.PipelineVariableOptions, 0, len(vars))
	for k, v := range vars {
		out = append(out, &gl.PipelineVariableOptions{
			Key:          gl.Ptr(k),
			Value:        gl.Ptr(v),
			VariableType: gl.Ptr(gl.EnvVariableType),
		})
	}
	return out
}

func init() {
	// list
	pipelineListCmd.Flags().StringVarP(&listProject, "project", "p", "", "project alias or ID (default: all)")
	pipelineListCmd.Flags().IntVarP(&listLimit, "limit", "n", 10, "number of pipelines per project")
	pipelineListCmd.Flags().StringVarP(&listStatus, "status", "s", "", "filter by status (running, success, failed, pending, canceled)")

	// trigger
	pipelineTriggerCmd.Flags().StringVarP(&triggerProject, "project", "p", "", "project alias or ID")
	pipelineTriggerCmd.Flags().StringVarP(&triggerRef, "ref", "r", "", "branch or tag (default: project's default_branch)")
	pipelineTriggerCmd.Flags().StringArrayVarP(&triggerVars, "var", "v", nil, "pipeline variable in KEY=VALUE format (repeatable)")
	_ = pipelineTriggerCmd.MarkFlagRequired("project")

	// cancel
	pipelineCancelCmd.Flags().StringVarP(&cancelProject, "project", "p", "", "project alias or ID")
	pipelineCancelCmd.Flags().Int64VarP(&cancelPipelineID, "id", "i", 0, "pipeline ID")
	_ = pipelineCancelCmd.MarkFlagRequired("project")
	_ = pipelineCancelCmd.MarkFlagRequired("id")

	// watch
	pipelineWatchCmd.Flags().StringVarP(&watchProject, "project", "p", "", "project alias or ID")
	pipelineWatchCmd.Flags().Int64VarP(&watchPipelineID, "id", "i", 0, "pipeline ID")
	pipelineWatchCmd.Flags().IntVar(&watchInterval, "interval", 5, "polling interval in seconds")
	pipelineWatchCmd.Flags().BoolVar(&watchPlayManual, "play-manual", false, "automatically trigger manual jobs as they appear")
	_ = pipelineWatchCmd.MarkFlagRequired("project")
	_ = pipelineWatchCmd.MarkFlagRequired("id")

	pipelineCmd.AddCommand(pipelineListCmd, pipelineTriggerCmd, pipelineCancelCmd, pipelineWatchCmd)
	rootCmd.AddCommand(pipelineCmd)
}
