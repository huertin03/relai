package tray

import (
	"log"

	"github.com/gen2brain/beeep"
)

// notify emite una notificación nativa. Un fallo aquí nunca debe tumbar la
// bandeja: en Linux sin demonio de notificaciones esto falla y es aceptable.
func notify(title, body string) {
	if err := beeep.Notify(title, body, ""); err != nil {
		log.Printf("relai: no se pudo notificar (%v): %s — %s", err, title, body)
	}
}
