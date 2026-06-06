package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	FUZZY_MAX_RESULTS = 12
	PIN_DIR           = ".pins"
	PIN_FILE          = "PIN.md"
	INDEX_FILE        = "INDEX.md"
)

func getFuzzyIgnoreDirs() []string {
	return []string{PIN_DIR, ".git", "node_modules", "vendor"}
}

var rootCmd = &cobra.Command{
	Use:   "pinner",
	Short: "Pin code examples for rediscovery",
	Long:  `snippet_pinner is a CLI tool that allows users to 'pin' code files and maintain a curated library of examples for discovery and reuse by LLMs, improving output consistency. The tool is designed for manual curation, encouraging the user to read AI-generated code to improve its results in the future.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
