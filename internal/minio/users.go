package minio

import (
	"context"
	"log/slog"

	madmin "github.com/minio/madmin-go/v4"
	apperrors "github.com/yourorg/minio-manager/internal/errors"
	"github.com/yourorg/minio-manager/internal/state"
)

// CreateUser creates a user, sets its account status and attaches its policies.
// The password is never logged.
func (c *Client) CreateUser(ctx context.Context, spec state.UserSpec) error {
	slog.Info("creating user", "user", spec.Name, "enabled", spec.Enabled, "policies", spec.Policies)

	if err := c.Admin.AddUser(ctx, spec.Name, spec.Password); err != nil {
		return &apperrors.MinioError{Op: "create", Kind: apperrors.ResourceUser, Resource: spec.Name, Cause: err}
	}
	if !spec.Enabled {
		if err := c.Admin.SetUser(ctx, spec.Name, spec.Password, madmin.AccountDisabled); err != nil {
			return &apperrors.MinioError{Op: "disable", Kind: apperrors.ResourceUser, Resource: spec.Name, Cause: err}
		}
	}
	return c.reconcileUserPolicies(ctx, spec.Name, spec.Policies, nil)
}

// UpdateUser re-asserts the user's password/status and converges its policy
// attachments to the desired set.
func (c *Client) UpdateUser(ctx context.Context, upd state.UserUpdate) error {
	d := upd.Desired
	slog.Info("updating user", "user", d.Name, "changes", upd.Changes)

	// SetUser re-asserts both the secret key and the account status; since the
	// password cannot be read back, applying the desired value keeps it in sync.
	if err := c.Admin.SetUser(ctx, d.Name, d.Password, statusFor(d.Enabled)); err != nil {
		return &apperrors.MinioError{Op: "update", Kind: apperrors.ResourceUser, Resource: d.Name, Cause: err}
	}
	return c.reconcileUserPolicies(ctx, d.Name, d.Policies, upd.Current.Policies)
}

// DeleteUser removes a user.
func (c *Client) DeleteUser(ctx context.Context, name string) error {
	slog.Warn("deleting user", "user", name)
	if err := c.Admin.RemoveUser(ctx, name); err != nil {
		return &apperrors.MinioError{Op: "delete", Kind: apperrors.ResourceUser, Resource: name, Cause: err}
	}
	return nil
}

// reconcileUserPolicies attaches policies present in desired but not current,
// and detaches policies present in current but not desired.
func (c *Client) reconcileUserPolicies(ctx context.Context, user string, desired, current []string) error {
	toAttach := setDifference(desired, current)
	toDetach := setDifference(current, desired)

	if len(toAttach) > 0 {
		_, err := c.Admin.AttachPolicy(ctx, madmin.PolicyAssociationReq{User: user, Policies: toAttach})
		if err != nil {
			return &apperrors.MinioError{Op: "attach policies", Kind: apperrors.ResourceUser, Resource: user, Cause: err}
		}
	}
	if len(toDetach) > 0 {
		_, err := c.Admin.DetachPolicy(ctx, madmin.PolicyAssociationReq{User: user, Policies: toDetach})
		if err != nil {
			return &apperrors.MinioError{Op: "detach policies", Kind: apperrors.ResourceUser, Resource: user, Cause: err}
		}
	}
	return nil
}

// statusFor maps the enabled flag to a MinIO account status.
func statusFor(enabled bool) madmin.AccountStatus {
	if enabled {
		return madmin.AccountEnabled
	}
	return madmin.AccountDisabled
}

// setDifference returns the elements of a that are not present in b.
func setDifference(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	inB := make(map[string]bool, len(b))
	for _, s := range b {
		inB[s] = true
	}
	var diff []string
	for _, s := range a {
		if !inB[s] {
			diff = append(diff, s)
		}
	}
	return diff
}
