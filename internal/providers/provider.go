// Package providers define el modelo de cuotas y la interfaz que implementa
// cada proveedor (Claude, Codex). No contiene lógica de presentación.
package providers

import (
	"context"
	"fmt"
	"time"
)

// Status explica por qué una cuenta no tiene medición utilizable.
// Existe porque pintar todo fallo como 0% mentiría al usuario justo
// cuando más importa acertar.
type Status int

const (
	StatusOK                  Status = iota
	StatusTokenExpired               // el token caducó mientras lo tenía Claude Code
	StatusAPIKey                     // cuenta de API key: no tiene cuota de suscripción
	StatusKeychainUnavailable        // Keychain ilegible
	StatusNoCredentials
	StatusFetchFailed // la medición falló o el contrato no se reconoce
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusTokenExpired:
		return "token caducado"
	case StatusAPIKey:
		return "API key (sin cuota de plan)"
	case StatusKeychainUnavailable:
		return "Keychain ilegible"
	case StatusNoCredentials:
		return "sin credenciales"
	case StatusFetchFailed:
		return "sin datos"
	default:
		return fmt.Sprintf("estado desconocido (%d)", int(s))
	}
}

// Window es una ventana de cuota. Pct es SIEMPRE consumido (0-100), nunca
// restante: todo el display asume "más alto = peor".
type Window struct {
	Kind     string // "5h" | "7d" | "30d" | "spend" | "scoped" | genérico
	Name     string // solo para "scoped": el modelo. Vacío en el resto.
	Pct      int
	ResetsAt time.Time // cero si el proveedor no la da
}

// Account es una cuenta de un proveedor con todas sus ventanas.
type Account struct {
	ID        string // clave para accionar sobre ella (cswap: `number`)
	Label     string // alias si existe, si no email
	Email     string
	Org       string
	Plan      string // planType cuando el proveedor lo expone
	Windows   []Window
	Active    bool
	Disabled  bool
	Status    Status
	FetchedAt time.Time // de la fuente, no del reloj local
	AgeS      float64   // de la fuente
}

// Worst devuelve la ventana más consumida. Es función y no campo: derivarla
// al pintar evita que el modelo mienta cuando cambia el conjunto de ventanas.
func (a Account) Worst() (Window, bool) {
	if len(a.Windows) == 0 {
		return Window{}, false
	}
	worst := a.Windows[0]
	for _, w := range a.Windows[1:] {
		if w.Pct > worst.Pct {
			worst = w
		}
	}
	return worst, true
}

// ShowsPct indica si esta cuenta tiene una medición que se pueda enseñar.
func (a Account) ShowsPct() bool { return a.Status == StatusOK }

// Provider es una fuente de cuotas. Añadir un proveedor nuevo es añadir un
// fichero que implemente esta interfaz; nada más cambia.
type Provider interface {
	Name() string
	Usage(ctx context.Context) ([]Account, error)
}
