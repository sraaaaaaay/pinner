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

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var pinCmd = &cobra.Command{
	Use:   "pin <name> [files...]",
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
	rootCmd.AddCommand(pinCmd)
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

	dir := fmt.Sprintf("pins/%s", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(err)
	}

	kw := getKeywords(name)
	files := getFilesources(args[1:])
	createCopies(dir, args[1:])

	fm := frontmatter{
		Keywords: string(kw),
		Files:    files,
	}

	yaml, err := yaml.MarshalWithOptions(fm, yaml.IndentSequence(true))
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(fmt.Sprintf("pins/%s/PIN.md", name), fmt.Appendf(nil, "---\n%s---", yaml), 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func getKeywords(name string) []byte {
	fmt.Println("Creating pin: " + name)
	fmt.Print("Keywords (comma-separated): ")
	r := bufio.NewReader(os.Stdin)
	kw, _ := r.ReadBytes('\n')
	return bytes.TrimSpace(kw)
}

func createCopies(dir string, fileArgs []string) {
	for _, file := range fileArgs {
		dst := filepath.Join(dir, filepath.Base(filepath.Dir(file)), filepath.Base(file))
		if err := copyFile(file, dst); err != nil {
			log.Fatal(err)
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

func getFilesources(fileArgs []string) []filesource {
	files := []filesource{}
	hasher := sha256.New()

	for _, file := range fileArgs {
		f, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		if _, err := io.Copy(hasher, f); err != nil {
			log.Fatal(err)
		}

		files = append(files, filesource{
			Source: file,
			Sha256: hex.EncodeToString(hasher.Sum(nil)),
		})

		hasher.Reset()
	}

	return files
}
