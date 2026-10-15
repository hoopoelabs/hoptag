package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Version   = "v0.1.0"
	Commit    = "dev"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print hoptag version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("hoptag %s (commit: %s, built at: %s)\n", Version, Commit, BuildDate)
		fmt.Printf("Go runtime: %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		fmt.Println("Developed by Hoopoe Labs (https://github.com/hoopoelabs/hoptag)")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
