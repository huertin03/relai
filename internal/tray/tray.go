// Package tray pinta el icono y su menú. No lanza subprocesos directamente:
// todo pasa por providers, sessions y actions.
package tray

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/huertin03/relai/internal/actions"
	"github.com/huertin03/relai/internal/config"
	"github.com/huertin03/relai/internal/providers"
	"github.com/huertin03/relai/internal/sessions"
	"github.com/huertin03/relai/internal/state"
)

const subprocessTimeout = 10 * time.Second

// acctSlot es un hueco reservado para una cuenta dentro del menú de un
// proveedor. id/switchable describen qué cuenta representa el slot AHORA
// MISMO: los escribe el goroutine de refresco en cada repintado y los lee
// el goroutine de clic de ese mismo slot al accionarse, así que van detrás
// de su propio mutex en vez de compartir uno global con el resto del menú.
type acctSlot struct {
	item *systray.MenuItem

	// visible solo lo toca repaint (serializado por app.repaintMu, nunca dos
	// invocaciones a la vez), así que no necesita mutex propio. Evita llamar
	// a Show()/Hide() cuando la visibilidad no cambió: en Windows un Hide()
	// repetido dispara RemoveMenu sobre un ítem ya ausente (log de error
	// benigno pero ruidoso) y en Linux reemite una señal dbus sin cambios
	// reales en cada ciclo de refresco.
	visible bool

	mu         sync.Mutex
	id         string
	switchable bool
}

func (s *acctSlot) setAccount(id string, switchable bool) {
	s.mu.Lock()
	s.id, s.switchable = id, switchable
	s.mu.Unlock()
}

func (s *acctSlot) account() (id string, switchable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id, s.switchable
}

// providerMenu son los ítems persistentes de un proveedor: se crean una
// sola vez en buildMenu y repaint solo les cambia texto y visibilidad.
// providerMenu son los ítems de un proveedor. El proveedor es un submenú:
// sus cuentas cuelgan de él, así que crearlas tarde no las manda al final del
// menú principal. Los hijos se crean PEREZOSAMENTE y ya con su texto puesto:
// en el backend de macOS un ítem creado con Hide() no reaparece nunca con
// Show(), así que la regla es no crear nada oculto.
type providerMenu struct {
	name   string
	parent *systray.MenuItem

	errItem    *systray.MenuItem
	errVisible bool

	// emptyItem explica que el proveedor no tiene cuentas. Sin él, un
	// submenú vacío no distingue "no hay nada configurado" de "está roto",
	// que es justo la confusión que hace inútil la bandeja.
	emptyItem    *systray.MenuItem
	emptyVisible bool

	slots []*acctSlot
}

type app struct {
	cfg       config.Config
	st        *state.State
	provs     []providers.Provider
	lister    sessions.Lister
	actor     actions.Actor
	cachePath string
	quitCh    chan struct{}

	// repaintMu serializa repaint() contra sí mismo. MenuItem.SetTitle (y
	// SetTooltip/Enable/Disable) escriben sus campos sin ningún lock propio
	// en fyne.io/systray v1.12.2, así que dos repintados concurrentes sobre
	// el mismo ítem serían una carrera real de Go, no solo un problema de
	// orden visual.
	repaintMu sync.Mutex
	menus     []*providerMenu
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

	// cfg.Providers no es solo qué se pinta: también es qué se invoca. Sin
	// este filtro, desactivar un proveedor en la config lo quitaba de la
	// vista pero Relai seguía lanzando su subproceso en cada refresco y
	// pudiendo notificar sus cruces de umbral.
	known := map[string]providers.Provider{
		claude.Name(): claude,
		codex.Name():  codex,
	}
	var provs []providers.Provider
	for _, name := range cfg.Providers {
		if p, ok := known[name]; ok {
			provs = append(provs, p)
		}
	}

	a := &app{
		cfg:    cfg,
		st:     state.New(cfg.AlertThreshold),
		provs:  provs,
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
	systray.SetTemplateIcon(gaugeIcon(0, false), gaugeIcon(0, false))
	systray.SetTitle("—")
	systray.SetTooltip("Relai")
	a.buildMenu()
	// Pinta lo que haya en el cache (o el vacío, en un arranque limpio)
	// antes de que el primer refresco tenga tiempo de completarse.
	a.repaint()

	// El usuario abre la bandeja precisamente cuando más le importa ver algo
	// al día; repintar en ese instante cubre el hueco entre refrescos
	// periódicos sin tener que acortar el intervalo del ticker. (En Windows
	// el backend de systray no emite en este canal: ahí este goroutine
	// simplemente nunca se despierta, sin efecto adverso.)
	go func() {
		for range systray.TrayOpenedCh {
			a.repaint()
		}
	}()

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
			a.repaint()
			for _, msg := range a.st.PendingAlerts() {
				notify("Relai", msg)
			}
			if a.cachePath != "" {
				_ = a.st.Save(a.cachePath)
			}
		}(p)
	}
}

