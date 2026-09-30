package errors

import "fmt"

// ResourceKind identifies the kind of MinIO resource an operation acted upon.
type ResourceKind string

const (
	ResourceBucket ResourceKind = "bucket"
	ResourceUser   ResourceKind = "user"
	ResourcePolicy ResourceKind = "policy"
)

// MinioError represents a failure of an operation against the MinIO API,
// enriched with the operation name and the resource it targeted so the failure
// can be reported with full context.
type MinioError struct {
	Op       string       // operation: "create", "update", "delete", "list", "fetch state"
	Kind     ResourceKind // resource kind: bucket, user, policy
	Resource string       // resource name (e.g. bucket or user name); empty for bulk ops
	Cause    error        // original error from the MinIO/madmin client (may be nil)
}

// Error implements the error interface. The resource name is included only when
// it is known, keeping messages meaningful for both single-resource and bulk
// operations.
func (e *MinioError) Error() string {
	if e.Resource != "" {
		if e.Cause != nil {
			return fmt.Sprintf("minio: %s %s %q: %v", e.Op, e.Kind, e.Resource, e.Cause)
		}
		return fmt.Sprintf("minio: %s %s %q", e.Op, e.Kind, e.Resource)
	}
	if e.Cause != nil {
		return fmt.Sprintf("minio: %s %s: %v", e.Op, e.Kind, e.Cause)
	}
	return fmt.Sprintf("minio: %s %s", e.Op, e.Kind)
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As traversal.
func (e *MinioError) Unwrap() error { return e.Cause }
