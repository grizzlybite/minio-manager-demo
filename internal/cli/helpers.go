package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"github.com/yourorg/minio-manager/internal/config"
	"github.com/yourorg/minio-manager/internal/config/source"
	minioc "github.com/yourorg/minio-manager/internal/minio"
)

// target is a single loaded configuration together with its display name. Each
// target maps to one MinIO cluster: multiple config files (dev/qa/prod) each
// carry their own minio: block and their own buckets/users/policies.
type target struct {
	cfg  *config.Config
	name string
}

// loadConfigs resolves every configured source into a list of targets and loads
// each one. File sources expand to one target per file (repeatable --config plus
// --config-dir); Consul and Vault sources yield a single target. When more than
// one file target is present, a --minio-endpoint override is rejected because a
// single endpoint flag cannot apply to several distinct clusters.
func loadConfigs(ctx context.Context) ([]target, error) {
	srcs, resolver, err := buildSources()
	if err != nil {
		return nil, err
	}
	if len(srcs) > 1 && viper.GetString("minio-endpoint") != "" {
		return nil, fmt.Errorf("--minio-endpoint/MINIO_ENDPOINT cannot be used with multiple config targets; " +
			"set the endpoint inside each config's minio: block")
	}

	targets := make([]target, 0, len(srcs))
	for _, s := range srcs {
		cfg, err := config.NewLoader(s.src, resolver).Load(ctx)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target{cfg: cfg, name: s.name})
	}
	return targets, nil
}

// namedSource pairs a config.Source with its human-readable name for diagnostics.
type namedSource struct {
	src  config.Source
	name string
}

// buildSources selects and expands the configured sources. Exactly one origin
// kind must be set: --vault-path, --consul-key, or file input (--config and/or
// --config-dir). When a Vault address is configured, a VaultResolver is also
// returned to resolve vault:// password references regardless of the origin.
func buildSources() ([]namedSource, config.PasswordResolver, error) {
	vaultAddr := viper.GetString("vault-addr")
	vaultToken := os.Getenv("VAULT_TOKEN")

	var resolver config.PasswordResolver
	if vaultAddr != "" {
		r, err := source.NewVaultResolver(vaultAddr, vaultToken)
		if err != nil {
			return nil, nil, err
		}
		resolver = r
	}

	files := viper.GetStringSlice("config")
	configDir := viper.GetString("config-dir")

	switch {
	case viper.GetString("vault-path") != "":
		path := viper.GetString("vault-path")
		src := &source.VaultSource{
			Addr:  vaultAddr,
			Token: vaultToken,
			Path:  path,
			Field: viper.GetString("vault-field"),
		}
		return []namedSource{{src: src, name: "vault://" + path}}, resolver, nil

	case viper.GetString("consul-key") != "":
		key := viper.GetString("consul-key")
		src := &source.ConsulSource{
			Addr:  viper.GetString("consul-addr"),
			Key:   key,
			Token: os.Getenv("CONSUL_HTTP_TOKEN"),
		}
		return []namedSource{{src: src, name: "consul://" + key}}, resolver, nil

	case len(files) > 0 || configDir != "":
		paths, err := collectConfigPaths(files, configDir)
		if err != nil {
			return nil, nil, err
		}
		srcs := make([]namedSource, 0, len(paths))
		for _, p := range paths {
			srcs = append(srcs, namedSource{src: &source.FileSource{Path: p}, name: p})
		}
		return srcs, resolver, nil

	default:
		return nil, nil, fmt.Errorf("no config source: set one of --config, --config-dir, --consul-key, --vault-path")
	}
}

// collectConfigPaths builds the ordered, de-duplicated list of config file
// paths: explicit --config files first in the given order, then *.yaml/*.yml
// files from --config-dir sorted alphabetically. It errors if --config-dir
// contains no config files, so a typo does not silently yield nothing.
func collectConfigPaths(files []string, dir string) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, f := range files {
		add(f)
	}

	if dir != "" {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("read config dir %q: %w", dir, err)
		}
		var found []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".yaml" || ext == ".yml" {
				found = append(found, filepath.Join(dir, e.Name()))
			}
		}
		if len(found) == 0 {
			return nil, fmt.Errorf("config dir %q contains no .yaml/.yml files", dir)
		}
		sort.Strings(found)
		for _, f := range found {
			add(f)
		}
	}

	return paths, nil
}

// newMinioClient builds a MinIO client for one target. When allowFlagOverride is
// true (a single target), connection parameters follow the priority
// CLI flag/ENV (via viper) > config file. With multiple targets each cluster
// must come entirely from its own config, so flag/ENV overrides are ignored.
func newMinioClient(cfg *config.Config, allowFlagOverride bool) (*minioc.Client, error) {
	endpoint := cfg.Minio.Endpoint
	accessKey := cfg.Minio.AccessKey
	secretKey := cfg.Minio.SecretKey
	if allowFlagOverride {
		endpoint = firstNonEmpty(viper.GetString("minio-endpoint"), endpoint)
		accessKey = firstNonEmpty(viper.GetString("minio-access-key"), accessKey)
		secretKey = firstNonEmpty(viper.GetString("minio-secret-key"), secretKey)
	}
	host, schemeTLS := normalizeEndpoint(endpoint)
	return minioc.NewClient(minioc.Config{
		Endpoint:  host,
		AccessKey: accessKey,
		SecretKey: secretKey,
		TLS:       cfg.Minio.TLS || schemeTLS,
	})
}

// normalizeEndpoint strips a leading http://|https:// scheme and a trailing
// slash from an endpoint, returning the bare host[:port] and whether the scheme
// implied TLS (https). The MinIO/madmin clients require a scheme-less endpoint.
func normalizeEndpoint(ep string) (host string, secure bool) {
	if rest, ok := strings.CutPrefix(ep, "https://"); ok {
		return strings.TrimSuffix(rest, "/"), true
	}
	if rest, ok := strings.CutPrefix(ep, "http://"); ok {
		return strings.TrimSuffix(rest, "/"), false
	}
	return strings.TrimSuffix(ep, "/"), false
}

// firstNonEmpty returns the first non-empty string among its arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
