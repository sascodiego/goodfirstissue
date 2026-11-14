package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The committed example must be exactly what the code generates, or the documentation
// starts describing settings that no longer exist. Regenerate with:
//
//	make config-example
func TestCommittedExampleMatchesCode(t *testing.T) {
	path := filepath.Join("..", "..", "config.example.json")
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	generated, err := Example()
	if err != nil {
		t.Fatalf("Example() error = %v", err)
	}
	if string(onDisk) != string(generated) {
		t.Errorf("config.example.json is stale; run `make config-example`\n--- disk ---\n%s\n--- code ---\n%s", onDisk, generated)
	}
}
