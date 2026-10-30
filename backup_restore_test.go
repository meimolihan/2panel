package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return path
}

func TestCheckZipRequiresRootDatabase(t *testing.T) {
	nested := writeTestZip(t, map[string]string{"sub/2panel.db": "x"})
	if err := checkZip(nested); err == nil {
		t.Fatal("nested 2panel.db should be rejected")
	}

	root := writeTestZip(t, map[string]string{backupDBName: "x", "log/a.log": "a"})
	if err := checkZip(root); err != nil {
		t.Fatalf("root 2panel.db should be accepted: %v", err)
	}
}
