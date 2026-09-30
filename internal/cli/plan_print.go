package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/yourorg/minio-manager/internal/state"
)

// printPlan writes a human-readable summary of the diff to w. Deletions are
// only shown when prune is enabled. Secrets are never printed.
func printPlan(w io.Writer, diff state.DiffResult) {
	fmt.Fprintln(w, "Plan:")
	if diff.Empty() {
		fmt.Fprintln(w, "  (no changes)")
		return
	}

	printSection(w, "Policies",
		mapNames(diff.PoliciesToCreate, func(p state.PolicySpec) string { return p.Name }),
		mapNames(diff.PoliciesToUpdate, func(p state.PolicyUpdate) string { return p.Name }),
		diff.PoliciesToDelete, diff.Prune)

	printSection(w, "Buckets",
		mapNames(diff.BucketsToCreate, func(b state.BucketSpec) string { return b.Name }),
		mapNames(diff.BucketsToUpdate, func(b state.BucketUpdate) string {
			return fmt.Sprintf("%s (%s)", b.Name, strings.Join(b.Changes, ", "))
		}),
		diff.BucketsToDelete, diff.Prune)

	printSection(w, "Users",
		mapNames(diff.UsersToCreate, func(u state.UserSpec) string { return u.Name }),
		mapNames(diff.UsersToUpdate, func(u state.UserUpdate) string {
			return fmt.Sprintf("%s (%s)", u.Name, strings.Join(u.Changes, ", "))
		}),
		diff.UsersToDelete, diff.Prune)
}

// printSection prints one resource kind's create (+), update (~) and, when
// prune is enabled, delete (-) entries. Nothing is printed for an empty kind.
func printSection(w io.Writer, title string, create, update, del []string, prune bool) {
	showDel := prune && len(del) > 0
	if len(create) == 0 && len(update) == 0 && !showDel {
		return
	}

	fmt.Fprintf(w, "  %s:\n", title)
	for _, n := range create {
		fmt.Fprintf(w, "    + %s\n", n)
	}
	for _, n := range update {
		fmt.Fprintf(w, "    ~ %s\n", n)
	}
	if showDel {
		for _, n := range del {
			fmt.Fprintf(w, "    - %s\n", n)
		}
	}
}

// mapNames projects a slice into a slice of display strings.
func mapNames[T any](items []T, name func(T) string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = name(it)
	}
	return out
}
