package tui

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/i18n"
)

type (
	treeLoadedMsg struct {
		ref, path string
		entries   []domain.TreeEntry
	}
	fileLoadedMsg struct {
		ref  string
		file domain.FileContent
	}
	branchesMsg []string
	treeErrMsg  struct{ path string }
)

// node is one visible row of the tree.
type node struct {
	e     domain.TreeEntry
	depth int
}

type codeScreen struct {
	c       *repoCtx
	spinner spinner.Model
	vp      viewport.Model

	children map[string][]domain.TreeEntry // dir path -> entries ("" = root)
	expanded map[string]bool
	loading  map[string]bool
	rows     []node
	cursor   int
	top      int // first visible row

	file     *domain.FileContent
	fileName string
	loadingF bool
	branches []string

	w, h int

	open, parent, branch key.Binding
}

func newCodeScreen(c *repoCtx) *codeScreen {
	vp := viewport.New()
	vp.SoftWrap = false
	return &codeScreen{
		c: c, vp: vp, spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		children: map[string][]domain.TreeEntry{}, expanded: map[string]bool{}, loading: map[string]bool{},
		open: keyOpenItem, parent: bind("parent"), branch: bind("branch"),
	}
}

func (s *codeScreen) Title() string  { return s.c.repo.FullName() }
func (s *codeScreen) Header() string { return s.c.header() }
func (s *codeScreen) Tabs() ([]tab, int) {
	return s.c.tabs(), tabCode
}
func (s *codeScreen) SwitchTab(i int) tea.Cmd { return s.c.switchTo(i) }

func (s *codeScreen) Init() tea.Cmd {
	return tea.Batch(s.loadDir(""), s.c.loadCounts(), s.spinner.Tick)
}

func (s *codeScreen) ref() string { return s.c.branch }

func (s *codeScreen) loadDir(path string) tea.Cmd {
	if s.loading[path] {
		return nil
	}
	s.loading[path] = true
	p, repo, ref := s.c.p, s.c.repo, s.ref()
	return func() tea.Msg {
		es, err := p.ListTree(context.Background(), repo, ref, path)
		if err != nil {
			return errMsg{err}
		}
		return treeLoadedMsg{ref, path, es}
	}
}

// rebuild flattens the expanded tree into rows: directories first, then files.
func (s *codeScreen) rebuild() {
	var rows []node
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		es := append([]domain.TreeEntry(nil), s.children[dir]...)
		sort.SliceStable(es, func(i, j int) bool {
			if (es[i].Type == domain.EntryDir) != (es[j].Type == domain.EntryDir) {
				return es[i].Type == domain.EntryDir
			}
			return strings.ToLower(es[i].Name) < strings.ToLower(es[j].Name)
		})
		for _, e := range es {
			rows = append(rows, node{e, depth})
			if e.Type == domain.EntryDir && s.expanded[e.Path] {
				walk(e.Path, depth+1)
			}
		}
	}
	walk("", 0)
	s.rows = rows
	s.cursor = min(s.cursor, max(len(rows)-1, 0))
	s.scrollIntoView()
}

func (s *codeScreen) treeHeight() int { return max(s.h-1-2, 1) }

func (s *codeScreen) scrollIntoView() {
	hh := s.treeHeight()
	if s.cursor < s.top {
		s.top = s.cursor
	}
	if s.cursor >= s.top+hh {
		s.top = s.cursor - hh + 1
	}
	s.top = max(min(s.top, max(len(s.rows)-hh, 0)), 0)
}

func (s *codeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if applyCounts(msg) {
		return s, nil
	}
	switch msg := msg.(type) {
	case treeLoadedMsg:
		if msg.ref != s.ref() {
			return s, nil
		}
		delete(s.loading, msg.path)
		s.children[msg.path] = msg.entries
		s.rebuild()
		return s, nil
	case fileLoadedMsg:
		if msg.ref != s.ref() || msg.file.Path != s.fileName {
			return s, nil
		}
		s.loadingF = false
		f := msg.file
		s.file = &f
		s.renderFile()
		return s, nil
	case branchesMsg:
		s.branches = msg
		return s, s.cycleBranch()
	case errMsg:
		s.loadingF = false
		clear(s.loading)
		return s, nil
	case spinner.TickMsg:
		if !s.busy() {
			return s, nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	case tea.KeyPressMsg:
		if cmd, ok := s.handleKey(msg); ok {
			return s, cmd
		}
	}
	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return s, cmd
}

func (s *codeScreen) busy() bool { return s.loadingF || len(s.loading) > 0 }

func (s *codeScreen) selected() (node, bool) {
	if s.cursor < 0 || s.cursor >= len(s.rows) {
		return node{}, false
	}
	return s.rows[s.cursor], true
}

func (s *codeScreen) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch k := msg.String(); {
	case k == "j" || k == "down":
		s.cursor = min(s.cursor+1, max(len(s.rows)-1, 0))
		s.scrollIntoView()
		return nil, true
	case k == "k" || k == "up":
		s.cursor = max(s.cursor-1, 0)
		s.scrollIntoView()
		return nil, true
	case key.Matches(msg, s.open) || k == "l" || k == "right":
		return s.activate(), true
	case key.Matches(msg, s.parent) || k == "left":
		return s.goParent(), true
	case key.Matches(msg, s.branch):
		if len(s.branches) == 0 {
			p, repo := s.c.p, s.c.repo
			return func() tea.Msg {
				bs, err := p.ListBranches(context.Background(), repo)
				if err != nil {
					return errMsg{err}
				}
				return branchesMsg(bs)
			}, true
		}
		return s.cycleBranch(), true
	case key.Matches(msg, keyBrowser):
		url := s.c.repo.URL
		if n, ok := s.selected(); ok && url != "" {
			kind := "blob"
			if n.e.Type == domain.EntryDir {
				kind = "tree"
			}
			url = fmt.Sprintf("%s/%s/%s/%s", url, kind, s.ref(), n.e.Path)
		}
		return openInBrowser(url), true
	case key.Matches(msg, keyRefresh):
		return s.reset(), true
	}
	return nil, false
}

