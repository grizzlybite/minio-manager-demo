// Package config defines the configuration data model for minio-manager and
// (in later steps) loads, resolves and validates it.
//
// The YAML config is the single source of truth: it declares the desired set of
// buckets, users and IAM policies that the reconciler applies to MinIO.
package config

// Config is the root structure of the configuration document.
type Config struct {
	Minio    MinioConfig `yaml:"minio"`
	Buckets  []Bucket    `yaml:"buckets"`
	Users    []User      `yaml:"users"`
	Policies []Policy    `yaml:"policies"`
}

// MinioConfig holds connection settings for the MinIO server. Any field may be
// overridden by a CLI flag or environment variable (flags > ENV > this file).
type MinioConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	TLS       bool   `yaml:"tls"`
}

// Bucket is the desired specification of a single S3 bucket.
type Bucket struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description,omitempty"` // optional, documentation only
	Region        string            `yaml:"region"`
	Versioning    bool              `yaml:"versioning"`
	ObjectLocking bool              `yaml:"object_locking"`
	QuotaGB       int64             `yaml:"quota_gb"`       // 0 = no quota
	LifecycleDays int               `yaml:"lifecycle_days"` // 0 = no lifecycle rule
	Tags          map[string]string `yaml:"tags"`
	LineNumber    int               `yaml:"-"` // source line, filled by the loader
}

// User is the desired specification of a MinIO user.
type User struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"` // optional, documentation only
	Password    string   `yaml:"password"`              // plain value or a vault://... reference
	Policies    []string `yaml:"policies"`
	Enabled     bool     `yaml:"enabled"`
	LineNumber  int      `yaml:"-"` // source line, filled by the loader
}

// Policy is the desired specification of a named IAM (canned) policy.
type Policy struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description,omitempty"` // optional, documentation only
	Statements  []Statement `yaml:"statements"`
	LineNumber  int         `yaml:"-"` // source line, filled by the loader
}

// Statement is a single IAM policy statement.
type Statement struct {
	Effect    string   `yaml:"effect"`
	Actions   []string `yaml:"actions"`
	Resources []string `yaml:"resources"`
}
