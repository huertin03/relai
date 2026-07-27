// Package actions dispara los efectos del menú: cambiar de cuenta de Claude
// y traspasar una sesión a otro CLI. No lee cuotas ni pinta nada.
package actions

import (
	"context"
	"errors"
	"os/exec"

	"github.com/huertin03/relai/internal/providers"
)

// Actor ejecuta las acciones del menú.
type Actor struct {
	Run          providers.Runner
	Look         providers.LookPath
	CswapBin     string
	ContinuesBin string
}

func NewActor(run providers.Runner) Actor {
	return Actor{
		Run:          run,
		Look:         exec.LookPath,
		CswapBin:     "cswap",
		ContinuesBin: "continues",
	}
}

// SwitchAccount cambia la cuenta activa de Claude. Se acciona por `number`,
// no por alias: el alias es opcional en cswap y puede no existir.
func (a Actor) SwitchAccount(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("actions: id de cuenta vacío")
	}
	_, err := a.Run(ctx, a.CswapBin, "switch", id, "--json")
	return err
}

// Handoff traspasa una sesión al CLI destino. continues es de solo lectura
// sobre los ficheros de origen: un fallo aquí no corrompe la sesión.
func (a Actor) Handoff(ctx context.Context, sessionID, target string) error {
	if sessionID == "" || target == "" {
		return errors.New("actions: sesión o destino vacíos")
	}
	_, err := a.Run(ctx, a.ContinuesBin, "resume", sessionID, "--in", target)
	return err
}

// TargetAvailable dice si el CLI destino existe. Se consulta al construir el
// menú, no al hacer clic: un destino ausente se pinta deshabilitado en vez
// de fallar después de que el usuario lo elija.
func (a Actor) TargetAvailable(target string) bool {
	look := a.Look
	if look == nil {
		look = exec.LookPath
	}
	_, err := look(target)
	return err == nil
}
