// Package config carga ~/.config/relai/config.yml. Todo tiene default y el
// fichero es opcional: Relai debe arrancar en una máquina limpia.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

type Handoff struct {
	RecentSessions int      `yaml:"recent_sessions"`
	Targets        []string `yaml:"targets"`
}

type Binaries struct {
	Cswap     string `yaml:"cswap"`
	Continues string `yaml:"continues"`
	Codex     string `yaml:"codex"`
}

type Config struct {
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	AlertThreshold  int           `yaml:"alert_threshold"`
	Providers       []string      `yaml:"providers"`
	Handoff         Handoff       `yaml:"handoff"`
	Binaries        Binaries      `yaml:"binaries"`
}

func Default() Config {
	return Config{
		RefreshInterval: 3 * time.Minute,
		AlertThreshold:  85,
		Providers:       []string{"claude", "codex"},
		Handoff: Handoff{
			RecentSessions: 5,
			Targets:        []string{"codex", "opencode"},
		},
	}
}

// DefaultPath devuelve la ruta canónica del fichero de configuración:
// $XDG_CONFIG_HOME/relai/config.yml, o ~/.config/relai/config.yml.
//
// Deliberadamente NO se usa os.UserConfigDir(): en macOS devuelve
// ~/Library/Application Support, mientras que toda la documentación de Relai
// promete ~/.config. Esa discrepancia dejó la configuración del usuario sin
// leer y silenciosamente sin efecto, que es el peor fallo posible en algo que
// solo existe para ser configurado. En Windows sí se delega en el sistema.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "relai", "config.yml"), nil
	}
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "relai", "config.yml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "relai", "config.yml"), nil
}

// Load parte de los defaults y sobreescribe solo lo presente en el fichero.
// Un fichero ausente no es error.
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Default(), fmt.Errorf("config: YAML inválido en %s: %w", path, err)
	}
	return sanitize(c), nil
}

// sanitize recorta valores imposibles en vez de rechazar el fichero entero:
// un umbral mal puesto no debe dejar al usuario sin bandeja.
func sanitize(c Config) Config {
	if c.AlertThreshold > 100 {
		c.AlertThreshold = 100
	}
	if c.AlertThreshold < 0 {
		c.AlertThreshold = 0
	}
	if c.RefreshInterval < 10*time.Second {
		c.RefreshInterval = 10 * time.Second
	}
	if c.Handoff.RecentSessions < 1 {
		c.Handoff.RecentSessions = 1
	}
	return c
}
