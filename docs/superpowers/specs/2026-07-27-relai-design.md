# Relai — diseño

**Fecha:** 2026-07-27
**Estado:** pendiente de revisión del usuario

## Problema

Álvaro trabaja contra varios planes de IA con límites por ventana (Claude 5h/7d, Codex 5h/semanal). Hoy no tiene forma pasiva de saber cuánto le queda en cada uno, y las tres piezas que resuelven el problema viven en sitios distintos:

- `cswap` (claude-swap): cuota por cuenta de Claude y cambio de cuenta.
- `continues`: traspaso de una sesión de Claude Code a Codex u otro harness.
- CodexBar: cuota multi-proveedor, pero sin traspaso ni gestión de cuentas.

Relai unifica las tres en un único icono de bandeja, multiplataforma.

## Decisión central

**Relai no reimplementa nada.** Invoca `cswap` y `continues` como subprocesos y parsea su salida JSON. El trabajo duro sigue viviendo upstream; Relai es la capa de presentación y disparo.

Consecuencia aceptada conscientemente: Relai duplica la *interfaz* de `cswap menubar` y de CodexBar, y hereda su riesgo de romperse si esos formatos cambian. Se asume a cambio de tener un solo icono que cubre cuota + cambio de cuenta + traspaso, cosa que ninguna herramienta existente hace.

## Fuera de alcance

- Reimplementar la lectura de cuotas de Claude que ya hace `cswap`.
- Reimplementar el parseo de sesiones que ya hace `continues`.
- Gestionar credenciales. Relai nunca lee ni escribe tokens: delega en `cswap`.
- Tests de UI de bandeja (no es testeable headless; fingirlo sería mentir).

## Arquitectura

```
relai/
  main.go                    arranque, systray.Run
  internal/
    providers/
      provider.go            interface Provider { Name() string; Usage(ctx) ([]Account, error) }
      claude.go              cswap list --json
      codex.go               account/rateLimits/read  (fallback: GET /backend-api/codex/usage)
    actions/
      switch.go              cswap switch <number> --json
      handoff.go             continues resume <id> --in <tool>
    sessions/
      sessions.go            continues list --jsonl
    tray/
      menu.go                render del menú, iconos, notificaciones
    config/
      config.go              ~/.config/relai/config.yml
```

Cada unidad tiene un propósito único y se puede entender y testear por separado. `providers` no sabe nada de la bandeja; `tray` no sabe nada de subprocesos.

### Interfaz Provider

Corregida el 2026-07-27 tras capturar la salida real de ambos proveedores. La versión anterior guardaba un único `Pct`; **ambos proveedores exponen varias ventanas por cuenta**, así que colapsarlas en el modelo perdía información irrecuperable. Ahora se guardan todas y "la peor" pasa a ser función derivada.

```go
// Una ventana de cuota. Pct es SIEMPRE consumido (0-100), nunca restante:
// todo el display (título, colores, umbral) asume "más alto = peor".
// Verificado: cswap emite `pct`, Codex emite `usedPercent`. Misma semántica.
type Window struct {
    Kind     string    // "5h" | "7d" | "spend" | "scoped"
    Name     string    // solo para "scoped": el modelo. Vacío en el resto.
    Pct      int
    ResetsAt time.Time // cero si el proveedor no la da
}

type Account struct {
    ID        string    // cswap: `number`, que es la clave del switch. Codex: cuenta única.
    Label     string    // `alias` si existe, si no el email. OJO: `alias` es opcional en cswap.
    Email     string
    Org       string
    Windows   []Window
    Active    bool
    Disabled  bool      // cswap: cuenta retirada de la rotación
    Status    Status    // ver abajo: no todo fallo es "0%"
    FetchedAt time.Time // de la fuente (`usageFetchedAt`), no del reloj local
    AgeS      float64   // de la fuente (`usageAgeSeconds`)
}

// Worst devuelve la ventana más consumida. Es función, no campo: derivarla al
// pintar evita que el modelo mienta cuando cambia el conjunto de ventanas.
func (a Account) Worst() (Window, bool)

type Provider interface {
    Name() string
    Usage(ctx context.Context) ([]Account, error)
}
```

