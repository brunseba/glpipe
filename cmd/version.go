package cmd

import (
	"fmt"

	"github.com/sebrun/glpipe/internal/build"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(build.Info())
	},
}

func init() {
	rootCmd.Version = build.Version
	rootCmd.AddCommand(versionCmd)
}
