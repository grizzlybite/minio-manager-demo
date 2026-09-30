package state_test

import (
	"testing"

	"github.com/yourorg/minio-manager/internal/state"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desired state.DesiredState
		current state.CurrentState
		check   func(t *testing.T, got state.DiffResult)
	}{
		{
			name:    "create bucket when absent",
			desired: state.DesiredState{Buckets: []state.BucketSpec{{Name: "new-bucket"}}},
			current: state.CurrentState{},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "BucketsToCreate", got.BucketsToCreate, 1)
				wantLen(t, "BucketsToUpdate", got.BucketsToUpdate, 0)
				wantLen(t, "BucketsToDelete", got.BucketsToDelete, 0)
			},
		},
		{
			name:    "delete bucket absent from desired",
			desired: state.DesiredState{},
			current: state.CurrentState{Buckets: []state.BucketInfo{{Name: "stale"}}},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "BucketsToCreate", got.BucketsToCreate, 0)
				wantLen(t, "BucketsToDelete", got.BucketsToDelete, 1)
			},
		},
		{
			name: "update bucket on versioning and tags",
			desired: state.DesiredState{Buckets: []state.BucketSpec{
				{Name: "b", Versioning: true, Tags: map[string]string{"env": "prod"}},
			}},
			current: state.CurrentState{Buckets: []state.BucketInfo{{Name: "b", Versioning: false}}},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "BucketsToUpdate", got.BucketsToUpdate, 1)
				if got := got.BucketsToUpdate[0].Changes; len(got) != 2 {
					t.Errorf("Changes = %v, want [versioning tags]", got)
				}
			},
		},
		{
			name: "no bucket update when equal (nil vs empty tags)",
			desired: state.DesiredState{Buckets: []state.BucketSpec{
				{Name: "b", Versioning: true, Tags: map[string]string{}},
			}},
			current: state.CurrentState{Buckets: []state.BucketInfo{{Name: "b", Versioning: true, Tags: nil}}},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "BucketsToUpdate", got.BucketsToUpdate, 0)
			},
		},
		{
			name: "policy create and json-equal no update",
			desired: state.DesiredState{Policies: []state.PolicySpec{
				{Name: "p1", Document: []byte(`{"Version":"x","Statement":[]}`)},
				{Name: "p2", Document: []byte(`{"Statement":[],"Version":"x"}`)}, // reordered keys
			}},
			current: state.CurrentState{Policies: []state.PolicyInfo{
				{Name: "p2", Document: []byte(`{"Version":"x","Statement":[]}`)},
			}},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "PoliciesToCreate", got.PoliciesToCreate, 1) // p1
				wantLen(t, "PoliciesToUpdate", got.PoliciesToUpdate, 0) // p2 equal modulo key order
			},
		},
		{
			name: "policy update when document differs",
			desired: state.DesiredState{Policies: []state.PolicySpec{
				{Name: "p", Document: []byte(`{"Version":"2012-10-17"}`)},
			}},
			current: state.CurrentState{Policies: []state.PolicyInfo{
				{Name: "p", Document: []byte(`{"Version":"2008-10-17"}`)},
			}},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "PoliciesToUpdate", got.PoliciesToUpdate, 1)
			},
		},
		{
			name: "user policy set equal ignores order",
			desired: state.DesiredState{Users: []state.UserSpec{
				{Name: "u", Enabled: true, Policies: []string{"a", "b"}},
			}},
			current: state.CurrentState{Users: []state.UserInfo{
				{Name: "u", Enabled: true, Policies: []string{"b", "a"}},
			}},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "UsersToUpdate", got.UsersToUpdate, 0)
			},
		},
		{
			name: "user update on enabled and policies",
			desired: state.DesiredState{Users: []state.UserSpec{
				{Name: "u", Enabled: false, Policies: []string{"a"}},
			}},
			current: state.CurrentState{Users: []state.UserInfo{
				{Name: "u", Enabled: true, Policies: []string{"a", "b"}},
			}},
			check: func(t *testing.T, got state.DiffResult) {
				wantLen(t, "UsersToUpdate", got.UsersToUpdate, 1)
				if c := got.UsersToUpdate[0].Changes; len(c) != 2 {
					t.Errorf("Changes = %v, want [enabled policies]", c)
				}
			},
		},
		{
			name:    "empty diff is Empty",
			desired: state.DesiredState{},
			current: state.CurrentState{},
			check: func(t *testing.T, got state.DiffResult) {
				if !got.Empty() {
					t.Error("expected Empty() == true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := state.Diff(tt.desired, tt.current)
			tt.check(t, got)
		})
	}
}

// wantLen is a tiny generic length assertion helper.
func wantLen[T any](t *testing.T, name string, got []T, want int) {
	t.Helper()
	if len(got) != want {
		t.Errorf("%s length = %d, want %d", name, len(got), want)
	}
}
