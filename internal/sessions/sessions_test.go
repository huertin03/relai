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
	// Líneas 3 y 4 están corruptas (sintaxis inválida y tipo equivocado) y deben descartarse.
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

func TestRecentDescartaLineaVacia(t *testing.T) {
	// Línea válida, línea vacía, línea válida
	payload := []byte(`{"id":"aaa","source":"claude","repo":"r1","summary":"s1","updatedAt":"2026-07-27T09:30:00Z"}

{"id":"bbb","source":"claude","repo":"r2","summary":"s2","updatedAt":"2026-07-27T09:30:00Z"}`)
	l := NewLister(fakeRunner(payload, nil))
	ss, err := l.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(ss) != 2 {
		t.Fatalf("esperaba 2 sesiones (línea vacía descartada), obtuve %d", len(ss))
	}
}

func TestRecentDescartaJSONValidoConIDVacio(t *testing.T) {
	// Línea válida, línea con ID vacío, línea válida
	payload := []byte(`{"id":"aaa","source":"claude","repo":"r1","summary":"s1","updatedAt":"2026-07-27T09:30:00Z"}
{"id":"","source":"claude","repo":"r2","summary":"s2","updatedAt":"2026-07-27T09:30:00Z"}
{"id":"bbb","source":"claude","repo":"r3","summary":"s3","updatedAt":"2026-07-27T09:30:00Z"}`)
	l := NewLister(fakeRunner(payload, nil))
	ss, err := l.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(ss) != 2 {
		t.Fatalf("esperaba 2 sesiones (ID vacío descartado), obtuve %d", len(ss))
	}
}

func TestRecentIgnoraUpdatedAtMalformado(t *testing.T) {
	// Línea con UpdatedAt malformado
	payload := []byte(`{"id":"ccc","source":"claude","repo":"r","summary":"s","updatedAt":"invalid-date"}`)
	l := NewLister(fakeRunner(payload, nil))
	ss, err := l.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(ss) != 1 {
		t.Fatalf("esperaba 1 sesión (UpdatedAt malformado ignora pero no falla), obtuve %d", len(ss))
	}
	if ss[0].ID != "ccc" {
		t.Errorf("ID inesperado: %q", ss[0].ID)
	}
	if !ss[0].UpdatedAt.IsZero() {
		t.Error("UpdatedAt malformado debe quedar en zero")
	}
}

func TestRecentDescartaJSONTipoEquivocado(t *testing.T) {
	// JSON sintácticamente válido pero con updatedAt como número (tipo equivocado).
	// Sin la rama de error del json.Unmarshal, raw quedaría con ID populado y se colaría.
	payload := []byte(`{"id":"bad-type","source":"claude","repo":"malformed","summary":"tipo equivocado","updatedAt":1234567890}`)
	l := NewLister(fakeRunner(payload, nil))
	ss, err := l.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(ss) != 0 {
		t.Fatalf("esperaba 0 sesiones (tipo equivocado descartado), obtuve %d", len(ss))
	}
}

func TestRecentInvocaContinuesConArgumentosCorrectos(t *testing.T) {
	// Captura name y args para verificar que Recent invoca realmente el comando correcto.
	var capturedName string
	var capturedArgs []string
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	}
	l := NewLister(runner)
	l.Recent(context.Background(), 5)
	if capturedName != "continues" {
		t.Errorf("esperaba binary 'continues', obtuve %q", capturedName)
	}
	expectedArgs := []string{"list", "--source", "claude", "--jsonl"}
	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("esperaba %d args, obtuve %d: %v", len(expectedArgs), len(capturedArgs), capturedArgs)
	}
	for i, expected := range expectedArgs {
		if capturedArgs[i] != expected {
			t.Errorf("arg[%d]: esperaba %q, obtuve %q", i, expected, capturedArgs[i])
		}
	}
}

func TestRecentUsaBinSobrescrito(t *testing.T) {
	// Verifica que Lister.Bin se usa cuando se sobrescribe.
	var capturedName string
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		capturedName = name
		return []byte(""), nil
	}
	l := NewLister(runner)
	l.Bin = "custom-continues"
	l.Recent(context.Background(), 5)
	if capturedName != "custom-continues" {
		t.Errorf("esperaba binary 'custom-continues', obtuve %q", capturedName)
	}
}
