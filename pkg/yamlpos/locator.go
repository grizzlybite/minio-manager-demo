// Package yamlpos locates source line numbers of nodes inside a YAML document.
//
// It is built on gopkg.in/yaml.v3 (NOT v2): only v3 exposes yaml.Node with the
// Line and Column fields required to attach a config-file position to every
// element of the buckets/users/policies sequences.
package yamlpos

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Parse decodes raw YAML into a document node tree suitable for line lookups.
// The returned node is the document's root content node (the top-level mapping
// for a typical config), already unwrapped from the surrounding DocumentNode.
func Parse(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yamlpos: decode document: %w", err)
	}
	return unwrapDocument(&doc), nil
}

// unwrapDocument returns the single content node of a DocumentNode, or the node
// itself if it is not a document wrapper. A nil, empty or zero-kind node (as
// produced by decoding empty input) yields nil.
func unwrapDocument(n *yaml.Node) *yaml.Node {
	if n == nil || n.Kind == 0 {
		return nil
	}
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil
		}
		return n.Content[0]
	}
	return n
}

// LineOfKey walks the node tree following path and returns the 1-based source
// line of the located value node, or 0 if the path cannot be resolved.
//
// Each path segment is interpreted against the current node:
//   - on a MappingNode the segment is matched against the mapping's keys;
//   - on a SequenceNode the segment is parsed as a 0-based index.
//
// Example: LineOfKey(root, "users", "1", "password") returns the line of the
// password value of the second user.
func LineOfKey(root *yaml.Node, path ...string) int {
	n := find(root, path)
	if n == nil {
		return 0
	}
	return n.Line
}

// find resolves path against root, returning the addressed node or nil.
func find(root *yaml.Node, path []string) *yaml.Node {
	cur := unwrapDocument(root)
	for _, seg := range path {
		if cur == nil {
			return nil
		}
		switch cur.Kind {
		case yaml.MappingNode:
			cur = mappingValue(cur, seg)
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(cur.Content) {
				return nil
			}
			cur = cur.Content[idx]
		default:
			return nil
		}
	}
	return cur
}

// mappingValue returns the value node for key in a MappingNode, or nil.
// MappingNode content is a flat slice of alternating key/value nodes.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// SequenceLines returns the source line of every element of the top-level
// sequence stored under key (e.g. "buckets", "users", "policies"). The result
// is indexed in document order, so SequenceLines(root, "users")[i] is the line
// of the i-th user. Returns nil if key is absent or not a sequence.
func SequenceLines(root *yaml.Node, key string) []int {
	seq := find(unwrapDocument(root), []string{key})
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	lines := make([]int, len(seq.Content))
	for i, el := range seq.Content {
		lines[i] = el.Line
	}
	return lines
}
