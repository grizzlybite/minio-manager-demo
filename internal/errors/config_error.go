// Package errors defines domain-specific error types for minio-manager.
//
// ConfigError carries the location (file/source + line) and field path of a
// configuration problem so the CLI can render actionable diagnostics.
// MinioError carries the MinIO operation context (operation, resource kind and
// name) so failures against the MinIO API can be reported precisely.
package errors

import "fmt"

// ConfigError represents a parsing or validation failure of the configuration,
// bound to a specific line of the YAML source when available.
type ConfigError struct {
	File    string // path to file or source identifier (consul://..., vault://...)
	Line    int    // line number in the YAML source (0 if unknown)
	Field   string // path to the field: "users[0].password"
	Message string // human-readable description
	Cause   error  // original error (may be nil)
}

// Error implements the error interface. When the line is known it is included
// in the rendered message, otherwise only file and field are shown.
func (e *ConfigError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("[%s:%d] %s: %s", e.File, e.Line, e.Field, e.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", e.File, e.Field, e.Message)
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As traversal.
func (e *ConfigError) Unwrap() error { return e.Cause }
