package cli

import "gopkg.in/yaml.v3"

// editStoragePaths adds paths to app_resources.paths, skipping ones already
// there. It reports whether anything changed.
func editStoragePaths(configPath string, paths []string) (bool, error) {
	return editConfigYAML(configPath, func(root *yaml.Node) bool {
		res := mapChild(root, "app_resources", yaml.MappingNode, true)
		list := mapChild(res, "paths", yaml.SequenceNode, true)
		have := map[string]bool{}
		for _, n := range list.Content {
			have[n.Value] = true
		}
		changed := false
		for _, p := range paths {
			if !have[p] {
				have[p] = true
				list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: p})
				changed = true
			}
		}
		return changed
	})
}