// repaint relee el snapshot y actualiza el título del icono y los ítems que
// buildMenu ya creó. No crea ni destruye ningún MenuItem: systray no permite
// insertar ítems a mitad de menú de forma portable (AddMenuItem siempre
// añade), así que la única forma segura de refrescar es escribir sobre lo
// que ya existe.
func (a *app) repaint() {
	a.repaintMu.Lock()
	defer a.repaintMu.Unlock()

	snap := a.st.Snapshot()
	systray.SetTitle(a.st.Title())
	// El icono repite el peor porcentaje en forma de depósito: de un vistazo,
	// sin leer el número. Sin medición se dibuja vacío con una diagonal.
	pct, ok := a.st.WorstPct()
	icon := gaugeIcon(pct, ok)
	systray.SetTemplateIcon(icon, icon)
	for _, pm := range a.menus {
		a.paintProvider(pm, snap.ByProvider[pm.name], snap.Errors[pm.name])
	}
}

// paintProvider actualiza los ítems de un proveedor, creando los que falten.
// Se invoca siempre desde repaint(), con repaintMu ya cogido, así que la
// creación perezosa está serializada.
//
// Por qué perezosa y no reservando slots por adelantado: en el backend de
// macOS de systray un MenuItem creado y ocultado con Hide() no vuelve a
// aparecer al llamar a Show(). Reservar ocho slots ocultos dejaba el menú
// permanentemente vacío aunque el estado tuviera cuentas — verificado con
// trazas: repaint() recibía la cuenta y llamaba a Show() sin efecto alguno.
// La regla, por tanto, es que ningún ítem nace oculto.
func (a *app) paintProvider(pm *providerMenu, accs []providers.Account, provErr error) {
	if provErr != nil {
		// ErrBinaryMissing distingue "no está instalado" de "falló": la
		// bandeja debe ofrecer un mensaje accionable en el primer caso, no
		// solo repetir el error crudo del segundo.
		title := "sin datos: " + provErr.Error()
		if errors.Is(provErr, providers.ErrBinaryMissing) {
			title = pm.name + ": la herramienta no está instalada"
		}
		if pm.errItem == nil {
			pm.errItem = pm.parent.AddSubMenuItem(title, provErr.Error())
			pm.errItem.Disable()
			pm.errVisible = true
		} else {
			pm.errItem.SetTitle(title)
			pm.errItem.SetTooltip(provErr.Error())
			if !pm.errVisible {
				pm.errVisible = true
				pm.errItem.Show()
			}
		}
	} else if pm.errItem != nil && pm.errVisible {
		pm.errVisible = false
		pm.errItem.Hide()
	}

	// Sin cuentas y sin error: decirlo explícitamente. Un submenú vacío no
	// distingue "no hay nada configurado" de "está roto".
	if len(accs) == 0 && provErr == nil {
		msg := pm.name + ": sin cuentas registradas"
		if pm.name == "claude" {
			msg = "sin cuentas · regístralas con: cswap add"
		}
		if pm.emptyItem == nil {
			pm.emptyItem = pm.parent.AddSubMenuItem(msg, "Relai no tiene nada que mostrar para este proveedor")
			pm.emptyItem.Disable()
			pm.emptyVisible = true
		} else if !pm.emptyVisible {
			pm.emptyVisible = true
			pm.emptyItem.Show()
		}
	} else if pm.emptyItem != nil && pm.emptyVisible {
		pm.emptyVisible = false
		pm.emptyItem.Hide()
	}

	for i, acc := range accs {
		// Codex es solo lectura: no hay equivalente a cswap para cambiar de
		// cuenta, así que sus ítems nunca se ofrecen como accionables.
		clickable := acc.Status == providers.StatusOK && pm.name != "codex"

		if i == len(pm.slots) {
			item := pm.parent.AddSubMenuItem(etiqueta(acc), acc.Email)
			slot := &acctSlot{item: item, visible: true}
			slot.setAccount(acc.ID, clickable)
			pm.slots = append(pm.slots, slot)
			go a.watchAccountClick(slot)
			if !clickable {
				item.Disable()
			}
			continue
		}

		slot := pm.slots[i]
		slot.setAccount(acc.ID, clickable)
		slot.item.SetTitle(etiqueta(acc))
		slot.item.SetTooltip(acc.Email)
		if clickable {
			slot.item.Enable()
		} else {
			slot.item.Disable()
		}
		if !slot.visible {
			slot.visible = true
			slot.item.Show()
		}
	}

	// Sobrantes de un refresco anterior: ocultarlos es seguro porque ya
	// estuvieron visibles al menos una vez.
	for i := len(accs); i < len(pm.slots); i++ {
		slot := pm.slots[i]
		slot.setAccount("", false)
		if slot.visible {
			slot.visible = false
			slot.item.Hide()
		}
	}
}

