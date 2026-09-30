package minio

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	madmin "github.com/minio/madmin-go/v4"
	miniogo "github.com/minio/minio-go/v7"
	apperrors "github.com/yourorg/minio-manager/internal/errors"
	"github.com/yourorg/minio-manager/internal/state"
	"golang.org/x/sync/errgroup"
)

// FetchCurrentState reads the current buckets, users and policies from MinIO.
// The three resource kinds are fetched concurrently; the first failure cancels
// the rest via the shared errgroup context.
func (c *Client) FetchCurrentState(ctx context.Context) (*state.CurrentState, error) {
	var (
		buckets  []state.BucketInfo
		users    []state.UserInfo
		policies []state.PolicyInfo
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		buckets, err = c.fetchBuckets(gctx)
		return err
	})
	g.Go(func() error {
		var err error
		users, err = c.fetchUsers(gctx)
		return err
	})
	g.Go(func() error {
		var err error
		policies, err = c.fetchPolicies(gctx)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &state.CurrentState{Buckets: buckets, Users: users, Policies: policies}, nil
}

// fetchBuckets lists buckets and reads each bucket's versioning and tagging
// concurrently. Only reliably-readable attributes are populated; quota,
// lifecycle and object-lock current values are applied by the reconciler rather
// than diffed here.
func (c *Client) fetchBuckets(ctx context.Context) ([]state.BucketInfo, error) {
	list, err := c.S3.ListBuckets(ctx)
	if err != nil {
		return nil, &apperrors.MinioError{Op: "list", Kind: apperrors.ResourceBucket, Cause: err}
	}

	out := make([]state.BucketInfo, len(list))
	g, gctx := errgroup.WithContext(ctx)
	for i, b := range list {
		g.Go(func() error {
			info := state.BucketInfo{Name: b.Name, Region: b.BucketRegion}

			ver, err := c.S3.GetBucketVersioning(gctx, b.Name)
			if err != nil {
				return &apperrors.MinioError{Op: "get versioning", Kind: apperrors.ResourceBucket, Resource: b.Name, Cause: err}
			}
			info.Versioning = ver.Enabled()

			tags, err := c.S3.GetBucketTagging(gctx, b.Name)
			switch {
			case err == nil && tags != nil:
				if m := tags.ToMap(); len(m) > 0 {
					info.Tags = m
				}
			case err != nil && miniogo.ToErrorResponse(err).Code != "NoSuchTagSet":
				return &apperrors.MinioError{Op: "get tagging", Kind: apperrors.ResourceBucket, Resource: b.Name, Cause: err}
			}

			out[i] = info
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// fetchUsers lists users via the admin API and normalizes them into UserInfo.
func (c *Client) fetchUsers(ctx context.Context) ([]state.UserInfo, error) {
	m, err := c.Admin.ListUsers(ctx)
	if err != nil {
		return nil, &apperrors.MinioError{Op: "list", Kind: apperrors.ResourceUser, Cause: err}
	}

	out := make([]state.UserInfo, 0, len(m))
	for name, ui := range m {
		out = append(out, state.UserInfo{
			Name:     name,
			Enabled:  ui.Status == madmin.AccountEnabled,
			Policies: splitPolicies(ui.PolicyName),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// fetchPolicies lists canned policies with their raw documents.
func (c *Client) fetchPolicies(ctx context.Context) ([]state.PolicyInfo, error) {
	m, err := c.Admin.ListCannedPolicies(ctx)
	if err != nil {
		return nil, &apperrors.MinioError{Op: "list", Kind: apperrors.ResourcePolicy, Cause: err}
	}

	out := make([]state.PolicyInfo, 0, len(m))
	for name, doc := range m {
		out = append(out, state.PolicyInfo{Name: name, Document: json.RawMessage(doc)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// splitPolicies parses MinIO's comma-separated PolicyName field into a slice,
// dropping empty entries. Returns nil when no policies are attached.
func splitPolicies(policyName string) []string {
	if policyName == "" {
		return nil
	}
	parts := strings.Split(policyName, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
