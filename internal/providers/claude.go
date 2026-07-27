package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// claudeSchemaVersion es la única versión del contrato de cswap que sabemos
// leer. cswap versiona su salida, así que un cambio incompatible es
// detectable: preferimos fallar visiblemente a inventar números.
const claudeSchemaVersion = 1

type cswapWindow struct {
	Pct      int    `json:"pct"`
	ResetsAt *int64 `json:"resetsAt"`
	Name     string `json:"name"`
}

type cswapUsage struct {
	FiveHour *cswapWindow  `json:"fiveHour"`
	SevenDay *cswapWindow  `json:"sevenDay"`
	Spend    *cswapWindow  `json:"spend"`
	Scoped   []cswapWindow `json:"scoped"`
}

type cswapAccount struct {
	Number           int         `json:"number"`
	Email            string      `json:"email"`
	OrganizationName string      `json:"organizationName"`
	Active           bool        `json:"active"`
	UsageStatus      string      `json:"usageStatus"`
	Usage            *cswapUsage `json:"usage"`
	Alias            string      `json:"alias"`
	Disabled         bool        `json:"disabled"`
	UsageFetchedAt   string      `json:"usageFetchedAt"`
	UsageAgeSeconds  float64     `json:"usageAgeSeconds"`
}

type cswapEnvelope struct {
	SchemaVersion int            `json:"schemaVersion"`
	Accounts      []cswapAccount `json:"accounts"`
}

// ClaudeProvider lee cuotas de Claude invocando `cswap list --json`.
type ClaudeProvider struct {
	Run Runner
	Bin string
}

func NewClaudeProvider(run Runner) ClaudeProvider {
	return ClaudeProvider{Run: run, Bin: "cswap"}
}

func (p ClaudeProvider) Name() string { return "claude" }

func (p ClaudeProvider) Usage(ctx context.Context) ([]Account, error) {
	out, err := p.Run(ctx, p.Bin, "list", "--json")
	if err != nil {
		return nil, err
	}
	var env cswapEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("cswap: JSON ilegible: %w", err)
	}
	if env.SchemaVersion != claudeSchemaVersion {
		return nil, fmt.Errorf(
			"cswap: schemaVersion %d no soportada (esperada %d)",
			env.SchemaVersion, claudeSchemaVersion)
	}
	accounts := make([]Account, 0, len(env.Accounts))
	for _, raw := range env.Accounts {
		accounts = append(accounts, toAccount(raw))
	}
	return accounts, nil
}

func toAccount(raw cswapAccount) Account {
	label := raw.Alias
	if label == "" {
		label = raw.Email
	}
	a := Account{
		ID:       strconv.Itoa(raw.Number),
		Label:    label,
		Email:    raw.Email,
		Org:      raw.OrganizationName,
		Active:   raw.Active,
		Disabled: raw.Disabled,
		Status:   claudeStatus(raw.UsageStatus, raw.Usage),
		AgeS:     raw.UsageAgeSeconds,
	}
	if t, err := time.Parse(time.RFC3339, raw.UsageFetchedAt); err == nil {
		a.FetchedAt = t
	}
	if raw.Usage != nil {
		a.Windows = toWindows(raw.Usage)
	}
	return a
}

func toWindows(u *cswapUsage) []Window {
	var ws []Window
	add := func(kind string, cw *cswapWindow) {
		if cw == nil {
			return
		}
		w := Window{Kind: kind, Pct: cw.Pct, Name: cw.Name}
		if cw.ResetsAt != nil {
			w.ResetsAt = time.Unix(*cw.ResetsAt, 0)
		}
		ws = append(ws, w)
	}
	add("5h", u.FiveHour)
	add("7d", u.SevenDay)
	add("spend", u.Spend)
	for i := range u.Scoped {
		add("scoped", &u.Scoped[i])
	}
	return ws
}

// claudeStatus traduce los centinelas de cswap. `usage: null` es fallo
// aunque el centinela diga "ok": sin medición no hay porcentaje que enseñar.
func claudeStatus(sentinel string, usage *cswapUsage) Status {
	switch sentinel {
	case "token_expired":
		return StatusTokenExpired
	case "api_key":
		return StatusAPIKey
	case "keychain_unavailable":
		return StatusKeychainUnavailable
	case "no_credentials":
		return StatusNoCredentials
	}
	if usage == nil {
		return StatusFetchFailed
	}
	return StatusOK
}
