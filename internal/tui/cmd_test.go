package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// mustMsg ejecuta un cmd y devuelve su mensaje.
//
// Ejecuta en una goroutine con un techo de tiempo a propósito. Un cmd puede ser
// un tea.Tick, que duerme varios segundos, y un test que ejecuta comandos a
// ciegas se cuelga entero cuando un mutante cambia la rama que devuelve. Un
// test colgado es la peor señal posible en mutation testing: el mutant se
// reporta como TIMED OUT en vez de como muerto, y el gate lo cuenta como
// "medición incompleta" en lugar de como un fallo real.
func mustMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd es nil")
	}

	type result struct{ msg tea.Msg }
	ch := make(chan result, 1)
	go func() { ch <- result{cmd()} }()

	select {
	case r := <-ch:
		return r.msg
	case <-time.After(250 * time.Millisecond):
		t.Fatal("el comando no devolvió un mensaje a tiempo: parece un tea.Tick (duerme), no un cmd de E/S")
		return nil
	}
}
