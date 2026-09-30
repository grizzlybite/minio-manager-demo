package config_test

import (
	"encoding/json"
	"testing"

	"github.com/yourorg/minio-manager/internal/config"
	"github.com/yourorg/minio-manager/internal/state"
)

func TestFromStateRoundTrip(t *testing.T) {
	t.Parallel()

	// Build desired state from a config, render policy documents the way the
	// apply path does, then import it back and assert it matches.
	original := config.Config{
		Policies: []config.Policy{{
			Name: "rw",
			Statements: []config.Statement{{
				Effect:    "Allow",
				Actions:   []string{"s3:GetObject", "s3:PutObject"},
				Resources: []string{"arn:aws:s3:::b", "arn:aws:s3:::b/*"},
			}},
		}},
	}
	desired := original.ToDesired()

	cur := state.CurrentState{
		Buckets: []state.BucketInfo{{Name: "b", Region: "us-east-1", Versioning: true, Tags: map[string]string{"env": "prod"}}},
		Users:   []state.UserInfo{{Name: "alice", Enabled: true, Policies: []string{"rw"}}},
		Policies: []state.PolicyInfo{
			{Name: "rw", Document: desired.Policies[0].Document},
			{Name: "readwrite", Document: json.RawMessage(`{"Statement":[]}`)}, // built-in
		},
	}

	cfg, err := config.FromState(cur, false)
	if err != nil {
		t.Fatalf("FromState: %v", err)
	}

	// Built-in policy filtered out.
	if len(cfg.Policies) != 1 || cfg.Policies[0].Name != "rw" {
		t.Fatalf("policies = %+v, want only [rw]", cfg.Policies)
	}
	got := cfg.Policies[0].Statements
	if len(got) != 1 || got[0].Effect != "Allow" ||
		len(got[0].Actions) != 2 || got[0].Actions[0] != "s3:GetObject" ||
		len(got[0].Resources) != 2 {
		t.Errorf("statements round-trip mismatch: %+v", got)
	}

	// Bucket and user mapping; password must be empty (cannot be exported).
	if len(cfg.Buckets) != 1 || !cfg.Buckets[0].Versioning || cfg.Buckets[0].Tags["env"] != "prod" {
		t.Errorf("bucket mapping wrong: %+v", cfg.Buckets)
	}
	if len(cfg.Users) != 1 || cfg.Users[0].Password != "" || !cfg.Users[0].Enabled {
		t.Errorf("user mapping wrong: %+v", cfg.Users)
	}

	// Including built-ins keeps them.
	withBuiltin, err := config.FromState(cur, true)
	if err != nil {
		t.Fatalf("FromState(includeBuiltin): %v", err)
	}
	if len(withBuiltin.Policies) != 2 {
		t.Errorf("expected 2 policies with built-ins, got %d", len(withBuiltin.Policies))
	}
}

func TestFromStateScalarActionResource(t *testing.T) {
	t.Parallel()

	// MinIO may return Action/Resource as a single string rather than an array.
	cur := state.CurrentState{
		Policies: []state.PolicyInfo{{
			Name:     "single",
			Document: json.RawMessage(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::b/*"}]}`),
		}},
	}

	cfg, err := config.FromState(cur, false)
	if err != nil {
		t.Fatalf("FromState: %v", err)
	}
	st := cfg.Policies[0].Statements[0]
	if len(st.Actions) != 1 || st.Actions[0] != "s3:GetObject" {
		t.Errorf("Actions = %v, want [s3:GetObject]", st.Actions)
	}
	if len(st.Resources) != 1 || st.Resources[0] != "arn:aws:s3:::b/*" {
		t.Errorf("Resources = %v, want [arn:aws:s3:::b/*]", st.Resources)
	}
}

// TestImportedConfigValidates ensures an exported config (once passwords are
// supplied) passes validation, i.e. import output is round-trip compatible with
// the apply path.
func TestImportedConfigValidates(t *testing.T) {
	t.Parallel()

	cur := state.CurrentState{
		Buckets: []state.BucketInfo{{Name: "data-bucket", Region: "us-east-1"}},
		Users:   []state.UserInfo{{Name: "alice", Enabled: true, Policies: []string{"rw"}}},
		Policies: []state.PolicyInfo{{
			Name:     "rw",
			Document: json.RawMessage(`{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::data-bucket/*"]}]}`),
		}},
	}
	cfg, err := config.FromState(cur, false)
	if err != nil {
		t.Fatalf("FromState: %v", err)
	}

	// Supply the password that import cannot export.
	cfg.Users[0].Password = "filled-in"

	if err := config.Validate(&cfg, "imported"); err != nil {
		t.Fatalf("imported config failed validation: %v", err)
	}

	// And it marshals to YAML.
	if _, err := config.Marshal(cfg); err != nil {
		t.Fatalf("Marshal: %v", err)
	}
}
