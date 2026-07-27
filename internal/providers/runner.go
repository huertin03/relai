package providers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrBinaryMissing distingue "la herramienta no está instalada" de "la
// herramienta falló". La bandeja los pinta distinto: uno ofrece instalar,
// el otro muestra el error.
var ErrBinaryMissing = errors.New("binario no encontrado en PATH")

// Runner ejecuta un comando y devuelve su stdout. Es un tipo función para
// que los tests inyecten respuestas sin lanzar procesos reales.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// LookPath resuelve un binario en PATH. Inyectable por el mismo motivo.
type LookPath func(name string) (string, error)

// ExecRunner es la implementación real de Runner. Siempre respeta el
// contexto: sin él, un subproceso colgado congelaría la bandeja.
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBinaryMissing, name)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return nil, fmt.Errorf("%s: %w: %s", name, err, msg)
	}
	return stdout.Bytes(), nil
}
