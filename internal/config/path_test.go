package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPathCumpleLoDocumentado(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(p), "/.config/relai/config.yml") {
		t.Fatalf("la documentación promete ~/.config/relai/config.yml, obtuve %q", p)
	}
}

func TestXDGConfigHomeManda(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgtest")
	p, _ := DefaultPath()
	if p != "/tmp/xdgtest/relai/config.yml" {
		t.Fatalf("XDG_CONFIG_HOME ignorado: %q", p)
	}
}
