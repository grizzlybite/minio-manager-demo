package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Diff computes the changes required to converge current state to desired
// state. Create and update lists follow the desired ordering; delete lists are
// sorted for determinism. The returned DiffResult has Prune=false; the caller
// sets it to authorize deletions.
func Diff(desired DesiredState, current CurrentState) DiffResult {
	var r DiffResult
	diffPolicies(&r, desired.Policies, current.Policies)
	diffBuckets(&r, desired.Buckets, current.Buckets)
	diffUsers(&r, desired.Users, current.Users)
	return r
}

func diffPolicies(r *DiffResult, desired []PolicySpec, current []PolicyInfo) {
	cur := make(map[string]PolicyInfo, len(current))
	for _, p := range current {
		cur[p.Name] = p
	}
	desiredNames := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredNames[d.Name] = true
		c, ok := cur[d.Name]
		switch {
		case !ok:
			r.PoliciesToCreate = append(r.PoliciesToCreate, d)
		case !jsonEqual(d.Document, c.Document):
			r.PoliciesToUpdate = append(r.PoliciesToUpdate, PolicyUpdate{Name: d.Name, Desired: d, Current: c})
		}
	}
	for _, c := range current {
		if !desiredNames[c.Name] {
			r.PoliciesToDelete = append(r.PoliciesToDelete, c.Name)
		}
	}
	sort.Strings(r.PoliciesToDelete)
}

func diffBuckets(r *DiffResult, desired []BucketSpec, current []BucketInfo) {
	cur := make(map[string]BucketInfo, len(current))
	for _, b := range current {
		cur[b.Name] = b
	}
	desiredNames := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredNames[d.Name] = true
		c, ok := cur[d.Name]
		if !ok {
			r.BucketsToCreate = append(r.BucketsToCreate, d)
			continue
		}
		// Region is immutable after creation, so it is not diffed. Only the
		// reliably-readable mutable attributes (versioning, tags) are compared.
		var changes []string
		if d.Versioning != c.Versioning {
			changes = append(changes, "versioning")
		}
		if !tagsEqual(d.Tags, c.Tags) {
			changes = append(changes, "tags")
		}
		if len(changes) > 0 {
			r.BucketsToUpdate = append(r.BucketsToUpdate, BucketUpdate{Name: d.Name, Desired: d, Current: c, Changes: changes})
		}
	}
	for _, c := range current {
		if !desiredNames[c.Name] {
			r.BucketsToDelete = append(r.BucketsToDelete, c.Name)
		}
	}
	sort.Strings(r.BucketsToDelete)
}

func diffUsers(r *DiffResult, desired []UserSpec, current []UserInfo) {
	cur := make(map[string]UserInfo, len(current))
	for _, u := range current {
		cur[u.Name] = u
	}
	desiredNames := make(map[string]bool, len(desired))

	for _, d := range desired {
		desiredNames[d.Name] = true
		c, ok := cur[d.Name]
		if !ok {
			r.UsersToCreate = append(r.UsersToCreate, d)
			continue
		}
		// The user secret cannot be read back, so password changes are not
		// detectable here; only status and policy attachments are diffed.
		var changes []string
		if d.Enabled != c.Enabled {
			changes = append(changes, "enabled")
		}
		if !stringSetEqual(d.Policies, c.Policies) {
			changes = append(changes, "policies")
		}
		if len(changes) > 0 {
			r.UsersToUpdate = append(r.UsersToUpdate, UserUpdate{Name: d.Name, Desired: d, Current: c, Changes: changes})
		}
	}
	for _, c := range current {
		if !desiredNames[c.Name] {
			r.UsersToDelete = append(r.UsersToDelete, c.Name)
		}
	}
	sort.Strings(r.UsersToDelete)
}

// jsonEqual reports whether two JSON documents are semantically equal,
// independent of key order and insignificant whitespace.
func jsonEqual(a, b []byte) bool {
	na, errA := normalizeJSON(a)
	nb, errB := normalizeJSON(b)
	if errA != nil || errB != nil {
		// Fall back to a raw byte comparison if either side is not valid JSON.
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	return bytes.Equal(na, nb)
}

// normalizeJSON re-encodes JSON into a canonical form (object keys sorted by the
// standard library's map marshaling) for stable comparison.
func normalizeJSON(data []byte) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return []byte("null"), nil
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("normalize json: %w", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("normalize json: %w", err)
	}
	return b, nil
}

// tagsEqual compares two tag maps, treating nil and empty as equal.
func tagsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// stringSetEqual compares two string slices as sets (order-insensitive,
// duplicates ignored).
func stringSetEqual(a, b []string) bool {
	sa := make(map[string]bool, len(a))
	for _, s := range a {
		sa[s] = true
	}
	sb := make(map[string]bool, len(b))
	for _, s := range b {
		sb[s] = true
	}
	if len(sa) != len(sb) {
		return false
	}
	for s := range sa {
		if !sb[s] {
			return false
		}
	}
	return true
}
