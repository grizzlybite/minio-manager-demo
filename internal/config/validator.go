package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	apperrors "github.com/yourorg/minio-manager/internal/errors"
)

// bucketNameRe enforces S3/MinIO bucket naming: 3-63 chars, lowercase letters,
// digits, dots and hyphens, starting and ending with an alphanumeric character.
var bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.\-]{1,61}[a-z0-9]$`)

// builtinPolicies are the canned policies MinIO ships with; user references to
// them are valid even though they are not declared in the config.
var builtinPolicies = map[string]bool{
	"readwrite":    true,
	"readonly":     true,
	"writeonly":    true,
	"diagnostics":  true,
	"consoleAdmin": true,
}

// validEffects are the only accepted IAM statement effects.
var validEffects = map[string]bool{"Allow": true, "Deny": true}

// Validate performs semantic validation of the configuration without contacting
// MinIO. All discovered problems are reported together as a joined error; each
// individual problem is an *errors.ConfigError carrying the source file, line
// and field path. It returns nil when the configuration is valid.
//
// The file argument is the source name returned by the Source (used only for
// diagnostics).
func Validate(cfg *Config, file string) error {
	var errs []error
	errs = append(errs, validatePolicies(cfg, file)...)
	errs = append(errs, validateBuckets(cfg, file)...)
	errs = append(errs, validateUsers(cfg, file)...)
	return errors.Join(errs...)
}

func validateBuckets(cfg *Config, file string) []error {
	var errs []error
	seen := make(map[string]bool, len(cfg.Buckets))

	for i, b := range cfg.Buckets {
		field := fmt.Sprintf("buckets[%d]", i)
		switch {
		case b.Name == "":
			errs = append(errs, cfgErr(file, b.LineNumber, field+".name", "bucket name is required"))
		case !bucketNameRe.MatchString(b.Name):
			errs = append(errs, cfgErr(file, b.LineNumber, field+".name",
				fmt.Sprintf("invalid bucket name %q: must be 3-63 chars, lowercase alphanumeric, '.' or '-'", b.Name)))
		case seen[b.Name]:
			errs = append(errs, cfgErr(file, b.LineNumber, field+".name",
				fmt.Sprintf("duplicate bucket name %q", b.Name)))
		default:
			seen[b.Name] = true
		}

		if b.QuotaGB < 0 {
			errs = append(errs, cfgErr(file, b.LineNumber, field+".quota_gb", "quota_gb must be >= 0"))
		}
		if b.LifecycleDays < 0 {
			errs = append(errs, cfgErr(file, b.LineNumber, field+".lifecycle_days", "lifecycle_days must be >= 0"))
		}
	}
	return errs
}

func validateUsers(cfg *Config, file string) []error {
	var errs []error
	defined := definedPolicyNames(cfg)
	seen := make(map[string]bool, len(cfg.Users))

	for i, u := range cfg.Users {
		field := fmt.Sprintf("users[%d]", i)
		switch {
		case u.Name == "":
			errs = append(errs, cfgErr(file, u.LineNumber, field+".name", "user name is required"))
		case seen[u.Name]:
			errs = append(errs, cfgErr(file, u.LineNumber, field+".name",
				fmt.Sprintf("duplicate user name %q", u.Name)))
		default:
			seen[u.Name] = true
		}

		if u.Password == "" {
			errs = append(errs, cfgErr(file, u.LineNumber, field+".password", "password is required (plain value or vault:// reference)"))
		}

		for j, p := range u.Policies {
			if p == "" {
				errs = append(errs, cfgErr(file, u.LineNumber, fmt.Sprintf("%s.policies[%d]", field, j), "policy name is empty"))
				continue
			}
			if !defined[p] && !builtinPolicies[p] {
				errs = append(errs, cfgErr(file, u.LineNumber, fmt.Sprintf("%s.policies[%d]", field, j),
					fmt.Sprintf("references undefined policy %q", p)))
			}
		}
	}
	return errs
}

func validatePolicies(cfg *Config, file string) []error {
	var errs []error
	seen := make(map[string]bool, len(cfg.Policies))

	for i, p := range cfg.Policies {
		field := fmt.Sprintf("policies[%d]", i)
		switch {
		case p.Name == "":
			errs = append(errs, cfgErr(file, p.LineNumber, field+".name", "policy name is required"))
		case seen[p.Name]:
			errs = append(errs, cfgErr(file, p.LineNumber, field+".name",
				fmt.Sprintf("duplicate policy name %q", p.Name)))
		default:
			seen[p.Name] = true
		}

		if len(p.Statements) == 0 {
			errs = append(errs, cfgErr(file, p.LineNumber, field+".statements", "policy must have at least one statement"))
		}
		for j, st := range p.Statements {
			sf := fmt.Sprintf("%s.statements[%d]", field, j)
			if !validEffects[st.Effect] {
				errs = append(errs, cfgErr(file, p.LineNumber, sf+".effect",
					fmt.Sprintf("effect must be \"Allow\" or \"Deny\", got %q", st.Effect)))
			}
			if len(st.Actions) == 0 {
				errs = append(errs, cfgErr(file, p.LineNumber, sf+".actions", "at least one action is required"))
			}
			if len(st.Resources) == 0 {
				errs = append(errs, cfgErr(file, p.LineNumber, sf+".resources", "at least one resource is required"))
			}
		}
	}
	return errs
}

// definedPolicyNames returns the set of policy names declared in the config.
func definedPolicyNames(cfg *Config) map[string]bool {
	names := make(map[string]bool, len(cfg.Policies))
	for _, p := range cfg.Policies {
		if p.Name != "" {
			names[p.Name] = true
		}
	}
	return names
}

// cfgErr builds a *errors.ConfigError. Message is trimmed for consistency.
func cfgErr(file string, line int, field, msg string) error {
	return &apperrors.ConfigError{
		File:    file,
		Line:    line,
		Field:   field,
		Message: strings.TrimSpace(msg),
	}
}
