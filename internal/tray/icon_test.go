package tray

import (
	"bytes"
	"image/png"
	"testing"
)

func TestGaugeIconEsPNGValido(t *testing.T) {
	for _, c := range []struct {
		pct int
		ok  bool
	}{{0, false}, {0, true}, {50, true}, {100, true}, {150, true}, {-5, true}} {
		b := gaugeIcon(c.pct, c.ok)
		if len(b) == 0 {
			t.Fatalf("pct=%d ok=%v: icono vacío", c.pct, c.ok)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("pct=%d ok=%v: PNG inválido: %v", c.pct, c.ok, err)
		}
		if img.Bounds().Dx() != iconSize || img.Bounds().Dy() != iconSize {
			t.Fatalf("pct=%d: tamaño inesperado %v", c.pct, img.Bounds())
		}
	}
}
