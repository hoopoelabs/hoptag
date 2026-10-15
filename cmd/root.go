package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "hoptag",
	Short: "hoptag — Modern Infrastructure as Code (IaC) Auto-Tagger & Governance CLI",
	Long: `hoptag is a high-performance, AST-safe CLI tool developed by Hoopoe Labs.
It automatically scans Terraform / OpenTofu IaC files and injects, merges, and validates
resource tags and labels across AWS, Microsoft Azure, and Google Cloud Platform (GCP).

Features:
  - AST-safe: Preserves comments, whitespace, and formatting using HashiCorp hclwrite.
  - Multi-Cloud: Seamlessly targets AWS (tags), Azure (tags), and GCP (labels).
  - Non-destructive: In-place edits, native override files, or clean backups.
  - Governance-ready: Enforces required tag keys and regex value compliance.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", red(bold("Error:")), err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
}

// Color helpers
func green(s string) string {
	return "\033[32m" + s + "\033[0m"
}

func yellow(s string) string {
	return "\033[33m" + s + "\033[0m"
}

func red(s string) string {
	return "\033[31m" + s + "\033[0m"
}

func cyan(s string) string {
	return "\033[36m" + s + "\033[0m"
}

func bold(s string) string {
	return "\033[1m" + s + "\033[0m"
}

func printBanner() {
	if quiet {
		return
	}
	fmt.Println(cyan(bold("  _                 _              ")))
	fmt.Println(cyan(bold(" | |__   ___  _ __ | |_ __ _  __ _ ")))
	fmt.Println(cyan(bold(" | '_ \\ / _ \\| '_ \\| __/ _' |/ _' |")))
	fmt.Println(cyan(bold(" | | | | (_) | |_) | || (_| | (_| |")))
	fmt.Println(cyan(bold(" |_| |_|\\___/| .__/ \\__\\__,_|\\__, |")))
	fmt.Println(cyan(bold("             |_|             |___/ ")))
	fmt.Println(bold(" Modern IaC Auto-Tagger & Governance CLI | Hoopoe Labs"))
	fmt.Println()
}