### Status: los fallos no son ceros

`cswap` distingue varios motivos por los que no hay medición. Pintarlos todos como 0% mentiría al usuario justo cuando más importa acertar.

```go
type Status int
const (
    StatusOK Status = iota
    StatusTokenExpired        // el token caducó mientras lo tenía Claude Code
    StatusAPIKey              // cuenta de API key: no tiene cuota de suscripción
    StatusKeychainUnavailable // Keychain ilegible
    StatusNoCredentials
    StatusFetchFailed         // usage == null
)
```

Solo `StatusOK` muestra porcentaje. El resto muestra su propio texto en el menú.

Añadir un proveedor nuevo = un fichero en `providers/` que implemente la interfaz. Nada más cambia.

### Proveedores

| Proveedor | Fuente | Nivel de verificación |
|---|---|---|
| Claude | `cswap list --json` | **Verificado contra la salida y el código reales** (ver abajo). `cswap 0.23.0` instalado el 2026-07-27. |

### Esquema de cuotas de Claude (verificado)

Envelope real de `cswap list --json` con cero cuentas dadas de alta:

```json
{ "schemaVersion": 1, "activeAccountNumber": null, "accounts": [] }
```

El campo `schemaVersion` es la mejor noticia del diseño: `cswap` versiona su contrato, así que un cambio incompatible es detectable en vez de silencioso. **`claude.go` debe rechazar y marcar `StatusFetchFailed` si `schemaVersion != 1`**, en lugar de parsear a ciegas.

Forma de cada fila, leída de `claude_swap/json_output.py` (`account_row`, `usage_to_json`):

```jsonc
{
  "number": 1,                      // clave del switch: `cswap switch <number>`
  "email": "...",
  "organizationName": "...",
  "organizationUuid": "...",
  "isOrganization": true,
  "active": true,
  "usageStatus": "ok",              // o los centinelas de fallo → mapean a Status
  "usage": {                        // null si la medición falló
    "fiveHour": { "pct": 78, "resetsAt": ..., "countdown": "2h14", "clock": "..." },
    "sevenDay": { "pct": 41, "resetsAt": ..., "expectedPct": 38.2, "aheadOfPace": true,
                  "projectedExhaustionAt": "...Z", "willLastToReset": true },
    "spend":    { "used": ..., "limit": ..., "pct": ..., "currency": "USD" },
    "scoped":   [ { "name": "opus", "pct": 62, ... } ]   // ventanas semanales por modelo
  },
  "alias": "work",                  // OPCIONAL: ausente si no se ha puesto
  "disabled": true,                 // OPCIONAL: solo si está fuera de rotación
  "usageFetchedAt": "2026-07-27T...Z",
  "usageAgeSeconds": 42.0
}
```

Tres consecuencias directas sobre el diseño, ya aplicadas arriba:

1. **Varias ventanas por cuenta** (`fiveHour`, `sevenDay`, `spend`, y N `scoped` por modelo) → `Account.Windows []Window` en vez de un `Pct` único.
2. **La frescura la da la fuente** (`usageFetchedAt`, `usageAgeSeconds`) → Relai no inventa su propio flag `Stale`; propaga el de `cswap`.
3. **El switch va por `number`**, no por alias, y `alias` puede no existir → `Account.ID` guarda el número; `Label` cae a email cuando no hay alias.
| Codex | App-server: `Account/rateLimits/readRequest` | **Verificado contra el esquema real** (ver abajo). `codex-cli 0.145.0` instalado y con sesión ChatGPT activa el 2026-07-27. |

### Esquema de rate limits de Codex (verificado)

Extraído de `codex app-server generate-json-schema --out <dir>` con la 0.145.0. El protocolo expone:

- `Account/rateLimits/readRequest` — snapshot bajo demanda.
- `Account/rateLimits/updatedNotification` — **push** con actualizaciones rodantes. Permite suscribirse en lugar de hacer polling.

`RateLimitSnapshot` contiene `primary` y `secondary`, ambos de tipo `RateLimitWindow`, más `planType`, `limitName`, `credits` y `rateLimitReachedType`.