// buildMenu crea todos los ítems del menú UNA sola vez. Cada proveedor
// reserva un número fijo de ítems de cuenta (maxAccountSlots) más un ítem de
// error, todos ocultos hasta que repaint los llene: AddMenuItem/
// AddSubMenuItem solo añaden, nunca reemplazan, así que reconstruir el menú
// en cada refresco duplicaría ítems y fugaría un goroutine de ClickedCh por
// copia. En su lugar, cada slot vive para siempre y repaint solo cambia su
// texto y visibilidad.
func (a *app) buildMenu() {
	// El orden lo fija la configuración, no un literal: así el menú no baila
	// y el usuario decide qué proveedor ve primero.
	for _, name := range a.cfg.Providers {
		pm := &providerMenu{name: name}
		pm.parent = systray.AddMenuItem(name, "")
		a.menus = append(a.menus, pm)
	}
}

// watchAccountClick es el único goroutine de ClickedCh de este slot, creado
// una vez en buildMenu y vivo el resto del programa. Lee qué cuenta
// representa el slot EN EL MOMENTO del clic, no la que tenía al crearse.
func (a *app) watchAccountClick(slot *acctSlot) {
	for range slot.item.ClickedCh {
		id, switchable := slot.account()
		if !switchable || id == "" {
			continue // ítem oculto o no accionable: clic fantasma, se ignora
		}
		ctx, cancel := context.WithTimeout(context.Background(), subprocessTimeout)
		if err := a.actor.SwitchAccount(ctx, id); err != nil {
			notify("Relai", "no se pudo cambiar de cuenta: "+err.Error())
		}
		cancel()
		a.refresh()
	}
}

// buildHandoffMenu construye el submenú de traspaso de sesiones, con la
// lista vigente en el arranque. A diferencia de las cuentas, el traspaso no
// está en el alcance del refresco periódico.
func (a *app) buildHandoffMenu() {
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