// activate expands/collapses a directory or opens a file.
func (s *codeScreen) activate() tea.Cmd {
	n, ok := s.selected()
	if !ok {
		return nil
	}
	if n.e.Type == domain.EntryDir {
		s.expanded[n.e.Path] = !s.expanded[n.e.Path]
		var cmd tea.Cmd
		if _, have := s.children[n.e.Path]; s.expanded[n.e.Path] && !have {
			cmd = tea.Batch(s.loadDir(n.e.Path), s.spinner.Tick)
		}
		s.rebuild()
		return cmd
	}
	s.fileName, s.file, s.loadingF = n.e.Path, nil, true
	s.vp.SetContent("")
	p, repo, ref, path := s.c.p, s.c.repo, s.ref(), n.e.Path
	load := func() tea.Msg {
		f, err := p.GetFile(context.Background(), repo, ref, path)
		if err != nil {
			return errMsg{err}
		}
		return fileLoadedMsg{ref, f}
	}
	return tea.Batch(load, s.spinner.Tick)
}

// goParent collapses the open directory, or moves the cursor to the parent.
func (s *codeScreen) goParent() tea.Cmd {
	n, ok := s.selected()
	if !ok {
		return nil
	}
	if n.e.Type == domain.EntryDir && s.expanded[n.e.Path] {
		s.expanded[n.e.Path] = false
		s.rebuild()
		return nil
	}
	for i := s.cursor - 1; i >= 0; i-- {
		if s.rows[i].depth < n.depth {
			s.cursor = i
			s.scrollIntoView()
			return nil
		}
	}
	return nil
}

func (s *codeScreen) cycleBranch() tea.Cmd {
	if len(s.branches) == 0 {
		return nil
	}
	next := s.branches[0]
	for i, b := range s.branches {
		if b == s.c.branch {
			next = s.branches[(i+1)%len(s.branches)]
		}
	}
	s.c.branch = next
	return tea.Batch(s.reset(), info(i18n.T("branch: %s", next)))
}

// reset forgets everything loaded and reloads the root.
func (s *codeScreen) reset() tea.Cmd {
	s.children = map[string][]domain.TreeEntry{}
	s.expanded = map[string]bool{}
	s.loading = map[string]bool{}
	s.rows, s.cursor, s.top = nil, 0, 0
	s.file, s.fileName, s.loadingF = nil, "", false
	s.vp.SetContent("")
	return tea.Batch(s.loadDir(""), s.spinner.Tick)
}

func (s *codeScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	_, rw := s.widths()
	s.vp.SetWidth(max(rw-4, 1))
	s.vp.SetHeight(max(h-2-2, 1)) // pane borders + file header + rule
	s.scrollIntoView()
	if s.file != nil {
		s.renderFile()
	}
}

func (s *codeScreen) widths() (int, int) {
	if !canSplit(s.w) {
		return s.w, 0
	}
	return splitWidths(s.w, s.w*30/100)
}

// highlight colors source for the file name, with line numbers.
func highlight(name, src string, dark bool) string {
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	numW := len(fmt.Sprint(len(lines)))
	var body []string
	lexer := lexers.Match(name)
	if lexer != nil {
		style := styles.Get("github")
		if dark {
			style = styles.Get("dracula")
		}
		if style == nil {
			style = styles.Fallback
		}
		var buf bytes.Buffer
		if it, err := chroma.Coalesce(lexer).Tokenise(nil, strings.TrimRight(src, "\n")); err == nil &&
			formatters.TTY16m.Format(&buf, style, it) == nil {
			body = strings.Split(buf.String(), "\n")
		}
	}
	if len(body) != len(lines) {
		body = lines
	}
	out := make([]string, len(lines))
	for i, l := range body {
		out[i] = dimStyle.Render(fmt.Sprintf("%*d ", numW, i+1)) + strings.ReplaceAll(l, "\t", "    ")
	}
	return strings.Join(out, "\n")
}

