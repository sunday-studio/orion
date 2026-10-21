package cli

import (
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"
)

//go:embed topics/*.md
var topicFiles embed.FS

// helpTopics are the longer pages behind `orion-node help <topic>`.
func helpTopicNames() []string {
	entries, _ := topicFiles.ReadDir("topics")
	var names []string
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(names)
	return names
}

// printHelpTopic writes the topic page, formatted when the terminal can show
// it. ok is false when name is not a topic, so the normal command help runs.
func printHelpTopic(w io.Writer, name string, width int) (ok bool) {
	if name == "topics" {
		fmt.Fprintln(w, "\nHelp topics (orion-node help <topic>):")
		for _, n := range helpTopicNames() {
			fmt.Fprintln(w, "  "+n)
		}
		fmt.Fprintln(w)
		return true
	}
	data, err := topicFiles.ReadFile("topics/" + name + ".md")
	if err != nil {
		return false
	}
	fmt.Fprint(w, "\n"+renderMarkdown(theme, string(data), width)+"\n")
	return true
}
