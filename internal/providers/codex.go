package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Transport abre un canal con el app-server: stdout para leer, stdin para
// escribir, y un cierre. Es inyectable para que los tests no lancen codex.
type Transport func(ctx context.Context) (io.ReadCloser, io.WriteCloser, func() error, error)

type codexWindow struct {
	UsedPercent        int    `json:"usedPercent"`
	WindowDurationMins int64  `json:"windowDurationMins"`
	ResetsAt           *int64 `json:"resetsAt"`
}

type codexRateLimits struct {
	PlanType  string       `json:"planType"`
	LimitName string       `json:"limitName"`
	Primary   *codexWindow `json:"primary"`
	Secondary *codexWindow `json:"secondary"`
}

type codexResponse struct {
	ID     *int64 `json:"id"`
	Result struct {
		RateLimits *codexRateLimits `json:"rateLimits"`
	} `json:"result"`
}

// CodexProvider lee cuotas hablando el protocolo stdio del app-server.
type CodexProvider struct {
	Transport Transport
	Bin       string
}

func NewCodexProvider() CodexProvider {
	p := CodexProvider{Bin: "codex"}
	p.Transport = p.spawn
	return p
}

func (p CodexProvider) Name() string { return "codex" }

// spawn arranca `codex app-server` como subproceso.
func (p CodexProvider) spawn(ctx context.Context) (io.ReadCloser, io.WriteCloser, func() error, error) {
	bin := p.Bin
	if bin == "" {
		bin = "codex"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %s", ErrBinaryMissing, bin)
	}
	cmd := exec.CommandContext(ctx, bin, "app-server")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	cierre := func() error {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil
	}
	return stdout, stdin, cierre, nil
}

func (p CodexProvider) Usage(ctx context.Context) ([]Account, error) {
	stdout, stdin, cierre, err := p.Transport(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cierre() }()

	// El protocolo NO lleva campo `jsonrpc`. Verificado en vivo.
	peticiones := []string{
		`{"id":1,"method":"initialize","params":{"clientInfo":{"name":"relai","version":"0.1.0"}}}`,
		`{"method":"initialized","params":null}`,
		`{"id":2,"method":"account/rateLimits/read","params":null}`,
	}
	for _, req := range peticiones {
		if _, err := io.WriteString(stdin, req+"\n"); err != nil {
			return nil, fmt.Errorf("codex: no se pudo escribir la petición: %w", err)
		}
	}

	rl, err := leerRespuesta(stdout, 2)
	if err != nil {
		return nil, err
	}
	return []Account{toCodexAccount(rl)}, nil
}

// leerRespuesta lee líneas hasta encontrar la que casa con wantID. El
// app-server intercala notificaciones no solicitadas, así que asumir
// "la siguiente línea es mi respuesta" falla de forma intermitente.
func leerRespuesta(r io.Reader, wantID int64) (*codexRateLimits, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var resp codexResponse
		if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
			continue // línea que no entendemos: la ignoramos, no es fatal
		}
		if resp.ID == nil || *resp.ID != wantID {
			continue // notificación u otra respuesta
		}
		if resp.Result.RateLimits == nil {
			return nil, errors.New("codex: respuesta sin rateLimits")
		}
		return resp.Result.RateLimits, nil
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("codex: error leyendo del app-server: %w", err)
	}
	return nil, errors.New("codex: el app-server cerró sin responder")
}

func toCodexAccount(rl *codexRateLimits) Account {
	a := Account{
		ID:     "codex",
		Label:  "Codex",
		Plan:   rl.PlanType,
		Status: StatusOK,
		Active: true,
	}
	for _, cw := range []*codexWindow{rl.Primary, rl.Secondary} {
		if cw == nil {
			continue
		}
		w := Window{Kind: kindFromMinutes(cw.WindowDurationMins), Pct: cw.UsedPercent}
		if cw.ResetsAt != nil {
			w.ResetsAt = time.Unix(*cw.ResetsAt, 0)
		}
		a.Windows = append(a.Windows, w)
	}
	if len(a.Windows) == 0 {
		a.Status = StatusFetchFailed
	}
	return a
}

// kindFromMinutes traduce la duración a etiqueta. Deliberadamente general:
// la captura real trajo 43200 (30d), que un switch cerrado a 5h/7d habría
// convertido en basura.
func kindFromMinutes(mins int64) string {
	switch {
	case mins <= 0:
		return "?"
	case mins%(60*24) == 0:
		return fmt.Sprintf("%dd", mins/(60*24))
	case mins%60 == 0:
		return fmt.Sprintf("%dh", mins/60)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}
