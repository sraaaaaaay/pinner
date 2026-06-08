package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/goccy/go-yaml"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Start creating a named pin for a group of files",
	Long:  "Start creating a named pin for a group of files",
	Args:  cobra.ExactArgs(1),
	Run:   pin,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

type frontmatter struct {
	Keywords string       `yaml:"keywords"`
	Files    []filesource `yaml:"files"`
}

type filesource struct {
	Source   string `yaml:"source"`
	Snapshot string `yaml:"snapshot"`
	Sha256   string `yaml:"sha256"`
}

func pin(cmd *cobra.Command, args []string) {
	name := args[0]
	fmt.Println("Creating pin: " + name)

	if files := pickFiles(); len(files) > 0 {
		dir := fmt.Sprintf("%s/%s", PIN_DIR, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal(err)
		}

		kw, err := getKeywords()
		if err != nil {
			log.Fatal(err)
		}

		names := dedupeNames(files)
		sources := createFileSources(files, name, names)
		hashToComment := getComments(sources)

		createFrontmatter(name, kw, sources)
		createCopies(dir, sources, hashToComment)
		index()
	}
}

func pickFiles() []string {
	files, err := listFiles(".")
	if err != nil {
		log.Fatal(err)
	}

	final, err := tea.NewProgram(newModel(files)).Run()
	if err != nil {
		log.Fatal(err)
	}

	for range lipgloss.Height(final.View().Content) {
		fmt.Print("\033[1A\033[2K")
	}

	return final.(model).selected
}

func listFiles(root string) ([]string, error) {
	var files []string
	ignoreDirs := getFuzzyIgnoreDirs()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if slices.Contains(ignoreDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

func getKeywords() ([]byte, error) {
	fmt.Print("Keywords (comma-separated): ")
	r := bufio.NewReader(os.Stdin)
	kw, err := r.ReadBytes('\n')
	kw = bytes.TrimSpace(kw)

	if err != nil {
		return nil, fmt.Errorf("error getting keywords: %w", err)
	}

	if len(kw) == 0 {
		return nil, errors.New("must enter at least 1 keyword")
	}

	return kw, nil
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

func createCopies(dir string, sources []filesource, hashToComment map[string]string) {
	for _, src := range sources {
		dst := fmt.Sprintf("%s/%s%s", dir, filepath.Base(src.Source), ".md")
		if err := copyAsMarkdown(src.Source, dst, hashToComment[src.Sha256]); err != nil {
			log.Fatal(err)
		}
	}
}

func copyAsMarkdown(src, dst, comment string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, asMarkdown(src, comment, content), 0644)
}

func asMarkdown(src string, comment string, content []byte) []byte {
	lang := strings.TrimPrefix(filepath.Ext(src), ".")
	fence := strings.Repeat("`", max(3, longestBacktickRun(content)+1))

	var b bytes.Buffer
	fmt.Fprintf(&b, "---\ncomment: %s---\n\n", comment)
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

func createFileSources(fileArgs []string, pinName string, names []string) []filesource {
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
			Source:   f.Name(),
			Snapshot: filepath.ToSlash(filepath.Join(PIN_DIR, pinName, names[i]+".md")),
			Sha256:   hex.EncodeToString(hasher.Sum(nil)),
		})

		hasher.Reset()
	}

	return files
}

func getComments(sources []filesource) map[string]string {
	comments := make(map[string]string, len(sources))
	rd := bufio.NewReader(os.Stdin)

	for _, src := range sources {
		fmt.Printf("Comments for %s: ", src.Source)
		if comment, err := rd.ReadString('\n'); err == nil {
			comments[src.Sha256] = comment
		}
	}

	return comments
}

func createFrontmatter(name string, kw []byte, sources []filesource) {
	yaml, err := yaml.MarshalWithOptions(
		frontmatter{
			Keywords: string(kw),
			Files:    sources,
		}, yaml.IndentSequence(true))

	if err != nil {
		log.Fatal(err)
	}

	if err = os.WriteFile(fmt.Sprintf("%s/%s/%s", PIN_DIR, name, PIN_FILE), fmt.Appendf(nil, "---\n%s---\n", yaml), 0644); err != nil {
		log.Fatal(err)
	}
}

type model struct {
	ti       textinput.Model
	style    lipgloss.Style
	files    []string
	matches  []string
	selected []string
	cursor   int
}

func newModel(files []string) model {
	ti := textinput.New()
	ti.Prompt = "Search files: "
	ti.Focus()

	m := model{ti: ti, files: files, style: lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))}
	m.filter()
	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) filter() {
	m.matches = m.files
	if query := m.ti.Value(); query != "" {
		m.matches = nil
		for _, res := range fuzzy.Find(query, m.files) {
			m.matches = append(m.matches, res.Str)
		}
	}
	m.cursor = min(m.cursor, max(0, len(m.matches)-1))
}

func (m *model) toggle(path string) {
	if i := slices.Index(m.selected, path); i >= 0 {
		m.selected = slices.Delete(m.selected, i, i+1)
	} else {
		m.selected = append(m.selected, path)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch k := key.Key(); {
	case k.Mod.Contains(tea.ModCtrl) && k.Code == 'c':
		m.selected = nil
		return m, tea.Quit
	case k.Mod.Contains(tea.ModCtrl) && k.Code == tea.KeyBackspace:
		m.ti.Reset()
	case k.Code == tea.KeyEnter && len(m.selected) > 0:
		return m, tea.Quit
	case k.Code == tea.KeyTab && m.cursor < len(m.matches):
		m.toggle(m.matches[m.cursor])
	case k.Code == tea.KeyUp && m.cursor > 0:
		m.cursor--
	case k.Code == tea.KeyDown && m.cursor < len(m.matches)-1:
		m.cursor++
	default:
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		m.filter()
		return m, cmd
	}
	return m, nil
}

func (m model) View() tea.View {
	var b strings.Builder

	confirm := ""
	if len(m.selected) > 0 {
		confirm = fmt.Sprintf(" · enter: confirm (%d selected)", len(m.selected))
	}

	fmt.Fprintf(&b, m.style.Render("\ntab: toggle · ↑/↓: move · ctrl+c: cancel%s"), confirm)
	fmt.Fprintf(&b, "\n%s\n", m.ti.View())

	start := max(0, m.cursor-FUZZY_MAX_RESULTS+1)
	for i := start; i < min(start+FUZZY_MAX_RESULTS, len(m.matches)); i++ {
		cursor, check := "  ", "[ ]"
		if i == m.cursor {
			cursor = "> "
		}
		if slices.Contains(m.selected, m.matches[i]) {
			check = "[x]"
		}
		fmt.Fprintf(&b, "%s%s %s\n", cursor, check, m.matches[i])
	}
	if len(m.matches) == 0 {
		b.WriteString("  (no matches)\n")
	}
	return tea.NewView(b.String())
}
