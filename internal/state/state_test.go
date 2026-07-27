package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/huertin03/relai/internal/providers"
)

func cuenta(label string, pct int, st providers.Status) providers.Account {
	return providers.Account{
		Label:   label,
		Status:  st,
		Windows: []providers.Window{{Kind: "5h", Pct: pct}},
	}
}

func TestWorstPctIgnoraCuentasSinMedicion(t *testing.T) {
	s := New(85)
	s.Update("claude", []providers.Account{
		cuenta("a", 40, providers.StatusOK),
		cuenta("b", 99, providers.StatusTokenExpired), // no compite
	}, nil)
	pct, ok := s.WorstPct()
	if !ok {
		t.Fatal("esperaba una medición válida")
	}
	if pct != 40 {
		t.Fatalf("esperaba 40, obtuve %d: una cuenta sin medición no debe competir", pct)
	}
}

func TestWorstPctCruzaProveedores(t *testing.T) {
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 40, providers.StatusOK)}, nil)
	s.Update("codex", []providers.Account{cuenta("c", 71, providers.StatusOK)}, nil)
	pct, _ := s.WorstPct()
	if pct != 71 {
		t.Fatalf("esperaba 71, obtuve %d", pct)
	}
}

func TestTitleSinDatos(t *testing.T) {
	s := New(85)
	if got := s.Title(); got != "—" {
		t.Fatalf("sin datos el título debe ser un guion, obtuve %q", got)
	}
}

func TestTitleConDatos(t *testing.T) {
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 78, providers.StatusOK)}, nil)
	if got := s.Title(); got != "78%" {
		t.Fatalf("esperaba 78%%, obtuve %q", got)
	}
}

func TestAvisaUnaSolaVezPorCruce(t *testing.T) {
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 90, providers.StatusOK)}, nil)
	if len(s.PendingAlerts()) != 1 {
		t.Fatal("el primer cruce del umbral debe avisar")
	}
	// Segundo refresco por encima del umbral: no debe volver a avisar.
	s.Update("claude", []providers.Account{cuenta("a", 92, providers.StatusOK)}, nil)
	if got := s.PendingAlerts(); len(got) != 0 {
		t.Fatalf("no debe repetir el aviso mientras siga por encima, obtuve %v", got)
	}
}

func TestVuelveAAvisarTrasBajarDelUmbral(t *testing.T) {
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 90, providers.StatusOK)}, nil)
	s.PendingAlerts()
	s.Update("claude", []providers.Account{cuenta("a", 10, providers.StatusOK)}, nil) // reset de ventana
	if len(s.PendingAlerts()) != 0 {
		t.Fatal("bajar del umbral no debe avisar")
	}
	s.Update("claude", []providers.Account{cuenta("a", 88, providers.StatusOK)}, nil)
	if len(s.PendingAlerts()) != 1 {
		t.Fatal("tras bajar y volver a cruzar, debe avisar de nuevo")
	}
}

func TestUpdateGuardaElError(t *testing.T) {
	s := New(85)
	boom := errors.New("boom")
	s.Update("claude", nil, boom)
	snap := s.Snapshot()
	if !errors.Is(snap.Errors["claude"], boom) {
		t.Fatalf("el error del proveedor debe conservarse, obtuve %v", snap.Errors["claude"])
	}
}

func TestSaveYLoadHacenIdaYVuelta(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "state.json")
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 77, providers.StatusOK)}, nil)
	if err := s.Save(ruta); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	s2 := New(85)
	if err := s2.Load(ruta); err != nil {
		t.Fatalf("Load falló: %v", err)
	}
	pct, ok := s2.WorstPct()
	if !ok || pct != 77 {
		t.Fatalf("esperaba 77 tras recargar, obtuve %d (ok=%v)", pct, ok)
	}
}

func TestLoadFicheroAusenteNoEsError(t *testing.T) {
	s := New(85)
	if err := s.Load(filepath.Join(t.TempDir(), "no-existe.json")); err != nil {
		t.Fatalf("un cache ausente es normal en el primer arranque: %v", err)
	}
}

func TestLoadCacheCorruptoNoEsError(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "state.json")
	os.WriteFile(ruta, []byte("{esto no es json"), 0o600)
	s := New(85)
	if err := s.Load(ruta); err != nil {
		t.Fatalf("un cache corrupto se descarta, no tumba el arranque: %v", err)
	}
	if _, ok := s.WorstPct(); ok {
		t.Fatal("tras un cache corrupto no debe haber datos")
	}
}

func TestLoadNoDisparaAvisos(t *testing.T) {
	// Cargar el cache no debe notificar: al arrancar, un 90% guardado ayer no
	// es un cruce del umbral que haya ocurrido ahora.
	ruta := filepath.Join(t.TempDir(), "state.json")
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 90, providers.StatusOK)}, nil)
	s.PendingAlerts()
	s.Save(ruta)

	s2 := New(85)
	s2.Load(ruta)
	if got := s2.PendingAlerts(); len(got) != 0 {
		t.Fatalf("cargar el cache no debe avisar, obtuve %v", got)
	}
}
