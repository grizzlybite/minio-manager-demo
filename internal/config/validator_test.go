package config_test

import (
	"strings"
	"testing"

	"github.com/yourorg/minio-manager/internal/config"
	apperrors "github.com/yourorg/minio-manager/internal/errors"
)

// validBucket/validUser/validPolicy return minimal valid building blocks so a
// test can introduce exactly one defect and assert on the single resulting
// error.
func validBucket() config.Bucket {
	return config.Bucket{Name: "my-bucket", Region: "us-east-1", LineNumber: 10}
}

func validUser() config.User {
	return config.User{Name: "alice", Password: "s3cr3t", Policies: []string{"p1"}, Enabled: true, LineNumber: 20}
}

func validPolicy() config.Policy {
	return config.Policy{
		Name:       "p1",
		LineNumber: 30,
		Statements: []config.Statement{{
			Effect:    "Allow",
			Actions:   []string{"s3:GetObject"},
			Resources: []string{"arn:aws:s3:::my-bucket/*"},
		}},
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       config.Config
		wantField string // substring of the expected ConfigError.Field ("" => no error)
		wantLine  int
		wantMsg   string // substring of the expected message
	}{
		{
			name: "valid config",
			cfg:  config.Config{Buckets: []config.Bucket{validBucket()}, Users: []config.User{validUser()}, Policies: []config.Policy{validPolicy()}},
		},
		{
			name: "invalid bucket name",
			cfg: config.Config{Buckets: []config.Bucket{
				{Name: "AB", LineNumber: 10},
			}},
			wantField: "buckets[0].name", wantLine: 10, wantMsg: "invalid bucket name",
		},
		{
			name: "duplicate bucket name",
			cfg: config.Config{Buckets: []config.Bucket{
				{Name: "dup", LineNumber: 10},
				{Name: "dup", LineNumber: 14},
			}},
			wantField: "buckets[1].name", wantLine: 14, wantMsg: "duplicate",
		},
		{
			name: "negative quota",
			cfg: config.Config{Buckets: []config.Bucket{
				{Name: "my-bucket", QuotaGB: -1, LineNumber: 11},
			}},
			wantField: "buckets[0].quota_gb", wantLine: 11, wantMsg: ">= 0",
		},
		{
			name:      "empty user password",
			cfg:       config.Config{Users: []config.User{{Name: "bob", Password: "", LineNumber: 22}}},
			wantField: "users[0].password", wantLine: 22, wantMsg: "password is required",
		},
		{
			name:      "undefined policy reference",
			cfg:       config.Config{Users: []config.User{{Name: "bob", Password: "x", Policies: []string{"ghost"}, LineNumber: 23}}},
			wantField: "users[0].policies[0]", wantLine: 23, wantMsg: "undefined policy",
		},
		{
			name:      "builtin policy reference allowed",
			cfg:       config.Config{Users: []config.User{{Name: "bob", Password: "x", Policies: []string{"readwrite"}, LineNumber: 23}}},
			wantField: "", // builtin policies need not be declared
		},
		{
			name:      "policy without statements",
			cfg:       config.Config{Policies: []config.Policy{{Name: "p", LineNumber: 30}}},
			wantField: "policies[0].statements", wantLine: 30, wantMsg: "at least one statement",
		},
		{
			name: "policy invalid effect",
			cfg: config.Config{Policies: []config.Policy{{
				Name: "p", LineNumber: 31,
				Statements: []config.Statement{{Effect: "Maybe", Actions: []string{"a"}, Resources: []string{"r"}}},
			}}},
			wantField: "policies[0].statements[0].effect", wantLine: 31, wantMsg: "Allow",
		},
		{
			name: "policy empty actions",
			cfg: config.Config{Policies: []config.Policy{{
				Name: "p", LineNumber: 32,
				Statements: []config.Statement{{Effect: "Allow", Resources: []string{"r"}}},
			}}},
			wantField: "policies[0].statements[0].actions", wantLine: 32, wantMsg: "action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := config.Validate(&tt.cfg, "test.yaml")

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}

			ce := findConfigError(t, err, tt.wantField)
			if ce.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d (err: %v)", ce.Line, tt.wantLine, ce)
			}
			if tt.wantMsg != "" && !strings.Contains(ce.Message, tt.wantMsg) {
				t.Errorf("Message %q does not contain %q", ce.Message, tt.wantMsg)
			}
		})
	}
}

// findConfigError fails the test unless err contains a *ConfigError whose Field
// matches wantField.
func findConfigError(t *testing.T, err error, wantField string) *apperrors.ConfigError {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with field %q, got nil", wantField)
	}
	for _, ce := range collectConfigErrors(err) {
		if ce.Field == wantField {
			return ce
		}
	}
	t.Fatalf("no ConfigError with field %q in: %v", wantField, err)
	return nil
}

// collectConfigErrors walks a (possibly joined) error tree and returns every
// *ConfigError it finds, without descending into their Cause.
func collectConfigErrors(err error) []*apperrors.ConfigError {
	var out []*apperrors.ConfigError
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		if ce, ok := e.(*apperrors.ConfigError); ok {
			out = append(out, ce)
			return
		}
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			for _, sub := range joined.Unwrap() {
				walk(sub)
			}
			return
		}
		if single, ok := e.(interface{ Unwrap() error }); ok {
			walk(single.Unwrap())
		}
	}
	walk(err)
	return out
}
