package sessions

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/huertin03/relai/internal/providers"
)

func fakeRunner(payload []byte, err error) providers.Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return payload, err
	}
}

func TestRecentParseaElFixture(t *testing.T) {
	b, err := os.ReadFile("../../testdata/continues_sessions.jsonl")
	if err != nil {
		t.Fatalf("no se pudo leer el fixture: %v", err)
	}
	l := NewLister(fakeRunner(b, nil))
	ss, err := l.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	// La tercera línea está corrupta y debe descartarse sin romper el resto.
	if len(ss) != 2 {
		t.Fatalf("esperaba 2 sesiones válidas, obtuve %d", len(ss))
	}
	if ss[0].ID != "bc4759ad-7f10-4bef-9536-a8838aa458e3" {
		t.Errorf("ID inesperado: %q", ss[0].ID)
	}
	if ss[0].Summary != "Diseño de Relai" {
		t.Errorf("Summary inesperado: %q", ss[0].Summary)
	}
	if ss[0].Repo != "u/proj" {
		t.Errorf("Repo inesperado: %q", ss[0].Repo)
	}
	if ss[0].UpdatedAt.IsZero() {
		t.Error("UpdatedAt debe parsearse")
	}
}

func TestRecentRespetaElLimite(t *testing.T) {
	b, _ := os.ReadFile("../../testdata/continues_sessions.jsonl")
	l := NewLister(fakeRunner(b, nil))
	ss, err := l.Recent(context.Background(), 1)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(ss) != 1 {
		t.Fatalf("esperaba 1 sesión, obtuve %d", len(ss))
	}
}

func TestRecentPropagaErrBinaryMissing(t *testing.T) {
	l := NewLister(fakeRunner(nil, providers.ErrBinaryMissing))
	if _, err := l.Recent(context.Background(), 5); !errors.Is(err, providers.ErrBinaryMissing) {
		t.Fatalf("esperaba ErrBinaryMissing, obtuve %v", err)
	}
}

func TestRecentSalidaVaciaNoEsError(t *testing.T) {
	l := NewLister(fakeRunner([]byte(""), nil))
	ss, err := l.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("sin sesiones no es error: %v", err)
	}
	if len(ss) != 0 {
		t.Fatalf("esperaba 0, obtuve %d", len(ss))
	}
}