func (s *codeScreen) renderFile() {
	if s.file == nil || s.file.Binary || s.file.TooLarge {
		s.vp.SetContent("")
		return
	}
	s.vp.SetContent(highlight(s.file.Path, s.file.Content, isDark))
	s.vp.GotoTop()
}

func (s *codeScreen) View() string {
	lw, rw := s.widths()
	ph := max(s.h, 3)
	tree := pinkStyle.Bold(true).Render(i18n.T("Files")) + "\n" +
		dimStyle.Render(fit(s.c.repo.Name+s.dirSuffix(), lw-4)) + "\n" + s.treeView(lw-4)
	if rw == 0 {
		if s.file != nil || s.loadingF {
			return pane(s.fileView(lw-4), lw, ph, true)
		}
		return pane(tree, lw, ph, true)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, pane(tree, lw, ph, true), pane(s.fileView(rw-4), rw, ph, false))
}

func (s *codeScreen) dirSuffix() string {
	if n, ok := s.selected(); ok {
		if i := strings.LastIndex(n.e.Path, "/"); i > 0 {
			return " / " + n.e.Path[:i]
		}
	}
	return ""
}

func (s *codeScreen) treeView(w int) string {
	if len(s.rows) == 0 {
		if s.loading[""] {
			return s.spinner.View() + " " + i18n.T("loading…")
		}
		return dimStyle.Render(i18n.T("Empty repository."))
	}
	hh := max(s.h-2-2, 1)
	end := min(s.top+hh, len(s.rows))
	lines := make([]string, 0, end-s.top)
	for i := s.top; i < end; i++ {
		lines = append(lines, s.row(s.rows[i], i == s.cursor, w))
	}
	return strings.Join(lines, "\n")
}

func (s *codeScreen) row(n node, sel bool, w int) string {
	indent := strings.Repeat("  ", n.depth)
	var label, size string
	if n.e.Type == domain.EntryDir {
		mark := "▸ "
		if s.expanded[n.e.Path] {
			mark = "▾ "
		}
		label = indent + mark + n.e.Name + "/"
		if sel {
			return selectLine(label, w, true)
		}
		return blueStyle.Render(fit(label, w))
	}
	label, size = indent+"  "+n.e.Name, humanBytes(n.e.Size)
	line := fit(label, max(w-len(size)-1, 1)) + " " + size
	if sel {
		return selectLine(line, w, true)
	}
	return fit(label, max(w-len(size)-1, 1)) + " " + dimStyle.Render(size)
}

func humanBytes(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1f KB", float64(n)/1000)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/1e6)
}

func (s *codeScreen) fileView(w int) string {
	if s.loadingF {
		return s.spinner.View() + " " + i18n.T("loading the file…")
	}
	if s.file == nil {
		return dimStyle.Render(i18n.T("Select a file (%s) to display it.", keyOpenItem.Help().Key))
	}
	f := s.file
	name := pinkStyle.Bold(true).Render(f.Path[strings.LastIndex(f.Path, "/")+1:])
	meta := fmt.Sprintf("%s · %s", languageOf(f.Path), humanBytes(f.Size))
	if !f.Binary && !f.TooLarge {
		meta = fmt.Sprintf("%s · %s · %s", languageOf(f.Path), i18n.T("%d lines", strings.Count(strings.TrimRight(f.Content, "\n"), "\n")+1), humanBytes(f.Size))
	}
	head := name + "  " + dimStyle.Render(meta)
	if f.CommitSHA != "" {
		sha := f.CommitSHA[:min(len(f.CommitSHA), 7)]
		head += "  " + dimStyle.Render(i18n.T("last commit")+" ") + yellowStyle.Render(sha) + dimStyle.Render(" "+ilYa(f.CommitTime))
	}
	var body string
	switch {
	case f.Binary:
		body = dimStyle.Render(i18n.T("Binary file: preview unavailable."))
	case f.TooLarge:
		body = dimStyle.Render(i18n.T("File too large to preview (%s to open it in the browser).", keyBrowser.Help().Key))
	default:
		body = s.vp.View() + "\n" + dimStyle.Render(fitRight(fmt.Sprintf("%.0f%%", s.vp.ScrollPercent()*100), w))
	}
	return ansiTrunc(head, w) + "\n" + rule(w, "") + "\n" + body
}

func ansiTrunc(s string, w int) string { return lipgloss.NewStyle().MaxWidth(w).Render(s) }

func languageOf(path string) string {
	if l := lexers.Match(path); l != nil {
		return l.Config().Name
	}
	return i18n.T("text")
}

func (s *codeScreen) Capturing() bool { return false }
func (s *codeScreen) Keys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("j"), key.WithHelp("j/k", i18n.T("navigate"))),
		s.open, s.parent, s.branch, keyBrowser, keySwitch, keyRefresh,
	}
}
