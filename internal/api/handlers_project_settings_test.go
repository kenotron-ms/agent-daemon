package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms/amplifier-app-loom/internal/amplifier"
)

func TestGetProjectSettings_empty(t *testing.T) {
	srv := newTestServer(t)
	tmp := t.TempDir()

	p, err := srv.WorkspaceStore().CreateProject(t.Context(), "test-proj", tmp)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/projects/"+p.ID+"/settings", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got amplifier.ProjectSettings
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Bundle != nil {
		t.Errorf("expected nil bundle for fresh project, got %+v", got.Bundle)
	}
}

func TestPutProjectSettings_writesYAML(t *testing.T) {
	srv := newTestServer(t)
	tmp := t.TempDir()

	p, err := srv.WorkspaceStore().CreateProject(t.Context(), "test-proj2", tmp)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	body := amplifier.ProjectSettings{
		Bundle: &amplifier.BundleSettings{Active: "foundation"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/projects/"+p.ID+"/settings",
		bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".amplifier", "settings.yaml"))
	if err != nil {
		t.Fatalf("read settings.yaml: %v", err)
	}
	if !bytes.Contains(data, []byte("foundation")) {
		t.Errorf("expected 'foundation' in settings.yaml, got: %s", data)
	}
}

func TestGetProjectSettings_afterPut(t *testing.T) {
	srv := newTestServer(t)
	tmp := t.TempDir()
	p, err := srv.WorkspaceStore().CreateProject(t.Context(), "round-trip", tmp)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// PUT settings
	body := amplifier.ProjectSettings{
		Bundle: &amplifier.BundleSettings{Active: "foundation"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/projects/"+p.ID+"/settings", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	// GET and verify round-trip
	req = httptest.NewRequest("GET", "/api/projects/"+p.ID+"/settings", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got amplifier.ProjectSettings
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Bundle == nil || got.Bundle.Active != "foundation" {
		t.Errorf("expected bundle.active=foundation, got %+v", got.Bundle)
	}
}
