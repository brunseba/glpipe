package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
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
	listGroup   string
	listLimit   int
	listStatus  string
)

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent pipelines (all projects, a group, or a single project)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		projects, err := cfg.ResolveTargets(listProject, listGroup)
		if err != nil {
			return err
		}

		t := ui.NewTable(os.Stdout, []string{"Project", "ID", "Branch", "Status", "Duration", "Created"})

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
	triggerProject    string
	triggerGroup      string
	triggerRef        string
	triggerVars       []string
	triggerPlayManual bool
	triggerInterval   int
)

var pipelineTriggerCmd = &cobra.Command{
	Use:   "trigger",
	Short: "Trigger a new pipeline on a project or a group of projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gitlab.NewClient(cfg)
		if err != nil {
			return err
		}
		projects, err := cfg.ResolveTargets(triggerProject, triggerGroup)
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			return fmt.Errorf("no projects to trigger")
		}

		vars := parseVars(triggerVars)
		glVars := buildGLVars(vars)

		// Trigger all projects and collect pipeline IDs.
		type result struct {
			project config.Project
			pipe    *gl.Pipeline
			err     error
		}
		results := make([]result, len(projects))
		for i, p := range projects {
			ref := triggerRef
			if ref == "" {
				ref = p.DefaultBranch
			}
			pipe, _, trigErr := client.Pipelines.CreatePipeline(p.ID, &gl.CreatePipelineOptions{
				Ref:       gl.Ptr(ref),
				Variables: &glVars,
			})
			results[i] = result{project: p, pipe: pipe, err: trigErr}
		}

		// Report triggered pipelines.
		var triggered []result
		for _, r := range results {
			if r.err != nil {
				color.Red("✗ %s: %v", r.project.Alias, r.err)
				continue
			}
			color.Green("✓ Pipeline #%d — %s @ %s  %s", r.pipe.ID, r.project.Alias, r.pipe.Ref, r.pipe.WebURL)
			triggered = append(triggered, r)
		}

		if !triggerPlayManual || len(triggered) == 0 {
			return nil
		}

		// Watch all triggered pipelines concurrently.
		fmt.Printf("\nWatching %d pipeline(s) with auto-play manual jobs...\n", len(triggered))
		var wg sync.WaitGroup
		errs := make(chan error, len(triggered))
		for _, r := range triggered {
			wg.Add(1)
			go func(r result) {
				defer wg.Done()
				if err := watchPipelineLabeled(client, r.project, r.pipe.ID, triggerInterval); err != nil {
					errs <- fmt.Errorf("%s: %w", r.project.Alias, err)
				}
			}(r)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			color.Red("watch error: %v", e)
		}
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
		return watchPipeline(client, p.ID, watchPipelineID, watchInterval, watchPlayManual)
	},
}

// watchPipeline polls a pipeline until it reaches a terminal state.
// If playManual is true, it triggers manual jobs as they appear.
func watchPipeline(client *gl.Client, projectID string, pipelineID int64, interval int, playManual bool) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()

	played := map[int64]bool{}

	fmt.Printf("Watching pipeline #%d on %s (interval %ds", pipelineID, projectID, interval)
	if playManual {
		fmt.Print(", auto-play manual jobs")
	}
	fmt.Println(")...")

	terminal := map[string]bool{"success": true, "failed": true, "canceled": true, "skipped": true}

	for {
		select {
		case <-sig:
			fmt.Println("\nStopped.")
			return nil
		case <-tick.C:
			pipe, _, err := client.Pipelines.GetPipeline(projectID, pipelineID)
			if err != nil {
				return err
			}
			jobs, _, _ := client.Jobs.ListPipelineJobs(projectID, pipelineID, &gl.ListJobsOptions{})

			if playManual {
				playManualJobs(client, projectID, jobs, played)
			}

			printWatchLine(pipe, jobs)

			if terminal[pipe.Status] {
				fmt.Printf("\nPipeline finished: %s\n", ui.ColorStatus(pipe.Status))
				return nil
			}
		}
	}
}

// watchPipelineLabeled is like watchPipeline but prefixes every log line with
// the project alias — used when watching multiple pipelines concurrently.
func watchPipelineLabeled(client *gl.Client, p config.Project, pipelineID int64, interval int) error {
	label := p.Alias
	if label == "" {
		label = p.ID
	}
	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()

	played := map[int64]bool{}
	terminal := map[string]bool{"success": true, "failed": true, "canceled": true, "skipped": true}

	for range tick.C {
		pipe, _, err := client.Pipelines.GetPipeline(p.ID, pipelineID)
		if err != nil {
			return err
		}
		jobs, _, _ := client.Jobs.ListPipelineJobs(p.ID, pipelineID, &gl.ListJobsOptions{})

		for _, j := range jobs {
			if j.Status != "manual" || played[j.ID] {
				continue
			}
			played[j.ID] = true
			if _, _, err := client.Jobs.PlayJob(p.ID, j.ID, nil); err != nil {
				fmt.Printf("[%s] auto-play job #%d (%s): %v\n", label, j.ID, j.Name, err)
			} else {
				fmt.Printf("[%s] auto-play job #%d (%s) triggered\n", label, j.ID, j.Name)
			}
		}

		fmt.Printf("[%s] pipeline #%d — %s\n", label, pipe.ID, ui.ColorStatus(pipe.Status))

		if terminal[pipe.Status] {
			color.Green("[%s] finished: %s", label, pipe.Status)
			return nil
		}
	}
	return nil
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
	pipelineListCmd.Flags().StringVarP(&listProject, "project", "p", "", "project alias or ID")
	pipelineListCmd.Flags().StringVarP(&listGroup, "group", "g", "", "project group name (defined in config)")
	pipelineListCmd.Flags().IntVarP(&listLimit, "limit", "n", 10, "number of pipelines per project")
	pipelineListCmd.Flags().StringVarP(&listStatus, "status", "s", "", "filter by status (running, success, failed, pending, canceled)")

	// trigger
	pipelineTriggerCmd.Flags().StringVarP(&triggerProject, "project", "p", "", "project alias or ID")
	pipelineTriggerCmd.Flags().StringVarP(&triggerGroup, "group", "g", "", "project group name (defined in config)")
	pipelineTriggerCmd.Flags().StringVarP(&triggerRef, "ref", "r", "", "branch or tag (default: each project's default_branch)")
	pipelineTriggerCmd.Flags().StringArrayVarP(&triggerVars, "var", "v", nil, "pipeline variable in KEY=VALUE format (repeatable)")
	pipelineTriggerCmd.Flags().BoolVar(&triggerPlayManual, "play-manual", false, "watch and auto-play manual jobs until all pipelines finish")
	pipelineTriggerCmd.Flags().IntVar(&triggerInterval, "interval", 5, "polling interval in seconds (used with --play-manual)")

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
