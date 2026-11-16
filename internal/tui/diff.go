package tui

import "strings"

// colorizeDiff styles a unified diff line by line. File headers
// (---/+++) are matched before add/delete so they don't look like changes.
func colorizeDiff(d string) string {
	lines := strings.Split(strings.TrimRight(d, "\n"), "\n")
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "diff --git"):
			lines[i] = diffFile.Render(l)
		case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"),
			strings.HasPrefix(l, "index "), strings.HasPrefix(l, "new file"),
			strings.HasPrefix(l, "deleted file"), strings.HasPrefix(l, "similarity"),
			strings.HasPrefix(l, "rename "):
			lines[i] = diffMeta.Render(l)
		case strings.HasPrefix(l, "@@"):
			lines[i] = diffHunk.Render(l)
		case strings.HasPrefix(l, "+"):
			lines[i] = diffAdd.Render(l)
		case strings.HasPrefix(l, "-"):
			lines[i] = diffDel.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}
