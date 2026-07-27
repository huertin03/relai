package tray

import (
	"sync"
	"testing"
)

// TestAcctSlotConcurrentAccess ejercita bajo -race el mapeo slot→cuenta que
// escribe repaint (goroutine de refresco) y lee watchAccountClick (goroutine
// de clic). Deliberadamente no crea ningún *systray.MenuItem real: eso solo
// existe tras systray.Run, que arranca un bucle de eventos nativo y no tiene
// sentido en un test headless. setAccount/account no tocan item, así que un
// acctSlot con item nil basta para probar la única parte de este diseño que
// de verdad necesita el mutex.
func TestAcctSlotConcurrentAccess(t *testing.T) {
	slot := &acctSlot{}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			slot.setAccount("acc-1", i%2 == 0)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			if id, _ := slot.account(); id != "" && id != "acc-1" {
				t.Errorf("id inesperado: %q", id)
			}
		}
	}()
	wg.Wait()
}
