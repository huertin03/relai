// Package tray pinta el icono y su menú. No lanza subprocesos directamente:
// todo pasa por providers, sessions y actions.
package tray

import (
	"context"
	"fmt"
	"time"

	"fyne.io/systray"

	"github.com/huertin03/relai/internal/actions"
	"github.com/huertin03/relai/internal/config"
	"github.com/huertin03/relai/internal/providers"
	"github.com/huertin03/relai/internal/sessions"
	"github.com/huertin03/relai/internal/state"
)

const subprocessTimeout = 10 * time.Second

type app struct {
	cfg       config.Config
	st        *state.State
	provs     []providers.Provider
	lister    sessions.Lister
	actor     actions.Actor
	cachePath string
	quitCh    chan struct{}
}

// Run arranca la bandeja. Bloquea hasta que el usuario sale.
func Run(cfg config.Config) error {
	run := providers.Runner(providers.ExecRunner)

	claude := providers.NewClaudeProvider(run)
	if cfg.Binaries.Cswap != "" {
		claude.Bin = cfg.Binaries.Cswap
	}
	codex := providers.NewCodexProvider()
	if cfg.Binaries.Codex != "" {
		codex.Bin = cfg.Binaries.Codex
	}
	lister := sessions.NewLister(run)
	if cfg.Binaries.Continues != "" {
		lister.Bin = cfg.Binaries.Continues
	}
	actor := actions.NewActor(run)
	if cfg.Binaries.Cswap != "" {
		actor.CswapBin = cfg.Binaries.Cswap
	}
	if cfg.Binaries.Continues != "" {
		actor.ContinuesBin = cfg.Binaries.Continues
	}

	a := &app{
		cfg:    cfg,
		st:     state.New(cfg.AlertThreshold),
		provs:  []providers.Provider{claude, codex},
		lister: lister,
		actor:  actor,
		quitCh: make(chan struct{}),
	}
	// Cache en disco: el menú muestra algo desde el primer instante en vez de
	// esperar al primer refresco. Un cache ausente o corrupto no es error.
	if p, err := state.CachePath(); err == nil {
		a.cachePath = p
		_ = a.st.Load(p)
	}
	systray.Run(a.onReady, func() {})
	return nil
}

func (a *app) onReady() {
	systray.SetTitle("—")
	systray.SetTooltip("Relai")
	a.rebuild()

	go func() {
		a.refresh()
		t := time.NewTicker(a.cfg.RefreshInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				a.refresh()
			case <-a.quitCh:
				return
			}
		}
	}()
}

// refresh consulta todos los proveedores en paralelo y repinta.
func (a *app) refresh() {
	for _, p := range a.provs {
		go func(p providers.Provider) {
			ctx, cancel := context.WithTimeout(context.Background(), subprocessTimeout)
			defer cancel()
			accs, err := p.Usage(ctx)
			a.st.Update(p.Name(), accs, err)
			systray.SetTitle(a.st.Title())
			for _, msg := range a.st.PendingAlerts() {
				notify("Relai", msg)
			}
			if a.cachePath != "" {
				_ = a.st.Save(a.cachePath)
			}
		}(p)
	}
}

// rebuild construye el menú una sola vez. systray no permite reconstruir
// items dinámicamente de forma portable, así que los items se crean aquí y
// su texto se actualiza en su sitio.
func (a *app) rebuild() {
	snap := a.st.Snapshot()
	// El orden lo fija la configuración, no un literal: así el menú no baila
	// y el usuario decide qué proveedor ve primero.
	for _, name := range a.cfg.Providers {
		accs := snap.ByProvider[name]
		header := systray.AddMenuItem(name, "")
		header.Disable()
		for _, acc := range accs {
			item := systray.AddMenuItem(etiqueta(acc), acc.Email)
			if acc.Status != providers.StatusOK || name == "codex" {
				// Codex es solo lectura: no hay equivalente a cswap.
				item.Disable()
				continue
			}
			id := acc.ID
			go func() {
				for range item.ClickedCh {
					ctx, cancel := context.WithTimeout(context.Background(), subprocessTimeout)
					if err := a.actor.SwitchAccount(ctx, id); err != nil {
						notify("Relai", "no se pudo cambiar de cuenta: "+err.Error())
					}
					cancel()
					a.refresh()
				}
			}()
		}
	}

	systray.AddSeparator()
	handoff := systray.AddMenuItem("Handoff", "Traspasar la sesión más reciente")
	ctx, cancel := context.WithTimeout(context.Background(), subprocessTimeout)
	ss, err := a.lister.Recent(ctx, a.cfg.Handoff.RecentSessions)
	cancel()
	if err != nil {
		sub := handoff.AddSubMenuItem("continues no disponible", err.Error())
		sub.Disable()
	}
	for _, s := range ss {
		// s.ID no tiene longitud mínima garantizada (solo se descarta si está
		// vacío): truncar a ciegas con [:8] podía reventar el arranque del
		// menú con un ID corto y sin resumen.
		label := s.Summary
		if label == "" {
			label = s.ID
			if len(label) > 8 {
				label = label[:8]
			}
		}
		if label == "" {
			label = "(sesión sin título)"
		}
		entry := handoff.AddSubMenuItem(label, s.Repo)
		for _, target := range a.cfg.Handoff.Targets {
			ti := entry.AddSubMenuItem("→ "+target, "")
			if !a.actor.TargetAvailable(target) {
				ti.Disable()
				continue
			}
			sid, tgt := s.ID, target
			go func() {
				for range ti.ClickedCh {
					c, cn := context.WithTimeout(context.Background(), subprocessTimeout)
					if err := a.actor.Handoff(c, sid, tgt); err != nil {
						notify("Relai", "handoff falló: "+err.Error())
					}
					cn()
				}
			}()
		}
	}

	systray.AddSeparator()
	quit := systray.AddMenuItem("Salir", "")
	go func() {
		<-quit.ClickedCh
		close(a.quitCh)
		systray.Quit()
	}()
}

func etiqueta(a providers.Account) string {
	if !a.ShowsPct() {
		return fmt.Sprintf("%s · %s", a.Label, a.Status)
	}
	w, ok := a.Worst()
	if !ok {
		return fmt.Sprintf("%s · sin ventanas", a.Label)
	}
	s := fmt.Sprintf("%s · %d%% (%s)", a.Label, w.Pct, w.Kind)
	if a.Plan != "" {
		s += " · " + a.Plan
	}
	return s
}
