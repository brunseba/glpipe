package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	tw "github.com/olekukonko/tablewriter/tw"
)

var (
	statusColors = map[string]func(...interface{}) string{
		"success":  color.New(color.FgGreen).SprintFunc(),
		"running":  color.New(color.FgCyan).SprintFunc(),
		"pending":  color.New(color.FgYellow).SprintFunc(),
		"failed":   color.New(color.FgRed).SprintFunc(),
		"canceled": color.New(color.FgHiBlack).SprintFunc(),
		"manual":   color.New(color.FgMagenta).SprintFunc(),
		"created":  color.New(color.FgWhite).SprintFunc(),
		"skipped":  color.New(color.FgHiBlack).SprintFunc(),
	}
)

func ColorStatus(status string) string {
	if fn, ok := statusColors[strings.ToLower(status)]; ok {
		return fn(status)
	}
	return status
}

func NewTable(w io.Writer, headers []string) *tablewriter.Table {
	if w == nil {
		w = os.Stdout
	}
	t := tablewriter.NewTable(w,
		tablewriter.WithHeader(headers),
		tablewriter.WithBorders(tw.Border{Left: tw.Off, Right: tw.Off, Top: tw.Off, Bottom: tw.Off}),
		tablewriter.WithAlignment(tw.MakeAlign(1, tw.AlignLeft)),
		tablewriter.WithHeaderAlignment(tw.AlignLeft),
	)
	return t
}

func FormatDuration(start, end *time.Time) string {
	if start == nil {
		return "-"
	}
	finish := time.Now()
	if end != nil {
		finish = *end
	}
	d := finish.Sub(*start).Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
