package config

import (
	"encoding/json"
	"fmt"

	"github.com/yourorg/minio-manager/internal/state"
	"gopkg.in/yaml.v3"
)

// FromState builds a Config from observed MinIO state — the inverse of the
// apply pipeline, used by the `import` command to bootstrap a config from a
// running cluster.
//
// User secret keys cannot be read back from MinIO, so every user's Password is
// left empty and must be filled in (or replaced with a vault:// reference)
// before the config is applied. Built-in MinIO policies are skipped unless
// includeBuiltin is set; user references to them remain valid because the
// validator recognizes built-in policy names.
func FromState(cur state.CurrentState, includeBuiltin bool) (Config, error) {
	cfg := Config{}

	for _, b := range cur.Buckets {
		cfg.Buckets = append(cfg.Buckets, Bucket{
			Name:       b.Name,
			Region:     b.Region,
			Versioning: b.Versioning,
			Tags:       b.Tags,
		})
	}

	for _, u := range cur.Users {
		cfg.Users = append(cfg.Users, User{
			Name:     u.Name,
			Password: "", // secret keys are never returned by MinIO
			Policies: u.Policies,
			Enabled:  u.Enabled,
		})
	}

	for _, p := range cur.Policies {
		if !includeBuiltin && builtinPolicies[p.Name] {
			continue
		}
		stmts, err := parseIAMDocument(p.Document)
		if err != nil {
			return Config{}, fmt.Errorf("policy %q: %w", p.Name, err)
		}
		cfg.Policies = append(cfg.Policies, Policy{Name: p.Name, Statements: stmts})
	}

	return cfg, nil
}

// Marshal renders the config as YAML matching the configs/example.yaml schema.
func Marshal(c Config) ([]byte, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return b, nil
}

// stringOrSlice unmarshals an IAM "Action"/"Resource" field that may be encoded
// either as a single string or as an array of strings.
type stringOrSlice []string

func (s *stringOrSlice) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("expected string or array of strings: %w", err)
	}
	*s = []string{single}
	return nil
}

// iamDocumentIn mirrors the canonical IAM policy document for decoding.
type iamDocumentIn struct {
	Statement []struct {
		Effect   string        `json:"Effect"`
		Action   stringOrSlice `json:"Action"`
		Resource stringOrSlice `json:"Resource"`
	} `json:"Statement"`
}

// parseIAMDocument decodes an IAM policy document into config statements. It is
// the inverse of buildIAMDocument and tolerates the string-or-array shapes that
// MinIO may return.
func parseIAMDocument(doc json.RawMessage) ([]Statement, error) {
	if len(doc) == 0 {
		return nil, nil
	}
	var in iamDocumentIn
	if err := json.Unmarshal(doc, &in); err != nil {
		return nil, fmt.Errorf("decode IAM document: %w", err)
	}

	stmts := make([]Statement, 0, len(in.Statement))
	for _, s := range in.Statement {
		stmts = append(stmts, Statement{
			Effect:    s.Effect,
			Actions:   s.Action,
			Resources: s.Resource,
		})
	}
	return stmts, nil
}