```jsonc
// RateLimitWindow
{
  "usedPercent":        int32,        // REQUERIDO. Porcentaje CONSUMIDO.
  "resetsAt":           int64 | null, // Unix epoch en segundos.
  "windowDurationMins": int64 | null  // 300 = ventana de 5h; 10080 = 7d.
}
```

Mapeo a `Window`: `Pct` ← `usedPercent` (misma semántica de "consumido" que `cswap`), `ResetsAt` ← `resetsAt`, `Kind` ← derivado de `windowDurationMins` (300 → `"5h"`, 10080 → `"7d"`). `primary` y `secondary` producen dos entradas en `Account.Windows`, igual que `fiveHour`/`sevenDay` en Claude.

**Implicación de diseño:** como Codex ofrece notificación push, `codex.go` puede mantener una conexión al app-server y actualizar al vuelo, dejando el ticker de 3 min como respaldo. Se decidirá en el plan de implementación; el diseño del ticker sigue siendo válido para ambos casos.

### Esquema de sesiones (verificado)

`continues list --jsonl` devuelve una línea JSON por sesión con estos campos, comprobado contra la máquina el 2026-07-27:

```
id, source, cwd, repo, branch, lines, bytes, createdAt, updatedAt, originalPath, summary
```

`sessions.go` consume `id`, `source`, `repo`, `updatedAt` y `summary` para poblar el submenú de handoff.

## Flujo de datos

1. Ticker cada `refresh_interval` (default 3 min) dispara todos los `Provider.Usage()` en paralelo.
2. Cada subproceso con `context.WithTimeout` de 10 s.
3. Resultados → estado en memoria → redibujo del menú.
4. Título de la bandeja = `Worst()` de todas las cuentas de todos los proveedores, es decir el porcentaje consumido más alto. Las cuentas con `Status != StatusOK` no compiten por el título.
5. Estado persistido en `~/.cache/relai/state.json` → arranque instantáneo y modo offline.

## Acciones del menú

- Clic en una cuenta de Claude → `cswap switch <number> --json`. Se usa `number` (el `Account.ID`), no el alias: el alias es opcional en `cswap` y no siempre existe.
- Las entradas de Codex son **solo lectura**: muestran cuota, no permiten cambiar de cuenta. No existe equivalente a `cswap` para Codex, y Relai no va a inventarlo.
- Submenú **Handoff →**: las 5 sesiones más recientes × destinos (`codex`, `opencode`) → `continues resume <id> --in <tool>`.
- Ajustes: intervalo de refresco, umbral de aviso, orden de proveedores.

### Aviso por umbral

Cuando un proveedor cruza `alert_threshold` (default 85%) al alza, Relai emite **una** notificación nativa del sistema y no vuelve a avisar de ese proveedor hasta que su porcentaje baje del umbral (típicamente al resetear la ventana). Esto evita el goteo de avisos repetidos en cada ciclo de refresco. El estado de "ya avisado" por proveedor vive en memoria, no se persiste: tras un reinicio, el primer cruce vuelve a avisar.

## Configuración

`~/.config/relai/config.yml`. Todo tiene default; el fichero es opcional y se crea al primer guardado desde Ajustes.

```yaml
refresh_interval: 3m
alert_threshold: 85        # porcentaje
providers: [claude, codex] # también define el orden en el menú
handoff:
  recent_sessions: 5
  targets: [codex, opencode]
binaries:                  # override si no están en PATH
  cswap: ""
  continues: ""
```

## Manejo de errores

