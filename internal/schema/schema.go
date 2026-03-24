package schema

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type PoolSchema struct {
	Name              string               `yaml:"name"`
	Description       string               `yaml:"description"`
	RelayURL          string               `yaml:"relay_url"`
	OperatorPublicKey string               `yaml:"operator_public_key"`
	Profile           map[string]Attribute `yaml:"profile"`
	Roles             yaml.Node            `yaml:"roles"`
	Matching          yaml.Node            `yaml:"matching"`
	Indexing          *IndexingConfig      `yaml:"indexing,omitempty"`
}

// IndexingConfig defines how profiles are bucketed and encrypted for discovery.
type IndexingConfig struct {
	Partitions   []PartitionConfig `yaml:"partitions"`
	Permutations int               `yaml:"permutations"`
	Difficulty   int               `yaml:"difficulty"` // maps to nonce_space internally
}

// PartitionConfig defines how to partition profiles on one attribute.
type PartitionConfig struct {
	Field   string `yaml:"field"`
	Step    int    `yaml:"step,omitempty"`
	Overlap int    `yaml:"overlap,omitempty"`
}

type Attribute struct {
	Type       string `yaml:"type"`       // enum, multi, range, text
	Values     any    `yaml:"values"`     // string (file/csv) or []string
	Min        *int   `yaml:"min"`
	Max        *int   `yaml:"max"`
	Required   *bool  `yaml:"required"`   // default true
	Visibility string `yaml:"visibility"` // public (default) or private
}

func (a Attribute) IsRequired() bool {
	if a.Required == nil {
		return true
	}
	return *a.Required
}

func (a Attribute) IsPublic() bool {
	return a.Visibility == "" || a.Visibility == "public"
}

// ParsedRoles returns role names. If roles is a list, names only.
// If roles is a map, names with their role-specific attributes.
func (s *PoolSchema) ParsedRoles() ([]string, map[string]map[string]Attribute) {
	if s.Roles.Kind == 0 {
		return nil, nil
	}

	// Handle list: ["man", "woman"]
	if s.Roles.Kind == yaml.SequenceNode {
		var names []string
		for _, n := range s.Roles.Content {
			names = append(names, n.Value)
		}
		return names, nil
	}

	// Handle map: employer: {company: ...}, candidate: {skills: ...}
	if s.Roles.Kind == yaml.MappingNode {
		var names []string
		roleAttrs := make(map[string]map[string]Attribute)
		for i := 0; i < len(s.Roles.Content)-1; i += 2 {
			name := s.Roles.Content[i].Value
			names = append(names, name)

			var attrs map[string]Attribute
			if err := s.Roles.Content[i+1].Decode(&attrs); err == nil && len(attrs) > 0 {
				roleAttrs[name] = attrs
			}
		}
		if len(roleAttrs) == 0 {
			return names, nil
		}
		return names, roleAttrs
	}

	return nil, nil
}

// ResolveValues resolves attribute values — handles inline list, comma-separated string, and file reference.
func (a Attribute) ResolveValues(baseDir string) ([]string, error) {
	switch v := a.Values.(type) {
	case []any:
		var vals []string
		for _, item := range v {
			vals = append(vals, fmt.Sprint(item))
		}
		return vals, nil
	case string:
		// Check if it's a file path
		if strings.HasPrefix(v, "./") || strings.HasPrefix(v, "/") {
			return loadValuesFromFile(filepath.Join(baseDir, v))
		}
		// Comma-separated
		parts := strings.Split(v, ",")
		var vals []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				vals = append(vals, p)
			}
		}
		return vals, nil
	default:
		return nil, nil
	}
}
