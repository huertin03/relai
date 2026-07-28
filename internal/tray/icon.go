package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// iconSize es el lado del icono en píxeles. 22 es la altura útil de la barra
// de menú de macOS; en Windows y Linux el sistema lo reescala.
const iconSize = 22

// gaugeIcon dibuja un depósito que se llena de abajo arriba según pct.
//
// Es un icono de PLANTILLA: solo negro y alfa. macOS lo recolorea según el
// tema de la barra de menú, así que se ve bien en claro y en oscuro sin
// mantener dos versiones. Por eso no lleva ningún color propio.
//
// Cuando no hay medición (ok=false) se dibuja solo el contorno: un depósito
// vacío significaría "cuota libre", que es justo lo contrario de "no sé".
func gaugeIcon(pct int, ok bool) []byte {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	const (
		left   = 5
		right  = 16
		top    = 3
		bottom = 19
	)
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	black := color.NRGBA{0, 0, 0, 255}
	faint := color.NRGBA{0, 0, 0, 90}

	// Contorno del depósito.
	for x := left; x <= right; x++ {
		img.Set(x, top, black)
		img.Set(x, bottom, black)
	}
	for y := top; y <= bottom; y++ {
		img.Set(left, y, black)
		img.Set(right, y, black)
	}

	if !ok {
		// Sin medición: una diagonal tenue que se lee como "no hay dato".
		for i := 0; i <= right-left; i++ {
			img.Set(left+i, top+i*(bottom-top)/(right-left), faint)
		}
		return encodePNG(img)
	}

	// Relleno proporcional, de abajo arriba, sin tocar el contorno.
	innerTop, innerBottom := top+2, bottom-2
	height := innerBottom - innerTop + 1
	filled := height * pct / 100
	for y := innerBottom; y > innerBottom-filled; y-- {
		for x := left + 2; x <= right-2; x++ {
			img.Set(x, y, black)
		}
	}
	return encodePNG(img)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	// png.Encode sobre un bytes.Buffer no puede fallar salvo por OOM, en cuyo
	// caso el programa ya está perdido; devolver el buffer vacío deja a
	// systray sin icono en vez de tumbar la bandeja.
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