| Situación | Comportamiento |
|---|---|
| `cswap` no está en PATH | Item de menú "claude-swap no encontrado · instalar" con enlace. No se cae. |
| Subproceso devuelve error | Se conserva el último valor conocido y se muestra su antigüedad usando `usageAgeSeconds` de la propia fuente (`· 5m ago`). Relai no calcula frescura por su cuenta. |
| Subproceso excede 10 s | Se cancela vía contexto. La bandeja nunca se congela. |
| `schemaVersion != 1` | **No se parsea.** La cuenta pasa a `StatusFetchFailed` con el texto "formato de cswap no soportado". Fallar visiblemente es mejor que mostrar números inventados sobre un contrato cambiado. |
| Campos opcionales ausentes (`alias`, `disabled`, `usage: null`) | Contemplados por diseño, no son error: `Label` cae a email, `Disabled` a false, `usage: null` → `StatusFetchFailed`. |
| `usageStatus` es un centinela de fallo | Se mapea a su `Status` concreto y se pinta su texto propio. Nunca se muestra 0%. |
| Destino de handoff no instalado (p. ej. Codex ausente) | El destino aparece en el submenú **deshabilitado**, con la razón como tooltip. Se comprueba la existencia del binario al construir el menú, no al hacer clic. |
| `continues resume` falla | Notificación con el stderr recortado. La sesión de origen no se toca: `continues` es de solo lectura sobre los ficheros de Claude Code. |

## Testing

- `providers` recibe un ejecutor de comandos inyectable (`func(ctx, name string, args ...string) ([]byte, error)`), de modo que los tests no lanzan procesos reales.
- Fixtures capturados de la salida real de `cswap list --json` y `continues list --jsonl`.
- Tests de tabla para: JSON válido, JSON con campos extra, JSON truncado, binario ausente, timeout.
- Sin tests de UI de bandeja, por lo dicho en Fuera de alcance.

## Multiplataforma

Compilación cruzada verificada desde macOS arm64 el 2026-07-27, sin toolchain adicional:

| Target | Resultado | Tamaño |
|---|---|---|
| darwin/arm64 | compila | 3,8 MB |
| windows/amd64 | compila | 4,1 MB |
| linux/amd64 (`CGO_ENABLED=0`) | compila | 6,1 MB |

Stack: Go 1.26.2 + `fyne.io/systray` v1.12.2.

## Riesgos

1. **Dependencia de formatos ajenos.** Riesgo real pero acotado, y desigual entre las dos dependencias: `cswap` versiona su contrato con `schemaVersion`, así que un cambio incompatible es *detectable* y Relai puede fallar visiblemente en vez de mentir. `continues` no versiona nada, así que ahí la mitigación es solo parseo tolerante + fixtures. En ningún caso se cae la bandeja.
2. **Linux necesita un host StatusNotifierItem** (appindicator o equivalente) en tiempo de ejecución. Compila en cualquier caso, pero en un escritorio sin ese soporte el icono no aparece. Documentar en el README.
3. **`continues` lleva sin actualizarse desde el 7-may-2026** y parsea un formato interno de Claude Code. Funciona con la 2.1.220 (verificado), pero puede romper en cualquier actualización. Relai debe tratar su fallo como no fatal.
4. ~~**La forma real del JSON de `cswap` está sin verificar.**~~ **RESUELTO el 2026-07-27.** Se instaló `cswap 0.23.0` y se leyó `claude_swap/json_output.py`. Resultado: `pct` sí es porcentaje consumido, así que la semántica de `Pct` era correcta — pero la struct `Account` era **incorrecta** en tres puntos (una sola ventana en vez de varias, frescura inventada en vez de propagada, y switch por alias en vez de por `number`). Los tres están corregidos arriba. Verificar antes de escribir código evitó exactamente el rehacer-la-interfaz que este riesgo anticipaba.

5. **Queda un supuesto menor sin verificar: la forma de una fila con cuentas reales.** El envelope se capturó con `accounts: []`, porque dar de alta una cuenta exige credenciales. La forma por fila viene del código fuente, que es fiable, pero no de una ejecución. **Acción para el plan de implementación:** capturar un `cswap list --json` real como fixture en cuanto haya al menos una cuenta añadida.

6. **Duplicación asumida.** `cswap menubar` y CodexBar cubren por separado dos de las tres funciones. Relai solo se justifica por la unificación; si esa unificación deja de aportar, la decisión correcta es abandonarlo y volver a las tres herramientas.

## Ubicación

Repositorio propio e independiente. Es una herramienta personal: no vive dentro de ningún monorepo de trabajo.
