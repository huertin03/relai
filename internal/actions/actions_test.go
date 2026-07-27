package actions

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/huertin03/relai/internal/providers"
)

// espia registra el comando invocado para poder afirmar sobre él.
type espia struct {
	nombre string
	args   []string
	err    error
}

func (e *espia) runner() providers.Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		e.nombre = name
		e.args = args
		return nil, e.err
	}
}

func TestSwitchUsaElNumeroNoElAlias(t *testing.T) {
	e := &espia{}
	a := NewActor(e.runner())
	if err := a.SwitchAccount(context.Background(), "2"); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if e.nombre != "cswap" {
		t.Errorf("esperaba cswap, obtuve %q", e.nombre)
	}
	esperado := []string{"switch", "2", "--json"}
	if len(e.args) != 3 || e.args[0] != esperado[0] || e.args[1] != esperado[1] || e.args[2] != esperado[2] {
		t.Errorf("args inesperados: %v (esperaba %v)", e.args, esperado)
	}
}

func TestSwitchIDVacioEsError(t *testing.T) {
	e := &espia{}
	a := NewActor(e.runner())
	if err := a.SwitchAccount(context.Background(), ""); err == nil {
		t.Fatal("un ID vacío debe rechazarse antes de invocar nada")
	}
	if e.nombre != "" {
		t.Errorf("no debió invocarse ningún comando, se invocó %q", e.nombre)
	}
}

func TestHandoffConstruyeElComando(t *testing.T) {
	e := &espia{}
	a := NewActor(e.runner())
	if err := a.Handoff(context.Background(), "abc123", "codex"); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if e.nombre != "continues" {
		t.Errorf("esperaba continues, obtuve %q", e.nombre)
	}
	esperado := []string{"resume", "abc123", "--in", "codex"}
	for i := range esperado {
		if i >= len(e.args) || e.args[i] != esperado[i] {
			t.Fatalf("args inesperados: %v (esperaba %v)", e.args, esperado)
		}
	}
}

func TestHandoffPropagaError(t *testing.T) {
	e := &espia{err: errors.New("boom")}
	a := NewActor(e.runner())
	if err := a.Handoff(context.Background(), "abc", "codex"); err == nil {
		t.Fatal("esperaba que el error del subproceso se propagara")
	}
}

func TestTargetAvailableComprueba(t *testing.T) {
	a := NewActor((&espia{}).runner())
	a.Look = func(name string) (string, error) {
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", exec.ErrNotFound
	}
	if !a.TargetAvailable("codex") {
		t.Error("codex debería estar disponible")
	}
	if a.TargetAvailable("opencode") {
		t.Error("opencode no debería estar disponible")
	}
}
