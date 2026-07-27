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

// A partir de aquí: tests añadidos más allá de la lista exacta del brief,
// para cerrar huecos que la verificación por mutación dejó al descubierto.

func TestAvisaExactamenteEnElUmbral(t *testing.T) {
	// El umbral es inclusivo: llegar exactamente a él ya cuenta como cruce,
	// no hace falta superarlo. Sin este test, `pct >= threshold` podía
	// mutarse a `pct > threshold` sin que ningún test lo notara.
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 85, providers.StatusOK)}, nil)
	if got := s.PendingAlerts(); len(got) != 1 {
		t.Fatalf("pct igual al umbral debe avisar (umbral inclusive), obtuve %v", got)
	}
}

func TestUpdateLimpiaElErrorTrasUnRefrescoOK(t *testing.T) {
	// Un refresco correcto debe borrar el error de un ciclo anterior; si no,
	// la bandeja seguiría enseñando un fallo ya resuelto.
	s := New(85)
	s.Update("claude", nil, errors.New("boom"))
	s.Update("claude", []providers.Account{cuenta("a", 40, providers.StatusOK)}, nil)
	snap := s.Snapshot()
	if _, existe := snap.Errors["claude"]; existe {
		t.Fatalf("el error debe limpiarse tras un refresco correcto, obtuve %v", snap.Errors["claude"])
	}
}

func TestSnapshotEsCopiaIndependiente(t *testing.T) {
	// Snapshot existe para que la bandeja pinte sin mantener el mutex
	// cogido; si compartiera el slice con el estado interno, escribir sobre
	// el snapshot corrompería (sin mutex) los datos que el ticker sigue
	// actualizando.
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 40, providers.StatusOK)}, nil)
	snap := s.Snapshot()
	snap.ByProvider["claude"][0] = cuenta("hackeada", 999, providers.StatusOK)
	pct, _ := s.WorstPct()
	if pct != 40 {
		t.Fatalf("mutar el snapshot no debe afectar al estado interno, obtuve %d", pct)
	}
}

func TestSaveEsAtomica(t *testing.T) {
	// Save debe escribir en un temporal y renombrarlo sobre el destino, no
	// escribir directamente: un corte de luz a mitad de una escritura directa
	// deja el cache truncado. Se prepara basura en la ruta ".tmp" de antes:
	// una escritura directa la ignoraría y la dejaría intacta; la escritura
	// atómica la sobreescribe y luego la hace desaparecer al renombrarla.
	ruta := filepath.Join(t.TempDir(), "state.json")
	tmpRuta := ruta + ".tmp"
	if err := os.WriteFile(tmpRuta, []byte("basura"), 0o600); err != nil {
		t.Fatalf("no se pudo preparar el temporal previo: %v", err)
	}
	s := New(85)
	s.Update("claude", []providers.Account{cuenta("a", 50, providers.StatusOK)}, nil)
	if err := s.Save(ruta); err != nil {
		t.Fatalf("Save falló: %v", err)
	}
	if _, err := os.Stat(tmpRuta); !os.IsNotExist(err) {
		t.Fatalf("Save debe renombrar el temporal sobre el destino; %s no debería sobrevivir (err=%v)", tmpRuta, err)
	}
}
