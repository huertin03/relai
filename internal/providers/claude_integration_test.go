package providers

import (
	"context"
	"os/exec"
	"testing"
)

// TestProveedorClaudeContraCswapReal es la guardia que faltaba. El fixture
// original se derivó del código fuente de cswap en vez de una ejecución, y
// erró en dos tipos: `pct` llega como float y `resetsAt` como cadena ISO. Con
// `int` y `*int64` el Unmarshal del envelope ENTERO fallaba, así que la
// bandeja no mostraba ni una cuenta. Ningún test con fixture lo habría
// detectado, porque el fixture compartía el error.
func TestProveedorClaudeContraCswapReal(t *testing.T) {
	if _, err := exec.LookPath("cswap"); err != nil {
		t.Skip("cswap no instalado")
	}
	p := NewClaudeProvider(ExecRunner)
	accs, err := p.Usage(context.Background())
	if err != nil {
		t.Fatalf("FALLO CONTRA CSWAP REAL: %v", err)
	}
	t.Logf("cuentas: %d", len(accs))
	for _, a := range accs {
		w, ok := a.Worst()
		t.Logf("  %s status=%v ventanas=%d peor=%s/%d ok=%v", a.Label, a.Status, len(a.Windows), w.Kind, w.Pct, ok)
	}
}
