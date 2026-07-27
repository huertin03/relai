// Command relai muestra la cuota consumida de cada plan de IA en la bandeja
// del sistema, cambia de cuenta de Claude y traspasa sesiones a otros CLIs.
package main

import (
	"log"

	"github.com/huertin03/relai/internal/config"
	"github.com/huertin03/relai/internal/tray"
)

func main() {
	path, err := config.DefaultPath()
	if err != nil {
		log.Fatalf("relai: no se pudo resolver la ruta de configuración: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		log.Printf("relai: configuración ignorada (%v), se usan los defaults", err)
		cfg = config.Default()
	}
	if err := tray.Run(cfg); err != nil {
		log.Fatalf("relai: %v", err)
	}
}
