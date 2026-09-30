package source

import (
	"context"
	"fmt"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
)

// VaultSource loads the configuration from a HashiCorp Vault KV v2 secret.
type VaultSource struct {
	Addr  string // Vault address; empty falls back to VAULT_ADDR
	Token string // Vault token; empty falls back to VAULT_TOKEN
	Path  string // full KV path, e.g. "secret/data/minio/config" or "secret/minio/config"
	Field string // field inside the secret containing the YAML
}

// Load reads the configured field from the Vault KV v2 secret at Path and
// returns it as raw bytes. The YAML config is expected to be stored as a single
// string field.
func (s *VaultSource) Load(ctx context.Context) ([]byte, string, error) {
	client, err := newVaultClient(s.Addr, s.Token)
	if err != nil {
		return nil, "", err
	}

	mount, secretPath := splitVaultPath(s.Path)
	secret, err := client.KVv2(mount).Get(ctx, secretPath)
	if err != nil {
		return nil, "", fmt.Errorf("vault: get %q: %w", s.Path, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, "", fmt.Errorf("vault: path %q: no data", s.Path)
	}

	raw, ok := secret.Data[s.Field].(string)
	if !ok {
		return nil, "", fmt.Errorf("vault: path %q: field %q not found or not a string", s.Path, s.Field)
	}
	return []byte(raw), "vault://" + s.Path, nil
}

// newVaultClient builds a Vault client, overriding the env-derived address and
// token only when explicitly provided (preserving flag > ENV priority).
func newVaultClient(addr, token string) (*vaultapi.Client, error) {
	cfg := vaultapi.DefaultConfig()
	if addr != "" {
		cfg.Address = addr
	}
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault: new client: %w", err)
	}
	if token != "" {
		client.SetToken(token)
	}
	return client, nil
}

// splitVaultPath splits a full KV path into the KV v2 mount and the secret path
// relative to that mount. A "data" segment (the literal KV v2 API infix) right
// after the mount is dropped, since KVv2.Get re-inserts it. Examples:
//
//	"secret/data/minio/config" -> ("secret", "minio/config")
//	"secret/minio/config"      -> ("secret", "minio/config")
//	"kv/data/app"              -> ("kv", "app")
func splitVaultPath(full string) (mount, secretPath string) {
	parts := strings.Split(strings.TrimPrefix(full, "/"), "/")
	if len(parts) == 0 {
		return "", ""
	}
	mount = parts[0]
	rest := parts[1:]
	if len(rest) > 0 && rest[0] == "data" {
		rest = rest[1:]
	}
	return mount, strings.Join(rest, "/")
}
