package providers

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// guionTransport simula el app-server: responde a initialize y, cuando ve la
// petición de rate limits, emite una notificación intrusa y luego la respuesta.
// La notificación intrusa es deliberada: reproduce lo observado en vivo.
func guionTransport(t *testing.T, respuestaRateLimits string) Transport {
	t.Helper()
	return func(ctx context.Context) (io.ReadCloser, io.WriteCloser, func() error, error) {
		var salida bytes.Buffer
		salida.WriteString(`{"id":1,"result":{"userAgent":"test","codexHome":"/tmp"}}` + "\n")
		salida.WriteString(`{"method":"remoteControl/status/changed","params":{"status":"disabled"}}` + "\n")
		salida.WriteString(respuestaRateLimits + "\n")
		return io.NopCloser(&salida), nopWriteCloser{io.Discard}, func() error { return nil }, nil
	}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestCodexParseaElFixtureReal(t *testing.T) {
	b, err := os.ReadFile("../../testdata/codex_ratelimits.json")
	if err != nil {
		t.Fatalf("no se pudo leer el fixture: %v", err)
	}
	// El fixture está indentado; el protocolo es una línea por mensaje.
	linea := strings.Join(strings.Fields(string(b)), "")

	p := CodexProvider{Transport: guionTransport(t, linea)}
	accs, err := p.Usage(context.Background())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(accs) != 1 {
		t.Fatalf("esperaba 1 cuenta, obtuve %d", len(accs))
	}
	a := accs[0]
	if a.Plan != "free" {
		t.Errorf("Plan debe venir de planType, obtuve %q", a.Plan)
	}
	if a.Status != StatusOK {
		t.Errorf("esperaba StatusOK, obtuve %v", a.Status)
	}
	if len(a.Windows) != 1 {
		t.Fatalf("el fixture tiene primary y secondary=null: esperaba 1 ventana, obtuve %d", len(a.Windows))
	}
	if a.Windows[0].Kind != "30d" {
		t.Errorf("43200 minutos deben mapear a 30d, obtuve %q", a.Windows[0].Kind)
	}
	if a.Windows[0].Pct != 0 {
		t.Errorf("usedPercent del fixture es 0, obtuve %d", a.Windows[0].Pct)
	}
}

func TestCodexIgnoraNotificacionesIntrusas(t *testing.T) {
	// Si el cliente asumiera "la siguiente línea es mi respuesta", la
	// notificación de remoteControl lo rompería. Este test lo blinda.
	linea := `{"id":2,"result":{"rateLimits":{"planType":"pro","primary":{"usedPercent":55,"windowDurationMins":300,"resetsAt":1787749472},"secondary":{"usedPercent":12,"windowDurationMins":10080,"resetsAt":1788049472}}}}`
	p := CodexProvider{Transport: guionTransport(t, linea)}
	accs, err := p.Usage(context.Background())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(accs[0].Windows) != 2 {
		t.Fatalf("esperaba 2 ventanas (primary y secondary), obtuve %d", len(accs[0].Windows))
	}
	w, _ := accs[0].Worst()
	if w.Pct != 55 || w.Kind != "5h" {
		t.Errorf("la peor debe ser 5h/55, obtuve %s/%d", w.Kind, w.Pct)
	}
}

func TestKindFromMinutes(t *testing.T) {
	// 1440 → "1d", no "24h": la regla de días va primero a propósito porque
	// "1d" se lee mejor que "24h" en un menú.
	casos := map[int64]string{300: "5h", 10080: "7d", 43200: "30d", 60: "1h", 1440: "1d"}
	for mins, esperado := range casos {
		if got := kindFromMinutes(mins); got != esperado {
			t.Errorf("kindFromMinutes(%d) = %q, esperaba %q", mins, got, esperado)
		}
	}
}

func TestCodexSinRespuestaEsError(t *testing.T) {
	vacio := func(ctx context.Context) (io.ReadCloser, io.WriteCloser, func() error, error) {
		return io.NopCloser(strings.NewReader("")), nopWriteCloser{io.Discard}, func() error { return nil }, nil
	}
	p := CodexProvider{Transport: vacio}
	if _, err := p.Usage(context.Background()); err == nil {
		t.Fatal("esperaba error cuando el servidor no responde")
	}
}
