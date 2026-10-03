package tui

import (
	"strings"
	"testing"
)

// Los dos modales de texto --el de filtros y el de tags-- filtran lo que llega
// por una condición del mismoshape: "un solo carácter, imprimible".
//
// Lo que hay que comprobar no es que una letra se acepte, sino dónde está el
// borde. El rango del filtro es [33, 127) y el de tags es [32, 127): el filtro
// deja fuera el espacio, tags lo acepta. Los dos conmutan a la vez en 127.
//
// Un test con "a" no dice nada de eso: 'a' está en medio de los dos rangos y
// los dos lo aceptan. Sólo el borde los separa, y por eso se prueban los
// caracteres que están justo en el borde y los que están justo fuera.

// teclasDelBorde son los caracteres alrededor de los dos límites: el 32 (espacio,
// que sólo acepta tags), el 33 ('!', el primero del filtro) y el 126 (~, el
// último imprimible ASCII) y el 127 (DEL, fuera de los dos rangos).
var teclasDelBorde = []struct {
	nombre   string
	tecla    string
	acepta   bool
	caracter rune
}{
	{"espacio (32)", " ", false, 0},
	{"exclamación (33)", "!", true, '!'},
	{"virgulina (44)", ",", true, ','},
	{"tilde (126)", "~", true, '~'},
	{"DEL (127)", "\x7f", false, 0},
}

// El filtro se abre con "/" y escribe en el campo que tenga el foco, que al
// abrir es el primero.
func filtroAbierto(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.currentView = viewList
	abierto, _ := pulsar(t, m, "/")
	if !abierto.filterOpen {
		t.Fatal("el modal de filtros no se ha abierto con /")
	}
	return abierto
}

// updateTagPaste pega texto en el modal de tags por el camino del pegado, que
// es el único por el que un espacio puede llegar.
func updateTagPaste(t *testing.T, m *Model, texto string) *Model {
	t.Helper()
	m.tagOpen = true
	pegado, _ := m.handleTagPaste(texto)
	model, ok := pegado.(Model)
	if !ok {
		t.Fatalf("handleTagPaste(%q) ha devuelto %T, want Model", texto, pegado)
	}
	return &model
}

// Cada carácter del borde tiene que acabar en el campo de búsqueda o no, según
// lo que diga la comparación del código. Lo que se mira es el RESULTADO, no el
// código: si alguien cambia el rango, el campo deja de growing y el test lo ve.
func TestElModalDeFiltroAceptaSoloImprimibles(t *testing.T) {
	for _, tc := range teclasDelBorde {
		t.Run(tc.nombre, func(t *testing.T) {
			m := filtroAbierto(t)

			m, _ = pulsar(t, m, tc.tecla)

			if tc.acepta {
				if m.filterSearch != string(tc.caracter) {
					t.Errorf("el filtro no ha aceptado %q: filterSearch = %q",
						string(tc.caracter), m.filterSearch)
				}
				return
			}
			if m.filterSearch != "" {
				t.Errorf("el filtro ha aceptado %q, que no es imprimible: filterSearch = %q",
					tc.tecla, m.filterSearch)
			}
		})
	}
}

// Los dos modales rechazan el espacio, pero por razones distintas, y conviene
// que quede escrito.
//
// En el filtro es la comparación: el rango empieza en 33, y un espacio en el
// buscador de un desplegable no es una búsqueda, es un hueco.
//
// En tags es el `len(key) == 1` de delante: Bubbletea entrega la barra
// espaciadora con nombre ("space"), no como un carácter suelto, así que un byte
// 32 suelto nunca llega. El suelo 32 del rango documenta la intención pero no lo
// hace alcanzable; por eso el `>= 32` -> `> 32` es un mutante EQUIVALENTE y
// está en .mutation-allowlist con este motivo, no con el de un hueco.
//
// Lo que sí llega a los dos es un pegado, que va por otro camino y conserva los
// espacios. Lo que deja claro que la diferencia es del camino y no de que los
// espacios estén prohibidos.
func TestElEspacioNoLlegaNiAlFiltroNiATags(t *testing.T) {
	filtro := filtroAbierto(t)
	filtro, _ = pulsar(t, filtro, " ")
	if filtro.filterSearch != "" {
		t.Errorf("el filtro ha tomado el espacio: %q", filtro.filterSearch)
	}

	tags := newDetailModel(t, 0)
	tags, _ = pulsar(t, tags, "t")
	if !tags.tagOpen {
		t.Fatal("el modal de tags no se ha abierto con t")
	}
	tags, _ = pulsar(t, tags, " ")
	if tags.tagInput != "" {
		t.Errorf("tags ha tomado la barra espaciadora: %q", tags.tagInput)
	}

	tags = updateTagPaste(t, tags, "con espacios")
	if !strings.Contains(tags.tagInput, " ") {
		t.Errorf("un pegado con espacios ha llegado sin ellos: %q", tags.tagInput)
	}
}

