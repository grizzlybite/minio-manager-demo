package config

import (
	"context"
	"fmt"

	"github.com/yourorg/minio-manager/internal/config/source"
	apperrors "github.com/yourorg/minio-manager/internal/errors"
	"github.com/yourorg/minio-manager/pkg/yamlpos"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// Source abstracts the origin of the raw configuration bytes (file, Consul KV,
// Vault). It returns the raw data and a human-readable source name used in
// diagnostics (path, "consul://...", "vault://...").
type Source interface {
	Load(ctx context.Context) (data []byte, sourceName string, err error)
}

// PasswordResolver resolves a single "vault://<path>#<field>" reference to its
// secret string value. *source.VaultResolver implements this interface.
type PasswordResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// Loader reads, parses and post-processes the configuration from a Source.
type Loader struct {
	Source        Source
	VaultResolver PasswordResolver // optional; required only when vault:// refs are present
}

// NewLoader constructs a Loader. resolver may be nil if no vault:// password
// references are expected.
func NewLoader(src Source, resolver PasswordResolver) *Loader {
	return &Loader{Source: src, VaultResolver: resolver}
}

// Load performs the full configuration loading pipeline:
//
//  1. read the raw bytes from the Source;
//  2. unmarshal the YAML into a *Config;
//  3. annotate every bucket/user/policy with its source line number;
//  4. resolve vault:// password references in parallel.
//
// Parsing and resolution failures are reported as *errors.ConfigError carrying
// the source name, line and field path.
func (l *Loader) Load(ctx context.Context) (*Config, error) {
	data, name, err := l.Source.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &apperrors.ConfigError{
			File:    name,
			Field:   "(document)",
			Message: "invalid YAML: " + err.Error(),
			Cause:   err,
		}
	}

	if err := annotateLines(&cfg, data); err != nil {
		return nil, err
	}

	if err := l.resolvePasswords(ctx, &cfg, name); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// annotateLines fills the LineNumber field of each element of the buckets,
// users and policies sequences from the YAML source positions.
func annotateLines(cfg *Config, data []byte) error {
	root, err := yamlpos.Parse(data)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}

	for i, ln := range yamlpos.SequenceLines(root, "buckets") {
		if i < len(cfg.Buckets) {
			cfg.Buckets[i].LineNumber = ln
		}
	}
	for i, ln := range yamlpos.SequenceLines(root, "users") {
		if i < len(cfg.Users) {
			cfg.Users[i].LineNumber = ln
		}
	}
	for i, ln := range yamlpos.SequenceLines(root, "policies") {
		if i < len(cfg.Policies) {
			cfg.Policies[i].LineNumber = ln
		}
	}
	return nil
}

// resolvePasswords replaces every vault:// password reference with the secret
// value fetched from Vault. Independent users are resolved concurrently; the
// first failure cancels the rest via the shared errgroup context.
func (l *Loader) resolvePasswords(ctx context.Context, cfg *Config, file string) error {
	g, gctx := errgroup.WithContext(ctx)
	for i := range cfg.Users {
		ref := cfg.Users[i].Password
		if !source.IsVaultRef(ref) {
			continue
		}
		field := fmt.Sprintf("users[%d].password", i)
		line := cfg.Users[i].LineNumber

		g.Go(func() error {
			if l.VaultResolver == nil {
				return &apperrors.ConfigError{
					File:    file,
					Line:    line,
					Field:   field,
					Message: "vault reference requires Vault configuration (--vault-addr/--vault-token)",
				}
			}
			val, err := l.VaultResolver.Resolve(gctx, ref)
			if err != nil {
				return &apperrors.ConfigError{
					File:    file,
					Line:    line,
					Field:   field,
					Message: fmt.Sprintf("cannot resolve vault reference %q", ref),
					Cause:   err,
				}
			}
			cfg.Users[i].Password = val
			return nil
		})
	}
	return g.Wait()
}
