package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all pins within the current directory",
	Long:  "Clear all pins within the current directory",
	Run: func(cmd *cobra.Command, args []string) {
		if err := os.RemoveAll(PIN_DIR); err != nil {
			log.Fatal(err)
		}

		if err := os.MkdirAll(PIN_DIR, 0755); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(clearCmd)
}
