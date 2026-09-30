//go:build integration

// Package integration_test exercises the full load -> plan -> apply pipeline
// against a real MinIO server started with testcontainers-go.
//
// Run with: go test -tags=integration -race -timeout=120s ./tests/integration/...
package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yourorg/minio-manager/internal/config"
	mm "github.com/yourorg/minio-manager/internal/minio"
	"github.com/yourorg/minio-manager/internal/state"
)

const (
	rootUser = "minioadmin"
	rootPass = "minioadmin"
)

// startMinIO launches a MinIO container and returns its API endpoint plus a
// cleanup function.
func startMinIO(t *testing.T) (endpoint string, cleanup func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     rootUser,
			"MINIO_ROOT_PASSWORD": rootPass,
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true},
	)
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}

	return host + ":" + port.Port(), func() { _ = container.Terminate(ctx) }
}

// sampleConfig returns a config exercising buckets, a policy and a user that
// references it.
func sampleConfig() *config.Config {
	return &config.Config{
		Buckets: []config.Bucket{{
			Name:       "data-bucket",
			Region:     "us-east-1",
			Versioning: true,
			Tags:       map[string]string{"env": "test"},
		}},
		Policies: []config.Policy{{
			Name: "rw-data-bucket",
			Statements: []config.Statement{{
				Effect:    "Allow",
				Actions:   []string{"s3:GetObject", "s3:PutObject", "s3:ListBucket"},
				Resources: []string{"arn:aws:s3:::data-bucket", "arn:aws:s3:::data-bucket/*"},
			}},
		}},
		Users: []config.User{{
			Name:     "alice",
			Password: "alicepass123",
			Policies: []string{"rw-data-bucket"},
			Enabled:  true,
		}},
	}
}

func newClient(t *testing.T, endpoint string) *mm.Client {
	t.Helper()
	client, err := mm.NewClient(mm.Config{
		Endpoint:  endpoint,
		AccessKey: rootUser,
		SecretKey: rootPass,
		TLS:       false,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func TestApplyCreatesResources(t *testing.T) {
	endpoint, cleanup := startMinIO(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg := sampleConfig()
	if err := config.Validate(cfg, "test"); err != nil {
		t.Fatalf("validate: %v", err)
	}

	client := newClient(t, endpoint)
	rec := mm.NewReconciler(client)

	// Plan should be non-empty against an empty server, then apply it.
	diff, err := rec.Plan(ctx, cfg.ToDesired(), false)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if diff.Empty() {
		t.Fatal("expected a non-empty plan against a fresh server")
	}
	if err := rec.Apply(ctx, diff); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Verify the resources now exist in the current state.
	cur, err := client.FetchCurrentState(ctx)
	if err != nil {
		t.Fatalf("fetch current state: %v", err)
	}
	if !hasBucket(cur.Buckets, "data-bucket") {
		t.Errorf("bucket data-bucket not found in %v", cur.Buckets)
	}
	if !hasPolicy(cur.Policies, "rw-data-bucket") {
		t.Errorf("policy rw-data-bucket not found")
	}
	if !hasUser(cur.Users, "alice") {
		t.Errorf("user alice not found")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	endpoint, cleanup := startMinIO(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg := sampleConfig()
	client := newClient(t, endpoint)
	rec := mm.NewReconciler(client)

	first, err := rec.Plan(ctx, cfg.ToDesired(), false)
	if err != nil {
		t.Fatalf("first plan: %v", err)
	}
	if err := rec.Apply(ctx, first); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// A second plan over the now-converged server must contain no changes.
	second, err := rec.Plan(ctx, cfg.ToDesired(), false)
	if err != nil {
		t.Fatalf("second plan: %v", err)
	}
	if !second.Empty() {
		t.Errorf("expected empty plan after apply, got: %+v", second)
	}
}

func TestImportReflectsAppliedState(t *testing.T) {
	endpoint, cleanup := startMinIO(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg := sampleConfig()
	client := newClient(t, endpoint)
	rec := mm.NewReconciler(client)

	diff, err := rec.Plan(ctx, cfg.ToDesired(), false)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if err := rec.Apply(ctx, diff); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Import the live state back into a config and check it reflects what we
	// applied (built-in policies excluded).
	cur, err := client.FetchCurrentState(ctx)
	if err != nil {
		t.Fatalf("fetch current state: %v", err)
	}
	imported, err := config.FromState(*cur, false)
	if err != nil {
		t.Fatalf("FromState: %v", err)
	}

	if !hasBucket(cur.Buckets, "data-bucket") {
		t.Error("imported config missing data-bucket")
	}
	var foundPolicy bool
	for _, p := range imported.Policies {
		if p.Name == "rw-data-bucket" {
			foundPolicy = true
			if len(p.Statements) == 0 || p.Statements[0].Effect != "Allow" {
				t.Errorf("imported policy statements wrong: %+v", p.Statements)
			}
		}
	}
	if !foundPolicy {
		t.Error("imported config missing rw-data-bucket policy")
	}

	// Passwords are never exported; once supplied, the config must validate.
	for i := range imported.Users {
		imported.Users[i].Password = "placeholder"
	}
	if err := config.Validate(&imported, "imported"); err != nil {
		t.Errorf("imported config (with passwords) failed validation: %v", err)
	}
}

func hasBucket(bs []state.BucketInfo, name string) bool {
	for _, b := range bs {
		if b.Name == name {
			return true
		}
	}
	return false
}

func hasUser(us []state.UserInfo, name string) bool {
	for _, u := range us {
		if u.Name == name {
			return true
		}
	}
	return false
}

func hasPolicy(ps []state.PolicyInfo, name string) bool {
	for _, p := range ps {
		if p.Name == name {
			return true
		}
	}
	return false
}
