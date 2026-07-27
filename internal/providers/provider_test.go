package providers

import (
	"testing"
	"time"
)

func TestWorstDevuelveLaVentanaMasConsumida(t *testing.T) {
	a := Account{Windows: []Window{
		{Kind: "5h", Pct: 30},
		{Kind: "7d", Pct: 71},
		{Kind: "30d", Pct: 12},
	}}
	w, ok := a.Worst()
	if !ok {
		t.Fatal("Worst() devolvió ok=false con ventanas presentes")
	}
	if w.Pct != 71 || w.Kind != "7d" {
		t.Fatalf("esperaba 7d/71, obtuve %s/%d", w.Kind, w.Pct)
	}
}

func TestWorstSinVentanas(t *testing.T) {
	if _, ok := (Account{}).Worst(); ok {
		t.Fatal("esperaba ok=false sin ventanas")
	}
}

func TestStatusStringNuncaEsVacio(t *testing.T) {
	for s := StatusOK; s <= StatusFetchFailed; s++ {
		if s.String() == "" {
			t.Fatalf("Status(%d).String() está vacío", int(s))
		}
	}
}

func TestSoloStatusOKMuestraPorcentaje(t *testing.T) {
	if !(Account{Status: StatusOK}).ShowsPct() {
		t.Fatal("StatusOK debe mostrar porcentaje")
	}
	for _, s := range []Status{StatusTokenExpired, StatusAPIKey, StatusKeychainUnavailable, StatusNoCredentials, StatusFetchFailed} {
		if (Account{Status: s}).ShowsPct() {
			t.Fatalf("Status %v no debe mostrar porcentaje", s)
		}
	}
}

func TestWindowConservaResetsAt(t *testing.T) {
	ts := time.Unix(1787749472, 0)
	w := Window{Kind: "30d", Pct: 0, ResetsAt: ts}
	if !w.ResetsAt.Equal(ts) {
		t.Fatal("ResetsAt no se conservó")
	}
}
