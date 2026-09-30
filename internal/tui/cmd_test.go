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

// mustRun ejecuta un cmd y devuelve los mensajes que produce, aplanando los
// tea.Batch. Los comandos que sólo duermen (un tea.Tick de un toast) se
// saltan:.sleep es su trabajo, no un fallo, y el techo de tiempo los elimina
// del camino en lugar de tumbar el test.
//
// tea.Batch devuelve un único BatchMsg: no ejecuta lo que envuelve, sólo lo
// agrupa para que el runtime de bubbletea lo reparta. Un test que usa mustMsg
// sobre un batch obtiene el BatchMsg y el trabajo nunca se hace, así que las
// assertions sobre la base de datos fallan sin que haya ningún bug detrás.
func mustRun(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}

	msg := tryMsg(cmd)
	if msg == nil {
		return nil // dormía: un tick de aviso, no trabajo
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, inner := range batch {
			out = append(out, mustRun(t, inner)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// tryMsg ejecuta un cmd con el mismo techo que mustMsg pero devuelve nil en
// vez de fallar cuando el comando no termina a tiempo.
func tryMsg(cmd tea.Cmd) tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()

	select {
	case msg := <-ch:
		return msg
	case <-time.After(250 * time.Millisecond):
		return nil
	}
}
