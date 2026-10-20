package cli

import (
	"strings"

	"orion/agent/internal/config"

	"gopkg.in/yaml.v3"

	"orion/agent/internal/ui"
)

// diffOp is one line of an edit script: ' ' kept, '-' removed, '+' added.
type diffOp struct {
	Kind byte
	Text string
	Old  int // 1-based line in the old text, 0 for an added line
	New  int // 1-based line in the new text, 0 for a removed line
}

// editScript returns the LCS-based edit script between two line lists.
func editScript(a, b []string) []diffOp {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i], i + 1, j + 1})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{'-', a[i], i + 1, 0})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j], 0, j + 1})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i], i + 1, 0})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j], 0, j + 1})
	}
	return ops
}

// yamlPaths maps each line of a YAML document to where it lives: the
// top-level key, plus the item's name inside a list ("monitors.redis").
func yamlPaths(data []byte) map[int]string {
	paths := map[int]string{}
	var root yaml.Node
	if yaml.Unmarshal(data, &root) != nil || len(root.Content) == 0 {
		return paths
	}
	var walk func(n *yaml.Node, path string)
	walk = func(n *yaml.Node, path string) {
		paths[n.Line] = path
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				p := path
				if p == "" {
					p = n.Content[i].Value
				}
				paths[n.Content[i].Line] = p
				walk(n.Content[i+1], p)
			}
		case yaml.SequenceNode:
			for _, item := range n.Content {
				p := path
				if item.Kind == yaml.MappingNode {
					for i := 0; i+1 < len(item.Content); i += 2 {
						if item.Content[i].Value == "name" {
							p = path + "." + item.Content[i+1].Value
						}
					}
				}
				walk(item, p)
			}
		}
	}
	walk(root.Content[0], "")
	return paths
}

// reviewHunks groups changed lines under "@@ path @@" headers, using the old
// file's path for removals and the new file's for additions.
func reviewHunks(th *ui.Theme, oldData, newData []byte) string {
	split := func(b []byte) []string { return strings.Split(strings.TrimRight(string(b), "\n"), "\n") }
	oldPaths, newPaths := yamlPaths(oldData), yamlPaths(newData)
	var b strings.Builder
	last := "\x00"
	for _, op := range editScript(split(oldData), split(newData)) {
		if op.Kind == ' ' {
			continue
		}
		path := newPaths[op.New]
		if op.Kind == '-' {
			path = oldPaths[op.Old]
		}
		if path == "" {
			path = "(top level)"
		}
		if path != last {
			b.WriteString("   " + th.Muted("@@ "+path+" @@") + "\n")
			last = path
		}
		text := strings.TrimSpace(config.RedactLine(op.Text))
		if op.Kind == '-' {
			b.WriteString("   " + th.Tone("down", "- "+text) + "\n")
		} else {
			b.WriteString("   " + th.Tone("up", "+ "+text) + "\n")
		}
	}
	return b.String()
}
