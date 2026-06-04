package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <name> [files...]",
	Short: "Create a pin for a group of files",
	Long:  "Create a pin for a group of files",
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), validateArgs),
	Run:   pin,
}

func validateArgs(cmd *cobra.Command, args []string) error {
	for _, f := range args[1:] {
		if info, err := os.Stat(f); err != nil {
			return fmt.Errorf("cannot pin %q: %w", f, err)
		} else if info.IsDir() {
			return fmt.Errorf("cannot pin %q: is a directory", f)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(addCmd)
}

type frontmatter struct {
	Keywords string       `yaml:"keywords"`
	Files    []filesource `yaml:"files"`
}

type filesource struct {
	Source string `yaml:"source"`
	Sha256 string `yaml:"sha256"`
}

func pin(cmd *cobra.Command, args []string) {
	name := args[0]

	dir := fmt.Sprintf("%s/%s", PIN_DIR, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(err)
	}

	names := dedupeNames(args[1:])
	kw := getKeywords(name)
	files := getFilesources(args[1:], name, names)
	createCopies(dir, args[1:], names)

	fm := frontmatter{
		Keywords: string(kw),
		Files:    files,
	}

	yaml, err := yaml.MarshalWithOptions(fm, yaml.IndentSequence(true))
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(fmt.Sprintf("%s/%s/%s", PIN_DIR, name, PIN_FILE), fmt.Appendf(nil, "---\n%s---", yaml), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// After adding, re-index
	indexCmd.Run(cmd, args)
}

func getKeywords(name string) []byte {
	fmt.Println("Creating pin: " + name)
	fmt.Print("Keywords (comma-separated): ")
	r := bufio.NewReader(os.Stdin)
	kw, _ := r.ReadBytes('\n')
	return bytes.TrimSpace(kw)
}

func createCopies(dir string, fileArgs []string, names []string) {
	for i, file := range fileArgs {
		dst := filepath.Join(dir, names[i]+".md")
		if err := copyAsMarkdown(file, dst); err != nil {
			log.Fatal(err)
		}
	}
}

func dedupeNames(fileArgs []string) []string {
	counts := map[string]int{}
	for _, file := range fileArgs {
		counts[filepath.Base(file)]++
	}

	names := make([]string, len(fileArgs))
	for i, file := range fileArgs {
		base := filepath.Base(file)
		if counts[base] > 1 {
			names[i] = filepath.Join(filepath.Base(filepath.Dir(file)), base)
		} else {
			names[i] = base
		}
	}
	return names
}

func copyAsMarkdown(src, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, asMarkdown(src, content), 0644)
}

func asMarkdown(src string, content []byte) []byte {
	lang := strings.TrimPrefix(filepath.Ext(src), ".")
	fence := strings.Repeat("`", max(3, longestBacktickRun(content)+1))

	var b bytes.Buffer
	fmt.Fprintf(&b, "%s%s\n", fence, lang)
	b.Write(content)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "%s\n", fence)
	return b.Bytes()
}

func longestBacktickRun(content []byte) int {
	longest, run := 0, 0
	for _, c := range content {
		if c == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return longest
}

func getFilesources(fileArgs []string, name string, names []string) []filesource {
	files := []filesource{}
	hasher := sha256.New()

	for i, file := range fileArgs {
		f, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		if _, err := io.Copy(hasher, f); err != nil {
			log.Fatal(err)
		}

		files = append(files, filesource{
			Source: "./" + filepath.ToSlash(filepath.Join(PIN_DIR, name, names[i]+".md")),
			Sha256: hex.EncodeToString(hasher.Sum(nil)),
		})

		hasher.Reset()
	}

	return files
}
