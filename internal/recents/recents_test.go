package recents

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevin/gitui/internal/domain"
)

func repo(n string) domain.Repo { return domain.Repo{Owner: "o", Name: n} }

func TestAddMovesToFrontWithoutDuplicates(t *testing.T) {
	s := New("")
	for _, n := range []string{"a", "b", "c", "a"} {
		if err := s.Add(repo(n)); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, r := range s.List() {
		got = append(got, r.Name)
	}
	if fmt.Sprint(got) != "[a c b]" {
		t.Fatalf("order = %v, want [a c b]", got)
	}
}

func TestCappedAtMax(t *testing.T) {
	s := New("")
	for i := 0; i < Max+5; i++ {
		_ = s.Add(repo(fmt.Sprint(i)))
	}
	if n := len(s.List()); n != Max {
		t.Fatalf("len = %d, want %d", n, Max)
	}
}

func TestPersistsAcrossStores(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "recent.json")
	if err := New(p).Add(repo("x")); err != nil {
		t.Fatal(err)
	}
	got := New(p).List()
	if len(got) != 1 || got[0].Name != "x" {
		t.Fatalf("reloaded = %+v", got)
	}
	if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, %v", fi, err)
	}
}

func TestCorruptFileIsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "recent.json")
	_ = os.WriteFile(p, []byte("{not json"), 0o600)
	s := New(p)
	if len(s.List()) != 0 {
		t.Fatal("corrupt file should read as empty")
	}
	if err := s.Add(repo("y")); err != nil || len(s.List()) != 1 {
		t.Fatalf("add after corrupt: %v", err)
	}
}
