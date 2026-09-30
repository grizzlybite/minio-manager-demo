package source

import (
	"context"
	"fmt"

	consulapi "github.com/hashicorp/consul/api"
)

// ConsulSource loads the configuration from a Consul KV key.
type ConsulSource struct {
	Addr  string // Consul address; empty falls back to CONSUL_HTTP_ADDR
	Key   string // KV key holding the config YAML
	Token string // ACL token; empty falls back to CONSUL_HTTP_TOKEN
}

// Load fetches the value stored at Key from Consul KV. Addr/Token override the
// defaults loaded from the environment only when non-empty, preserving the
// flag > ENV priority.
func (s *ConsulSource) Load(ctx context.Context) ([]byte, string, error) {
	cfg := consulapi.DefaultConfig()
	if s.Addr != "" {
		cfg.Address = s.Addr
	}
	if s.Token != "" {
		cfg.Token = s.Token
	}

	client, err := consulapi.NewClient(cfg)
	if err != nil {
		return nil, "", fmt.Errorf("consul: new client: %w", err)
	}

	pair, _, err := client.KV().Get(s.Key, (&consulapi.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return nil, "", fmt.Errorf("consul: get key %q: %w", s.Key, err)
	}
	if pair == nil {
		return nil, "", fmt.Errorf("consul: key %q not found", s.Key)
	}
	return pair.Value, "consul://" + s.Key, nil
}
