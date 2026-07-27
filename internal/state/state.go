// Package state agrega lo que devuelven los proveedores y decide cuándo
// avisar. No lanza subprocesos ni pinta: solo mantiene el estado.
package state

import (
	"fmt"
	"sync"

	"github.com/huertin03/relai/internal/providers"
)

// Snapshot es una vista inmutable del estado, para que la bandeja pinte sin
// mantener el mutex cogido.
type Snapshot struct {
	ByProvider map[string][]providers.Account
	Errors     map[string]error
}

type State struct {
	mu        sync.Mutex
	threshold int
	accounts  map[string][]providers.Account
	errs      map[string]error
	// avisado recuerda qué proveedores ya cruzaron el umbral, para emitir un
	// solo aviso por cruce en vez de uno por ciclo de refresco. Vive solo en
	// memoria: tras un reinicio, el primer cruce vuelve a avisar.
	avisado  map[string]bool
	pendient []string
}

func New(threshold int) *State {
	return &State{
		threshold: threshold,
		accounts:  map[string][]providers.Account{},
		errs:      map[string]error{},
		avisado:   map[string]bool{},
	}
}

// Update reemplaza los datos de un proveedor y recalcula si toca avisar.
func (s *State) Update(provider string, accs []providers.Account, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.errs[provider] = err
	} else {
		delete(s.errs, provider)
		s.accounts[provider] = accs
	}

	pct, ok := worstOf(accs)
	if !ok {
		return
	}
	switch {
	case pct >= s.threshold && !s.avisado[provider]:
		s.avisado[provider] = true
		s.pendient = append(s.pendient, fmt.Sprintf("%s al %d%%", provider, pct))
	case pct < s.threshold:
		// Histéresis: al bajar se rearma, de modo que el próximo cruce vuelve
		// a avisar sin gotear un aviso por refresco mientras sigue arriba.
		s.avisado[provider] = false
	}
}

// PendingAlerts devuelve los avisos acumulados y los consume.
func (s *State) PendingAlerts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.pendient
	s.pendient = nil
	return out
}

// WorstPct es el porcentaje consumido más alto entre todos los proveedores.
func (s *State) WorstPct() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	worst, found := 0, false
	for _, accs := range s.accounts {
		if pct, ok := worstOf(accs); ok && (!found || pct > worst) {
			worst, found = pct, true
		}
	}
	return worst, found
}

// Title es el texto del icono de bandeja.
func (s *State) Title() string {
	if pct, ok := s.WorstPct(); ok {
		return fmt.Sprintf("%d%%", pct)
	}
	return "—"
}

func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := Snapshot{
		ByProvider: make(map[string][]providers.Account, len(s.accounts)),
		Errors:     make(map[string]error, len(s.errs)),
	}
	for k, v := range s.accounts {
		cp := make([]providers.Account, len(v))
		copy(cp, v)
		snap.ByProvider[k] = cp
	}
	for k, v := range s.errs {
		snap.Errors[k] = v
	}
	return snap
}

// worstOf ignora las cuentas sin medición: un fallo no es un 0%.
func worstOf(accs []providers.Account) (int, bool) {
	worst, found := 0, false
	for _, a := range accs {
		if !a.ShowsPct() {
			continue
		}
		if w, ok := a.Worst(); ok && (!found || w.Pct > worst) {
			worst, found = w.Pct, true
		}
	}
	return worst, found
}
