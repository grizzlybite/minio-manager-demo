package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectConfigPaths(t *testing.T) {
	dir := t.TempDir()
	// Create files out of alphabetical order to verify sorting.
	for _, n := range []string{"prod.yaml", "dev.yaml", "qa.yml", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("minio: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("explicit files preserve order", func(t *testing.T) {
		got, err := collectConfigPaths([]string{"b.yaml", "a.yaml"}, "")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"b.yaml", "a.yaml"}
		if !equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("dir sorted and filtered to yaml/yml", func(t *testing.T) {
		got, err := collectConfigPaths(nil, dir)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			filepath.Join(dir, "dev.yaml"),
			filepath.Join(dir, "prod.yaml"),
			filepath.Join(dir, "qa.yml"),
		}
		if !equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("explicit files come before dir and dedup", func(t *testing.T) {
		explicit := filepath.Join(dir, "dev.yaml") // also present in dir
		got, err := collectConfigPaths([]string{explicit}, dir)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			filepath.Join(dir, "dev.yaml"), // explicit, first, not repeated
			filepath.Join(dir, "prod.yaml"),
			filepath.Join(dir, "qa.yml"),
		}
		if !equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("empty dir errors", func(t *testing.T) {
		empty := t.TempDir()
		if _, err := collectConfigPaths(nil, empty); err == nil {
			t.Fatal("expected error for dir without yaml files")
		}
	})

	t.Run("missing dir errors", func(t *testing.T) {
		if _, err := collectConfigPaths(nil, filepath.Join(dir, "nope")); err == nil {
			t.Fatal("expected error for missing dir")
		}
	})
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
