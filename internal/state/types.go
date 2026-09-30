// Package state models the desired and current states of MinIO resources and
// computes the diff between them. It is independent of the config and MinIO
// client packages: callers map their data into these types.
package state

import "encoding/json"

// ---------------------------------------------------------------------------
// Desired state (derived from the YAML config)
// ---------------------------------------------------------------------------

// BucketSpec is the desired specification of a bucket.
type BucketSpec struct {
	Name          string
	Region        string
	Versioning    bool
	ObjectLocking bool
	QuotaGB       int64
	LifecycleDays int
	Tags          map[string]string
	Line          int // source line in the config, for diagnostics
}

// UserSpec is the desired specification of a user. Password is sensitive and
// must never be logged or printed.
type UserSpec struct {
	Name     string
	Password string
	Policies []string
	Enabled  bool
	Line     int
}

// PolicySpec is the desired specification of an IAM policy. Document is the
// canonical IAM policy JSON used to create/update the canned policy.
type PolicySpec struct {
	Name     string
	Document json.RawMessage
	Line     int
}

// DesiredState is the complete desired configuration.
type DesiredState struct {
	Buckets  []BucketSpec
	Users    []UserSpec
	Policies []PolicySpec
}

// ---------------------------------------------------------------------------
// Current state (read from MinIO)
// ---------------------------------------------------------------------------

// BucketInfo is the observed state of a bucket. Only fields that can be read
// back reliably are populated (see internal/minio/state.go).
type BucketInfo struct {
	Name       string
	Region     string
	Versioning bool
	Tags       map[string]string
}

// UserInfo is the observed state of a user.
type UserInfo struct {
	Name     string
	Policies []string
	Enabled  bool
}

// PolicyInfo is the observed state of an IAM policy, with the raw policy
// document as returned by MinIO.
type PolicyInfo struct {
	Name     string
	Document json.RawMessage
}

// CurrentState is the complete observed configuration.
type CurrentState struct {
	Buckets  []BucketInfo
	Users    []UserInfo
	Policies []PolicyInfo
}

// ---------------------------------------------------------------------------
// Diff result
// ---------------------------------------------------------------------------

// BucketUpdate describes a bucket that exists but whose mutable attributes
// differ from the desired spec. Changes lists the human-readable field names
// that differ.
type BucketUpdate struct {
	Name    string
	Desired BucketSpec
	Current BucketInfo
	Changes []string
}

// UserUpdate describes a user whose attributes differ from the desired spec.
type UserUpdate struct {
	Name    string
	Desired UserSpec
	Current UserInfo
	Changes []string
}

// PolicyUpdate describes a policy whose document differs from the desired spec.
type PolicyUpdate struct {
	Name    string
	Desired PolicySpec
	Current PolicyInfo
}

// DiffResult is the set of changes required to converge current state to
// desired state. Delete lists are always computed but are only executed by the
// reconciler when Prune is true.
type DiffResult struct {
	BucketsToCreate []BucketSpec
	BucketsToUpdate []BucketUpdate
	BucketsToDelete []string // only acted upon when --prune

	UsersToCreate []UserSpec
	UsersToUpdate []UserUpdate
	UsersToDelete []string

	PoliciesToCreate []PolicySpec
	PoliciesToUpdate []PolicyUpdate
	PoliciesToDelete []string

	Prune bool
}

// Empty reports whether the diff contains no changes to apply (ignoring the
// Prune flag itself).
func (d DiffResult) Empty() bool {
	return len(d.BucketsToCreate) == 0 && len(d.BucketsToUpdate) == 0 && len(d.BucketsToDelete) == 0 &&
		len(d.UsersToCreate) == 0 && len(d.UsersToUpdate) == 0 && len(d.UsersToDelete) == 0 &&
		len(d.PoliciesToCreate) == 0 && len(d.PoliciesToUpdate) == 0 && len(d.PoliciesToDelete) == 0
}
