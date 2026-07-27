package providers

import (
	"context"
	"errors"
	"os"
	"testing"
)

// fakeRunner devuelve siempre el mismo payload, ignorando el comando.
func fakeRunner(payload []byte, err error) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return payload, err
	}
}

func cargarFixture(t *testing.T, ruta string) []byte {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer el fixture %s: %v", ruta, err)
	}
	return b
}

func TestClaudeParseaElFixtureReal(t *testing.T) {
	p := NewClaudeProvider(fakeRunner(cargarFixture(t, "../../testdata/cswap_list.json"), nil))
	accs, err := p.Usage(context.Background())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(accs) != 2 {
		t.Fatalf("esperaba 2 cuentas, obtuve %d", len(accs))
	}

	a := accs[0]
	if a.ID != "1" {
		t.Errorf("ID debe ser el `number` como string, obtuve %q", a.ID)
	}
	if a.Label != "work" {
		t.Errorf("Label debe usar el alias cuando existe, obtuve %q", a.Label)
	}
	if a.Status != StatusOK {
		t.Errorf("esperaba StatusOK, obtuve %v", a.Status)
	}
	if !a.Active {
		t.Error("la cuenta 1 debe estar activa")
	}
	// 4 ventanas: fiveHour, sevenDay, spend, y una scoped.
	if len(a.Windows) != 4 {
		t.Fatalf("esperaba 4 ventanas, obtuve %d", len(a.Windows))
	}
	w, _ := a.Worst()
	if w.Pct != 78 || w.Kind != "5h" {
		t.Errorf("la peor ventana debe ser 5h/78, obtuve %s/%d", w.Kind, w.Pct)
	}
	if a.AgeS != 42.0 {
		t.Errorf("AgeS debe venir de usageAgeSeconds, obtuve %v", a.AgeS)
	}
}

func TestClaudeSinAliasCaeAlEmail(t *testing.T) {
	p := NewClaudeProvider(fakeRunner(cargarFixture(t, "../../testdata/cswap_list.json"), nil))
	accs, _ := p.Usage(context.Background())
	b := accs[1]
	if b.Label != "dos@example.com" {
		t.Errorf("sin alias, Label debe caer al email, obtuve %q", b.Label)
	}
	if !b.Disabled {
		t.Error("la cuenta 2 está `disabled` en el fixture")
	}
	if b.Status != StatusTokenExpired {
		t.Errorf("esperaba StatusTokenExpired, obtuve %v", b.Status)
	}
	if b.ShowsPct() {
		t.Error("una cuenta con token caducado nunca muestra porcentaje")
	}
}

func TestClaudeRechazaSchemaVersionDesconocida(t *testing.T) {
	p := NewClaudeProvider(fakeRunner([]byte(`{"schemaVersion":2,"accounts":[]}`), nil))
	_, err := p.Usage(context.Background())
	if err == nil {
		t.Fatal("esperaba error con schemaVersion 2: parsear a ciegas es peor que fallar")
	}
}

func TestClaudeEnvelopeVacioNoEsError(t *testing.T) {
	p := NewClaudeProvider(fakeRunner([]byte(`{"schemaVersion":1,"activeAccountNumber":null,"accounts":[]}`), nil))
	accs, err := p.Usage(context.Background())
	if err != nil {
		t.Fatalf("cero cuentas es un estado válido, no un error: %v", err)
	}
	if len(accs) != 0 {
		t.Fatalf("esperaba 0 cuentas, obtuve %d", len(accs))
	}
}

func TestClaudePropagaErrBinaryMissing(t *testing.T) {
	p := NewClaudeProvider(fakeRunner(nil, ErrBinaryMissing))
	_, err := p.Usage(context.Background())
	if !errors.Is(err, ErrBinaryMissing) {
		t.Fatalf("esperaba ErrBinaryMissing, obtuve %v", err)
	}
}

func TestClaudeJSONTruncadoEsError(t *testing.T) {
	p := NewClaudeProvider(fakeRunner([]byte(`{"schemaVersion":1,"acc`), nil))
	if _, err := p.Usage(context.Background()); err == nil {
		t.Fatal("esperaba error con JSON truncado")
	}
}
