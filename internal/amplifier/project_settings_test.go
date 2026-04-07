package amplifier_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ms/amplifier-app-loom/internal/amplifier"
)

func TestReadProjectSettings_missing(t *testing.T) {
	s, err := amplifier.ReadProjectSettings("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if s.Bundle != nil {
		t.Error("expected nil Bundle for missing file")
	}
}

func TestReadWriteProjectSettings_roundtrip(t *testing.T) {
	dir := t.TempDir()
	amplifierDir := filepath.Join(dir, ".amplifier")
	if err := os.MkdirAll(amplifierDir, 0755); err != nil {
		t.Fatal(err)
	}

	in := amplifier.ProjectSettings{
		Bundle: &amplifier.BundleSettings{
			Active: "foundation",
			App:    []string{"git+https://github.com/microsoft/lifeos@main"},
		},
		Routing: &amplifier.RoutingSettings{Matrix: "balanced"},
	}

	if err := amplifier.WriteProjectSettings(dir, in); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := amplifier.ReadProjectSettings(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if out.Bundle == nil || out.Bundle.Active != "foundation" {
		t.Errorf("Bundle.Active: got %v", out.Bundle)
	}
	if out.Routing == nil || out.Routing.Matrix != "balanced" {
		t.Errorf("Routing.Matrix: got %v", out.Routing)
	}
}
