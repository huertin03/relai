// Package sessions lista las sesiones de CLIs de código invocando
// `continues list --jsonl`. Solo lectura: nunca modifica ficheros de sesión.
package sessions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/huertin03/relai/internal/providers"
)

// Session es una sesión candidata a traspaso.
type Session struct {
	ID        string
	Source    string
	Repo      string
	Summary   string
	UpdatedAt time.Time
}

type rawSession struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Repo      string `json:"repo"`
	Summary   string `json:"summary"`
	UpdatedAt string `json:"updatedAt"`
}

// Lister invoca continues. Bin es sobreescribible desde configuración.
type Lister struct {
	Run providers.Runner
	Bin string
}

func NewLister(run providers.Runner) Lister {
	return Lister{Run: run, Bin: "continues"}
}

// Recent devuelve como mucho n sesiones. Las líneas ilegibles se descartan:
// continues no versiona su formato, así que una línea rota no debe tumbar
// el submenú entero.
func (l Lister) Recent(ctx context.Context, n int) ([]Session, error) {
	out, err := l.Run(ctx, l.Bin, "list", "--source", "claude", "--jsonl")
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sessions := make([]Session, 0, n)
	for sc.Scan() && len(sessions) < n {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw rawSession
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		if raw.ID == "" {
			continue
		}
		s := Session{ID: raw.ID, Source: raw.Source, Repo: raw.Repo, Summary: raw.Summary}
		if t, err := time.Parse(time.RFC3339, raw.UpdatedAt); err == nil {
			s.UpdatedAt = t
		}
		sessions = append(sessions, s)
	}
	return sessions, sc.Err()
}