// Los dos bordes altos: 126 se acepta y 127 no. Con el rango escrito como `<`
// en vez de `<=`, el 127 entraría; con `<=`, el 126 se quedaría fuera. Ningún
// test con una letra normal distingue eso.
func TestElBordeAltoDelRangoImprimible(t *testing.T) {
	for _, tc := range []struct {
		nombre string
		tecla  string
	}{
		{"tilde (126)", "~"},
		{"DEL (127)", "\x7f"},
	} {
		t.Run("filtro/"+tc.nombre, func(t *testing.T) {
			m := filtroAbierto(t)
			m, _ = pulsar(t, m, tc.tecla)
			aceptado := m.filterSearch != ""
			if tc.nombre[0] == 't' != aceptado {
				t.Errorf("filtro: %s aceptada=%v", tc.nombre, aceptado)
			}
		})
		t.Run("tags/"+tc.nombre, func(t *testing.T) {
			m := newDetailModel(t, 0)
			m, _ = pulsar(t, m, "t")
			if !m.tagOpen {
				t.Fatalf("el modal de tags no se ha abierto con t")
			}
			m, _ = pulsar(t, m, tc.tecla)
			aceptado := m.tagInput != ""
			if tc.nombre[0] == 't' != aceptado {
				t.Errorf("tags: %s aceptada=%v", tc.nombre, aceptado)
			}
		})
	}
}

// esImprimible es el rango de bytes que se acepta como tecla de un solo carácter,
// en el modal de tags y en el de filtros.
//
// Los dos bordes son alcanzables y comprobables, y eso es lo que hace el refactor:
// antes el rango del modal de tags empezaba en 32 (el espacio), un byte que
// Bubbletea nunca entrega suelto -- la barra espaciadora llega con nombre. Así que
// el suelo de 32 y el de 33 eran indistinguibles y el mutante de uno por otro
// sobrevivía. Con el suelo en el primer byte que sí llega, los dos lados existen.
func TestEsImprimible(t *testing.T) {
	casos := []struct {
		nombre string
		b      byte
		want   bool
	}{
		{"nulo", 0, false},
		{"tab (9)", 9, false},
		{"nueva línea (10)", 10, false},
		{"escape (27)", 27, false},
		{"justo debajo del suelo (32, el espacio)", 32, false},
		{"justo el suelo (33, exclamación)", 33, true},
		{"letra", 'a', true},
		{"cifra", '7', true},
		{"virgulina", ',', true},
		{"tilde (126)", 126, true},
		{"justo el techo (127, DEL)", 127, false},
		{"no-ASCII (200)", 200, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := esImprimible(c.b); got != c.want {
				t.Errorf("esImprimible(%d) = %v, want %v", c.b, got, c.want)
			}
		})
	}
}

// El recorrido entero del rango, byte a byte, para que un cambio en cualquiera de
// los dos extremos se note y no sólo los valores que happens a mirar un test.
func TestEsImprimibleRecorreElRangoEntero(t *testing.T) {
	for b := 0; b < 256; b++ {
		want := b >= primerByteImprimible && b < ultimoByteImprimible
		if got := esImprimible(byte(b)); got != want {
			t.Fatalf("esImprimible(%d) = %v, want %v: el rango declarado es [%d, %d)",
				b, got, want, primerByteImprimible, ultimoByteImprimible)
		}
	}
}
