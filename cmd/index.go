package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Create an index of pins",
	Long:  "Create an index of pins",
	Run: func(cmd *cobra.Command, args []string) {
		index()
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

func index() {
	if err := os.MkdirAll(PIN_DIR, 0755); err != nil {
		log.Fatal(err)
	}

	entries, err := os.ReadDir(PIN_DIR)
	if err != nil {
		log.Fatal(err)
	}

	index := map[string][]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fm, err := readFrontmatter(fmt.Sprintf("%s/%s/%s", PIN_DIR, entry.Name(), PIN_FILE))
		if errors.Is(err, os.ErrNotExist) {
			// Incomplete pin (no PIN.md yet); skip rather than fail the index.
			continue
		}
		if err != nil {
			log.Fatal(err)
		}

		for kw := range strings.SplitSeq(fm.Keywords, ",") {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}

			for _, f := range fm.Files {
				if !slices.Contains(index[kw], f.Snapshot) {
					index[kw] = append(index[kw], f.Snapshot)
				}
			}
		}
	}

	for kw := range index {
		slices.Sort(index[kw])
	}

	out, err := yaml.Marshal(index)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/%s", PIN_DIR, INDEX_FILE), out, 0644); err != nil {
		log.Fatal(err)
	}
}

func readFrontmatter(path string) (frontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return frontmatter{}, err
	}

	body := bytes.TrimSpace(data)
	body = bytes.TrimPrefix(body, []byte("---"))
	body = bytes.TrimSuffix(body, []byte("---"))

	var fm frontmatter
	if err := yaml.Unmarshal(body, &fm); err != nil {
		return frontmatter{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	return fm, nil
}
