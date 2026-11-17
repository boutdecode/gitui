package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider/fake"
)

func loadedCode(t *testing.T) *codeScreen {
	t.Helper()
	p := fake.New()
	c := newRepoCtx(p, domain.Repo{Owner: "o", Name: "r", DefaultBranch: "main"})
	s := newCodeScreen(c)
	s.SetSize(120, 24)
	root, err := p.ListTree(context.Background(), c.repo, "main", "")
	if err != nil {
		t.Fatal(err)
	}
	s.Update(treeLoadedMsg{"main", "", root})
	return s
}

func tap(s screen, k string) tea.Cmd {
	_, cmd := s.Update(tea.KeyPressMsg{Code: rune(k[0]), Text: k})
	return cmd
}

func TestCodeTreeOrdersDirsFirstAndExpands(t *testing.T) {
	s := loadedCode(t)
	if len(s.rows) < 3 || s.rows[0].e.Type != domain.EntryDir {
		t.Fatalf("directories must come first: %+v", s.rows)
	}
	for i, r := range s.rows {
		if r.e.Type == domain.EntryFile {
			for _, later := range s.rows[i:] {
				if later.e.Type == domain.EntryDir {
					t.Fatal("a directory follows a file at the same level")
				}
			}
			break
		}
	}
	before := len(s.rows)
	// move to "examples" and open it: children load lazily
	for s.rows[s.cursor].e.Path != "examples" {
		tap(s, "j")
	}
	cmd := s.activate()
	if cmd == nil || !s.expanded["examples"] {
		t.Fatal("expanding an unloaded directory must trigger a load")
	}
	entries, _ := fake.New().ListTree(context.Background(), s.c.repo, "main", "examples")
	s.Update(treeLoadedMsg{"main", "examples", entries})
	if len(s.rows) != before+len(entries) {
		t.Fatalf("rows = %d, want %d", len(s.rows), before+len(entries))
	}
	// h collapses an open directory
	tap(s, "h")
	if s.expanded["examples"] || len(s.rows) != before {
		t.Fatal("h should collapse the open directory")
	}
}

func TestCodeParentMovesCursorUp(t *testing.T) {
	s := loadedCode(t)
	for s.rows[s.cursor].e.Path != "examples" {
		tap(s, "j")
	}
	s.activate()
	entries, _ := fake.New().ListTree(context.Background(), s.c.repo, "main", "examples")
	s.Update(treeLoadedMsg{"main", "examples", entries})
	tap(s, "j") // first child
	if s.rows[s.cursor].depth != 1 {
		t.Fatalf("cursor on %+v", s.rows[s.cursor])
	}
	tap(s, "h")
	if s.rows[s.cursor].e.Path != "examples" {
		t.Fatalf("h on a file should select its parent, got %q", s.rows[s.cursor].e.Path)
	}
}

func TestCodeOpenFileShowsNumberedContent(t *testing.T) {
	s := loadedCode(t)
	p := fake.New()
	f, _ := p.GetFile(context.Background(), s.c.repo, "main", "go.mod")
	s.fileName = f.Path
	s.Update(fileLoadedMsg{"main", f})
	v := plain(s.View())
	for _, want := range []string{"go.mod", "module example.com/hello", "1 ", "dernier commit"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
}

func TestCodeBinaryAndStaleFiles(t *testing.T) {
	s := loadedCode(t)
	s.fileName = "assets/logo.png"
	s.Update(fileLoadedMsg{"main", domain.FileContent{Path: "assets/logo.png", Size: 12, Binary: true}})
	if v := plain(s.View()); !strings.Contains(v, "binaire") {
		t.Fatalf("binary file should be flagged:\n%s", v)
	}
	// a result for a file we've since left must be ignored
	s.fileName = "other"
	s.Update(fileLoadedMsg{"main", domain.FileContent{Path: "late.go", Content: "package x"}})
	if s.file != nil && s.file.Path == "late.go" {
		t.Fatal("stale file result must be dropped")
	}
}

func TestCodeBranchCycleResetsTree(t *testing.T) {
	s := loadedCode(t)
	s.Update(branchesMsg{"main", "dev"})
	if s.c.branch != "dev" {
		t.Fatalf("branch = %q, want dev", s.c.branch)
	}
	if len(s.rows) != 0 {
		t.Fatal("changing branch must clear the tree")
	}
	// a tree loaded for the old branch arriving late is ignored
	root, _ := fake.New().ListTree(context.Background(), s.c.repo, "main", "")
	s.Update(treeLoadedMsg{"main", "", root})
	if len(s.rows) != 0 {
		t.Fatal("tree of the previous branch must be dropped")
	}
}

func TestHighlightKeepsLineCount(t *testing.T) {
	src := "package main\n\nfunc main() {}\n"
	out := plain(highlight("main.go", src, true))
	if lines := strings.Split(out, "\n"); len(lines) != 3 || !strings.HasPrefix(lines[2], "3 ") {
		t.Fatalf("highlight output:\n%q", out)
	}
	// unknown extension falls back to plain text, still numbered
	if out := plain(highlight("notes.zzz", "a\nb", false)); !strings.Contains(out, "2 b") {
		t.Fatalf("fallback output: %q", out)
	}
}
