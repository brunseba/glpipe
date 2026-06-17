package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

// completionCmd replaces Cobra's built-in so we can add `install` as a sibling.
var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate or install shell completion scripts",
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate zsh completion script",
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenZshCompletion(os.Stdout)
	},
}

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generate bash completion script",
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenBashCompletion(os.Stdout)
	},
}

var completionFishCmd = &cobra.Command{
	Use:   "fish",
	Short: "Generate fish completion script",
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenFishCompletion(os.Stdout, true)
	},
}

var completionPsCmd = &cobra.Command{
	Use:   "powershell",
	Short: "Generate PowerShell completion script",
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenPowerShellCompletion(os.Stdout)
	},
}

var completionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Auto-detect shell and install completion script",
	Long: `Detect the current shell and write the completion script to the right location.

Supported shells: bash, zsh, fish.
After running this command, open a new terminal (or source the file) to activate.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := filepath.Base(os.Getenv("SHELL"))
		switch shell {
		case "zsh":
			return installZsh()
		case "bash":
			return installBash()
		case "fish":
			return installFish()
		default:
			return fmt.Errorf("shell %q not supported — run one of:\n  glpipe completion zsh\n  glpipe completion bash\n  glpipe completion fish", shell)
		}
	},
}

func installZsh() error {
	dirs := []string{
		filepath.Join(os.Getenv("HOME"), ".zsh", "completions"),
		"/usr/local/share/zsh/site-functions",
		"/usr/share/zsh/site-functions",
	}
	dir, err := firstWritableDir(dirs)
	if err != nil {
		return fmt.Errorf("no writable zsh completion directory found\nRun manually: glpipe completion zsh > ~/.zsh/completions/_glpipe")
	}
	dest := filepath.Join(dir, "_glpipe")
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := rootCmd.GenZshCompletion(f); err != nil {
		return err
	}
	fmt.Printf("Zsh completion installed: %s\n", dest)
	fmt.Println("Add to ~/.zshrc if not already present:")
	fmt.Printf("  fpath=(%s $fpath) && autoload -Uz compinit && compinit\n", dir)
	return nil
}

func installBash() error {
	var dir string
	if runtime.GOOS == "darwin" {
		dir = "/usr/local/etc/bash_completion.d"
	} else {
		dir = "/etc/bash_completion.d"
	}
	dest := filepath.Join(dir, "glpipe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dest = filepath.Join(os.Getenv("HOME"), ".bash_completion.d", "glpipe")
		_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w\nRun manually: glpipe completion bash > ~/.bash_completion.d/glpipe", dest, err)
	}
	defer f.Close()
	if err := rootCmd.GenBashCompletion(f); err != nil {
		return err
	}
	fmt.Printf("Bash completion installed: %s\n", dest)
	fmt.Println("Add to ~/.bashrc if not already present:")
	fmt.Printf("  source %s\n", dest)
	return nil
}

func installFish() error {
	dir := filepath.Join(os.Getenv("HOME"), ".config", "fish", "completions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, "glpipe.fish")
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := rootCmd.GenFishCompletion(f, true); err != nil {
		return err
	}
	fmt.Printf("Fish completion installed: %s\n", dest)
	return nil
}

func firstWritableDir(dirs []string) (string, error) {
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			continue
		}
		probe := filepath.Join(d, ".glpipe_probe")
		f, err := os.Create(probe)
		if err != nil {
			continue
		}
		f.Close()
		_ = os.Remove(probe)
		return d, nil
	}
	return "", fmt.Errorf("no writable dir")
}

func init() {
	// Disable Cobra's built-in completion command and use ours instead.
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	completionCmd.AddCommand(completionZshCmd, completionBashCmd, completionFishCmd, completionPsCmd, completionInstallCmd)
	rootCmd.AddCommand(completionCmd)
}
