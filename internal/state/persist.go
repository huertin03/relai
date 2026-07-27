package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/huertin03/relai/internal/providers"
)

type discoState struct {
	Accounts map[string][]providers.Account `json:"accounts"`
}

// CachePath devuelve la ruta del cache en disco.
func CachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "relai", "state.json"), nil
}

// Save vuelca las cuentas conocidas. Escritura atómica: un corte a mitad no
// debe dejar un cache medio escrito que rompa el siguiente arranque.
func (s *State) Save(path string) error {
	s.mu.Lock()
	d := discoState{Accounts: s.accounts}
	b, err := json.Marshal(d)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load rellena el estado desde el cache. Un cache ausente o corrupto no es
// error: es el primer arranque, o un cache viejo que simplemente se descarta.
// Deliberadamente NO pasa por Update, para no disparar avisos: un 90%
// guardado ayer no es un cruce del umbral ocurrido ahora.
func (s *State) Load(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var d discoState
	if err := json.Unmarshal(b, &d); err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range d.Accounts {
		s.accounts[k] = v
	}
	return nil
}
