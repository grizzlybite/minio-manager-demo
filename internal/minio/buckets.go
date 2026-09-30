package minio

import (
	"context"
	"log/slog"
	"slices"

	madmin "github.com/minio/madmin-go/v4"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/tags"
	apperrors "github.com/yourorg/minio-manager/internal/errors"
	"github.com/yourorg/minio-manager/internal/state"
)

const lifecycleRuleID = "minio-manager-expiration"

// bytesPerGB is the number of bytes in one gibibyte, used for quota conversion.
const bytesPerGB = 1 << 30

// CreateBucket creates a bucket and applies all of its declared settings
// (versioning, tags, quota, lifecycle).
func (c *Client) CreateBucket(ctx context.Context, spec state.BucketSpec) error {
	slog.Info("creating bucket", "name", spec.Name, "region", spec.Region)

	err := c.S3.MakeBucket(ctx, spec.Name, miniogo.MakeBucketOptions{
		Region:        spec.Region,
		ObjectLocking: spec.ObjectLocking,
	})
	if err != nil {
		return &apperrors.MinioError{Op: "create", Kind: apperrors.ResourceBucket, Resource: spec.Name, Cause: err}
	}

	if spec.Versioning {
		if err := c.applyVersioning(ctx, spec.Name, true); err != nil {
			return err
		}
	}
	if len(spec.Tags) > 0 {
		if err := c.applyTags(ctx, spec.Name, spec.Tags); err != nil {
			return err
		}
	}
	if spec.QuotaGB > 0 {
		if err := c.applyQuota(ctx, spec.Name, spec.QuotaGB); err != nil {
			return err
		}
	}
	if spec.LifecycleDays > 0 {
		if err := c.applyLifecycle(ctx, spec.Name, spec.LifecycleDays); err != nil {
			return err
		}
	}
	return nil
}

// UpdateBucket applies the changed mutable attributes of an existing bucket.
func (c *Client) UpdateBucket(ctx context.Context, upd state.BucketUpdate) error {
	slog.Info("updating bucket", "name", upd.Name, "changes", upd.Changes)

	if slices.Contains(upd.Changes, "versioning") {
		if err := c.applyVersioning(ctx, upd.Name, upd.Desired.Versioning); err != nil {
			return err
		}
	}
	if slices.Contains(upd.Changes, "tags") {
		if err := c.applyTags(ctx, upd.Name, upd.Desired.Tags); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBucket removes a bucket. The bucket must be empty.
func (c *Client) DeleteBucket(ctx context.Context, name string) error {
	slog.Warn("deleting bucket", "name", name)
	if err := c.S3.RemoveBucket(ctx, name); err != nil {
		return &apperrors.MinioError{Op: "delete", Kind: apperrors.ResourceBucket, Resource: name, Cause: err}
	}
	return nil
}

func (c *Client) applyVersioning(ctx context.Context, name string, enabled bool) error {
	status := "Suspended"
	if enabled {
		status = "Enabled"
	}
	err := c.S3.SetBucketVersioning(ctx, name, miniogo.BucketVersioningConfiguration{Status: status})
	if err != nil {
		return &apperrors.MinioError{Op: "set versioning", Kind: apperrors.ResourceBucket, Resource: name, Cause: err}
	}
	return nil
}

// applyTags sets the bucket tags, or removes all tags when the desired set is
// empty.
func (c *Client) applyTags(ctx context.Context, name string, tagMap map[string]string) error {
	if len(tagMap) == 0 {
		if err := c.S3.RemoveBucketTagging(ctx, name); err != nil {
			return &apperrors.MinioError{Op: "remove tags", Kind: apperrors.ResourceBucket, Resource: name, Cause: err}
		}
		return nil
	}

	t, err := tags.NewTags(tagMap, false)
	if err != nil {
		return &apperrors.MinioError{Op: "build tags", Kind: apperrors.ResourceBucket, Resource: name, Cause: err}
	}
	if err := c.S3.SetBucketTagging(ctx, name, t); err != nil {
		return &apperrors.MinioError{Op: "set tags", Kind: apperrors.ResourceBucket, Resource: name, Cause: err}
	}
	return nil
}

func (c *Client) applyQuota(ctx context.Context, name string, quotaGB int64) error {
	quota := &madmin.BucketQuota{
		Size: uint64(quotaGB) * bytesPerGB,
		Type: madmin.HardQuota,
	}
	if err := c.Admin.SetBucketQuota(ctx, name, quota); err != nil {
		return &apperrors.MinioError{Op: "set quota", Kind: apperrors.ResourceBucket, Resource: name, Cause: err}
	}
	return nil
}

func (c *Client) applyLifecycle(ctx context.Context, name string, days int) error {
	cfg := &lifecycle.Configuration{
		Rules: []lifecycle.Rule{{
			ID:         lifecycleRuleID,
			Status:     "Enabled",
			Expiration: lifecycle.Expiration{Days: lifecycle.ExpirationDays(days)},
		}},
	}
	if err := c.S3.SetBucketLifecycle(ctx, name, cfg); err != nil {
		return &apperrors.MinioError{Op: "set lifecycle", Kind: apperrors.ResourceBucket, Resource: name, Cause: err}
	}
	return nil
}
