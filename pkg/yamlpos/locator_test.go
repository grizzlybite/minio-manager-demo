package yamlpos

import (
	"reflect"
	"testing"
)

const sampleYAML = `minio:
  endpoint: e
buckets:
  - name: a
    region: r
  - name: b
users:
  - name: alice
    password: p
`

func TestLineOfKey(t *testing.T) {
	t.Parallel()

	root, err := Parse([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	tests := []struct {
		name string
		path []string
		want int
	}{
		{"nested mapping key", []string{"minio", "endpoint"}, 2},
		{"first bucket name", []string{"buckets", "0", "name"}, 4},
		{"second bucket name", []string{"buckets", "1", "name"}, 6},
		{"user password by index", []string{"users", "0", "password"}, 9},
		{"top-level sequence node (line of first element)", []string{"buckets"}, 4},
		{"missing key", []string{"nope"}, 0},
		{"missing nested key", []string{"minio", "ghost"}, 0},
		{"index out of range", []string{"buckets", "5"}, 0},
		{"non-numeric index on sequence", []string{"buckets", "x"}, 0},
		{"descend into scalar", []string{"minio", "endpoint", "deeper"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := LineOfKey(root, tt.path...); got != tt.want {
				t.Errorf("LineOfKey(%v) = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}

func TestSequenceLines(t *testing.T) {
	t.Parallel()

	root, err := Parse([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got, want := SequenceLines(root, "buckets"), []int{4, 6}; !reflect.DeepEqual(got, want) {
		t.Errorf("SequenceLines(buckets) = %v, want %v", got, want)
	}
	if got, want := SequenceLines(root, "users"), []int{8}; !reflect.DeepEqual(got, want) {
		t.Errorf("SequenceLines(users) = %v, want %v", got, want)
	}
	if got := SequenceLines(root, "policies"); got != nil {
		t.Errorf("SequenceLines(policies) = %v, want nil", got)
	}
	if got := SequenceLines(root, "minio"); got != nil {
		t.Errorf("SequenceLines(minio) on mapping = %v, want nil", got)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	if _, err := Parse([]byte("key: : : invalid")); err == nil {
		t.Error("Parse(invalid) expected error, got nil")
	}

	root, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("Parse(empty): %v", err)
	}
	if root != nil {
		t.Errorf("Parse(empty) root = %v, want nil", root)
	}
	if got := LineOfKey(root, "anything"); got != 0 {
		t.Errorf("LineOfKey on nil root = %d, want 0", got)
	}
}
