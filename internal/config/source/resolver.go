package source

import (
	"context"
	"fmt"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
)

// vaultRefPrefix marks a password value that must be resolved from Vault, e.g.
// "vault://secret/minio/alice#password".
const vaultRefPrefix = "vault://"

// IsVaultRef reports whether s is a Vault reference of the form
// "vault://<path>#<field>".
func IsVaultRef(s string) bool {
	return strings.HasPrefix(s, vaultRefPrefix)
}

// VaultResolver resolves vault:// password references to their secret values.
// The underlying Vault client is safe for concurrent use, so a single resolver
// may be shared across goroutines (e.g. when resolving users in parallel).
type VaultResolver struct {
	client *vaultapi.Client
}

// NewVaultResolver builds a resolver with its own Vault client. Addr/Token
// override the env-derived defaults only when non-empty.
func NewVaultResolver(addr, token string) (*VaultResolver, error) {
	client, err := newVaultClient(addr, token)
	if err != nil {
		return nil, err
	}
	return &VaultResolver{client: client}, nil
}

// Resolve fetches the value referenced by ref. ref must be a Vault reference of
// the form "vault://<path>#<field>"; the field is read from the KV v2 secret at
// <path> and must be a string.
func (r *VaultResolver) Resolve(ctx context.Context, ref string) (string, error) {
	if !IsVaultRef(ref) {
		return "", fmt.Errorf("vault ref %q: missing %q prefix", ref, vaultRefPrefix)
	}
	body := strings.TrimPrefix(ref, vaultRefPrefix)
	pathPart, field, ok := strings.Cut(body, "#")
	if !ok || field == "" {
		return "", fmt.Errorf("vault ref %q: missing '#field'", ref)
	}
	if pathPart == "" {
		return "", fmt.Errorf("vault ref %q: empty path", ref)
	}

	mount, secretPath := splitVaultPath(pathPart)
	secret, err := r.client.KVv2(mount).Get(ctx, secretPath)
	if err != nil {
		return "", fmt.Errorf("vault: get %q: %w", pathPart, err)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("vault ref %q: no data at path", ref)
	}

	val, ok := secret.Data[field].(string)
	if !ok {
		return "", fmt.Errorf("vault ref %q: field %q not found or not a string", ref, field)
	}
	return val, nil
}
