package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	PIN_DIR    = ".pins"
	PIN_FILE   = "PIN.md"
	INDEX_FILE = "INDEX.md"
)

var rootCmd = &cobra.Command{
	Use:   "pinner",
	Short: "Pin code examples for rediscovery",
	Long:  `snippet_pinner is a CLI tool that allows users to 'pin' code files and maintain a curated library of examples for discovery and reuse by LLMs, improving output consistency. The tool is designed for manual curation, encouraging the user to read AI-generated code to improve its results in the future.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
