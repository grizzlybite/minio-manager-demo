// Package source provides concrete config.Source implementations that fetch the
// raw YAML configuration from a local file, Consul KV or HashiCorp Vault.
//
// Each type structurally satisfies the config.Source interface
// (Load(ctx) (data []byte, sourceName string, err error)).
package source

import (
	"context"
	"fmt"
	"os"
)

// FileSource loads the configuration from a local file path.
type FileSource struct {
	Path string
}

// Load reads the file at Path and returns its bytes and the path as the source
// name. The context is accepted to satisfy the Source interface; os.ReadFile is
// not cancellable, so it is read in one shot.
func (s *FileSource) Load(ctx context.Context) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", fmt.Errorf("file source %q canceled: %w", s.Path, err)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, "", fmt.Errorf("read config file %q: %w", s.Path, err)
	}
	return data, s.Path, nil
}
