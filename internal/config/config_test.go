package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultTieneValoresUsables(t *testing.T) {
	c := Default()
	if c.RefreshInterval != 3*time.Minute {
		t.Errorf("intervalo por defecto inesperado: %v", c.RefreshInterval)
	}
	if c.AlertThreshold != 85 {
		t.Errorf("umbral por defecto inesperado: %d", c.AlertThreshold)
	}
	if len(c.Providers) != 2 {
		t.Errorf("esperaba 2 proveedores por defecto, obtuve %v", c.Providers)
	}
	if c.Handoff.RecentSessions != 5 {
		t.Errorf("recent_sessions por defecto inesperado: %d", c.Handoff.RecentSessions)
	}
}

func TestLoadFicheroAusenteDevuelveDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "no-existe.yml"))
	if err != nil {
		t.Fatalf("un fichero ausente no es error: %v", err)
	}
	if c.AlertThreshold != 85 {
		t.Errorf("esperaba los defaults, obtuve umbral %d", c.AlertThreshold)
	}
}

func TestLoadSobreescribeSoloLoPresente(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "config.yml")
	contenido := "refresh_interval: 30s\nalert_threshold: 60\n"
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(ruta)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if c.RefreshInterval != 30*time.Second {
		t.Errorf("intervalo no sobreescrito: %v", c.RefreshInterval)
	}
	if c.AlertThreshold != 60 {
		t.Errorf("umbral no sobreescrito: %d", c.AlertThreshold)
	}
	// Lo ausente conserva su default.
	if c.Handoff.RecentSessions != 5 {
		t.Errorf("recent_sessions debía seguir en 5, obtuve %d", c.Handoff.RecentSessions)
	}
}

func TestLoadYAMLInvalidoEsError(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "config.yml")
	os.WriteFile(ruta, []byte("refresh_interval: [esto no es una duración"), 0o600)
	if _, err := Load(ruta); err == nil {
		t.Fatal("esperaba error con YAML inválido")
	}
}

func TestUmbralFueraDeRangoSeCorrige(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "config.yml")
	os.WriteFile(ruta, []byte("alert_threshold: 500\n"), 0o600)
	c, err := Load(ruta)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if c.AlertThreshold != 100 {
		t.Errorf("un umbral de 500 debe recortarse a 100, obtuve %d", c.AlertThreshold)
	}
}

// Tests para las otras tres correcciones de sanitize que no estaban cubiertas.

func TestUmbralNegativoSeCorrigeA0(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "config.yml")
	os.WriteFile(ruta, []byte("alert_threshold: -10\n"), 0o600)
	c, err := Load(ruta)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if c.AlertThreshold != 0 {
		t.Errorf("un umbral de -10 debe recortarse a 0, obtuve %d", c.AlertThreshold)
	}
}

func TestIntervaloMenorAlMinimoSeCorrige(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "config.yml")
	os.WriteFile(ruta, []byte("refresh_interval: 5s\n"), 0o600)
	c, err := Load(ruta)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if c.RefreshInterval != 10*time.Second {
		t.Errorf("un intervalo de 5s debe recortarse a 10s, obtuve %v", c.RefreshInterval)
	}
}

func TestRecentSessionsMenorA1SeCorrige(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "config.yml")
	os.WriteFile(ruta, []byte("handoff:\n  recent_sessions: 0\n"), 0o600)
	c, err := Load(ruta)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if c.Handoff.RecentSessions != 1 {
		t.Errorf("recent_sessions de 0 debe recortarse a 1, obtuve %d", c.Handoff.RecentSessions)
	}
}
