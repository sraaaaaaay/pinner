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
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
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

	files := pickFiles()
	if len(files) == 0 {
		return
	}

	fileToLines, ok := pickLines(files)
	if !ok {
		return
	}

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
	createCopies(dir, sources, hashToComment, fileToLines)
	index()
}

func pickFiles() []string {
	files, err := listFiles(".")
	if err != nil {
		log.Fatal(err)
	}

	final, err := tea.NewProgram(newFilePicker(files)).Run()
	if err != nil {
		log.Fatal(err)
	}

	clearView(final)
	return final.(filePicker).selected
}

func pickLines(files []string) (map[string]pickedLine, bool) {
	final, err := tea.NewProgram(newLinePicker(files)).Run()
	if err != nil {
		log.Fatal(err)
	}

	clearView(final)
	lp := final.(linePicker)
	return lp.picked, !lp.aborted
}

func clearView(final tea.Model) {
	for range lipgloss.Height(final.View().Content) {
		fmt.Print("\033[1A\033[2K")
	}
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

func createCopies(dir string, sources []filesource, hashToComment map[string]string, fileToLines map[string]pickedLine) {
	for _, src := range sources {
		picked, ok := fileToLines[src.Source]
		if !ok {
			picked = pickedLine{start: -1, end: -1}
		}

		dst := fmt.Sprintf("%s/%s%s", dir, filepath.Base(src.Source), ".md")
		if err := copyAsMarkdown(src.Source, dst, hashToComment[src.Sha256], picked); err != nil {
			log.Fatal(err)
		}
	}
}

func copyAsMarkdown(src, dst, comment string, picked pickedLine) error {
	var content bytes.Buffer
	isWholeFile := picked.start == -1 && picked.end == -1

	if !isWholeFile {
		content.WriteString("...\n")
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	l := 0
	for scanner.Scan() {
		if isWholeFile || l >= picked.start && l <= picked.end {
			content.Write(scanner.Bytes())
			content.WriteRune('\n')
			l++
		}
	}

	if !isWholeFile {
		content.WriteString("...\n")
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, asMarkdown(src, comment, content.Bytes()), 0644)
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

var hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
var highlight = lipgloss.NewStyle().Background(lipgloss.Color("237"))

type filePicker struct {
	ti       textinput.Model
	files    []string
	matches  []string
	selected []string
	cursor   int
}

func newFilePicker(files []string) filePicker {
	ti := textinput.New()
	ti.Prompt = "Search files: "
	ti.Focus()

	m := filePicker{
		ti:    ti,
		files: files,
	}

	m.filter()
	return m
}

func (m filePicker) Init() tea.Cmd {
	return textinput.Blink
}

func (m *filePicker) filter() {
	m.matches = m.files
	if query := m.ti.Value(); query != "" {
		m.matches = nil
		for _, res := range fuzzy.Find(query, m.files) {
			m.matches = append(m.matches, res.Str)
		}
	}
	m.cursor = min(m.cursor, max(0, len(m.matches)-1))
}

func (m *filePicker) toggle(path string) {
	if i := slices.Index(m.selected, path); i >= 0 {
		m.selected = slices.Delete(m.selected, i, i+1)
	} else {
		m.selected = append(m.selected, path)
	}
}

func (m filePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m filePicker) View() tea.View {
	var b strings.Builder

	confirm := ""
	if len(m.selected) > 0 {
		confirm = fmt.Sprintf(" · enter: confirm (%d selected)", len(m.selected))
	}

	fmt.Fprintf(&b, hintStyle.Render("\ntab: toggle · ↑/↓: move · ctrl+c: cancel%s"), confirm)
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

type pickedLine struct {
	start int
	end   int
}

type linePicker struct {
	vp             viewport.Model
	files          []string
	picked         map[string]pickedLine
	markingFileIdx int
	lineCursor     int
	pendingStart   int
	aborted        bool
}

func newLinePicker(files []string) linePicker {
	vp := viewport.New()
	vp.FillHeight = true
	vp.SoftWrap = true

	m := linePicker{
		vp:           vp,
		files:        files,
		picked:       map[string]pickedLine{},
		pendingStart: -1,
	}

	m.loadFile(0)
	return m
}

func (m linePicker) Init() tea.Cmd {
	return nil
}

func (m linePicker) currentFile() string {
	return m.files[m.markingFileIdx]
}

func (m *linePicker) loadFile(i int) {
	b, err := os.ReadFile(m.files[i])
	if err != nil {
		log.Fatal(err)
	}
	m.markingFileIdx = i
	m.lineCursor = 0
	m.pendingStart = -1
	m.vp.SetContent(string(b))
	m.vp.GotoTop()
}

func (m *linePicker) markLine() {
	// If we hit space again after selecting start/end, clear the selection
	if _, done := m.picked[m.currentFile()]; done {
		delete(m.picked, m.currentFile())
		return
	}

	if m.pendingStart < 0 {
		m.pendingStart = m.lineCursor
		return
	}

	m.picked[m.currentFile()] = pickedLine{
		start: min(m.pendingStart, m.lineCursor),
		end:   max(m.pendingStart, m.lineCursor),
	}

	m.pendingStart = -1
}

func (m linePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.vp.SetWidth(wsMsg.Width)
		m.vp.SetHeight(wsMsg.Height - 1)
	}

	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	switch k := key.Key(); {
	case k.Mod.Contains(tea.ModCtrl) && k.Code == 'c':
		m.aborted = true
		return m, tea.Quit
	case k.Code == tea.KeyUp && m.lineCursor > 0:
		m.lineCursor--
		m.vp.EnsureVisible(m.lineCursor, 0, 0)
	case k.Code == tea.KeyDown && m.lineCursor < m.vp.TotalLineCount()-1:
		m.lineCursor++
		m.vp.EnsureVisible(m.lineCursor, 0, 0)
	case k.Code == tea.KeySpace:
		m.markLine()
	case k.Code == tea.KeyEnter:
		if m.pendingStart >= 0 {
			m.markLine()
		}
		if m.markingFileIdx+1 >= len(m.files) {
			return m, tea.Quit
		}
		m.loadFile(m.markingFileIdx + 1)
	}

	return m, nil
}

func (m linePicker) View() tea.View {
	picked, done := m.picked[m.currentFile()]

	status := "whole file"
	if done {
		status = fmt.Sprintf("lines %d-%d", picked.start+1, picked.end+1)
	} else if m.pendingStart >= 0 {
		status = fmt.Sprintf("start: line %d", m.pendingStart+1)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", hintStyle.Render(fmt.Sprintf(
		"%s (%d/%d) · %s · ↑/↓: move · space: mark · enter: confirm",
		m.currentFile(), m.markingFileIdx+1, len(m.files), status)))

	m.vp.LeftGutterFunc = func(gc viewport.GutterContext) string {
		cursor, marker, number := " ", " ", " "

		if !gc.Soft {
			if gc.Index <= gc.TotalLines {
				number = strconv.Itoa(gc.Index + 1)
			} else {
				number = "~"
			}

			if gc.Index == m.lineCursor {
				cursor = ">"
			}

			if (gc.Index == m.pendingStart) || (done && gc.Index == picked.start) {
				marker = "S"
			}

			if done && gc.Index == picked.end {
				marker = "E"
			}
		}

		return fmt.Sprintf("%-1s%2s%4s %-2s", cursor, marker, number, "│")
	}

	m.vp.StyleLineFunc = func(i int) lipgloss.Style {
		if (done && i >= picked.start && i <= picked.end) || i == m.pendingStart {
			return highlight
		}
		return lipgloss.NewStyle()
	}

	b.WriteString(m.vp.View())
	return tea.NewView(b.String())
}
