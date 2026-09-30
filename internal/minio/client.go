// Package minio wraps the MinIO S3 client (minio-go/v7) and the MinIO admin
// client (madmin-go/v4), and implements reading current state and reconciling
// it towards the desired state.
package minio

import (
	"fmt"

	madmin "github.com/minio/madmin-go/v4"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client bundles the S3 data-plane client and the admin control-plane client,
// both pointed at the same MinIO deployment.
type Client struct {
	S3    *miniogo.Client
	Admin *madmin.AdminClient
}

// Config holds the connection parameters required to build a Client. Values are
// expected to be already resolved by the caller following the
// flags > ENV > YAML priority.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	TLS       bool
}

// NewClient constructs both the S3 and admin clients from cfg. The endpoint
// must be a host[:port] without scheme; TLS toggles HTTPS.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("minio: endpoint is required")
	}

	creds := credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")

	s3, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  creds,
		Secure: cfg.TLS,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: new s3 client: %w", err)
	}

	admin, err := madmin.NewWithOptions(cfg.Endpoint, &madmin.Options{
		Creds:  creds,
		Secure: cfg.TLS,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: new admin client: %w", err)
	}

	return &Client{S3: s3, Admin: admin}, nil
}
