package config

import (
	"encoding/json"

	"github.com/yourorg/minio-manager/internal/state"
)

// iamDocument is the canonical AWS/MinIO IAM policy document shape produced from
// the config's policy statements.
type iamDocument struct {
	Version   string         `json:"Version"`
	Statement []iamStatement `json:"Statement"`
}

type iamStatement struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

// ToDesired maps the parsed configuration into the desired-state model consumed
// by the reconciler. Policy statements are rendered into canonical IAM policy
// JSON documents.
func (c *Config) ToDesired() state.DesiredState {
	ds := state.DesiredState{
		Buckets:  make([]state.BucketSpec, 0, len(c.Buckets)),
		Users:    make([]state.UserSpec, 0, len(c.Users)),
		Policies: make([]state.PolicySpec, 0, len(c.Policies)),
	}

	for _, b := range c.Buckets {
		ds.Buckets = append(ds.Buckets, state.BucketSpec{
			Name:          b.Name,
			Region:        b.Region,
			Versioning:    b.Versioning,
			ObjectLocking: b.ObjectLocking,
			QuotaGB:       b.QuotaGB,
			LifecycleDays: b.LifecycleDays,
			Tags:          b.Tags,
			Line:          b.LineNumber,
		})
	}
	for _, u := range c.Users {
		ds.Users = append(ds.Users, state.UserSpec{
			Name:     u.Name,
			Password: u.Password,
			Policies: u.Policies,
			Enabled:  u.Enabled,
			Line:     u.LineNumber,
		})
	}
	for _, p := range c.Policies {
		ds.Policies = append(ds.Policies, state.PolicySpec{
			Name:     p.Name,
			Document: buildIAMDocument(p.Statements),
			Line:     p.LineNumber,
		})
	}
	return ds
}

// buildIAMDocument renders policy statements into canonical IAM policy JSON.
func buildIAMDocument(stmts []Statement) json.RawMessage {
	doc := iamDocument{
		Version:   "2012-10-17",
		Statement: make([]iamStatement, 0, len(stmts)),
	}
	for _, s := range stmts {
		doc.Statement = append(doc.Statement, iamStatement{
			Effect:   s.Effect,
			Action:   s.Actions,
			Resource: s.Resources,
		})
	}
	b, err := json.Marshal(doc)
	if err != nil {
		// The document is built from plain strings and slices, so marshaling
		// cannot realistically fail; fall back to an empty object.
		return json.RawMessage("{}")
	}
	return b
}
