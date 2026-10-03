package tui

import (
	"strings"
	"testing"
)

// fuzzyFilter tiene dosapacity(topepor defecto) y un tope (max). Los cuatro
// bordes que separan las comparaciones son:
//
//	len(items) > max	-- cuántos elementos hay por debajo del tope
//	max > 0		-- si el tope está activo
//	score > score	-- el desempate del ranking
//	item < item		-- el desempate por nombre
//
// Ninguno se distingue con una lista corta y una query normal: si hay menos
// elementos que el tope, ambos lados dan lo mismo, y si no hay empates, el
// desempate no se ejecuta nunca. Lo que separa son tres situaciones concretas:
// el tope JUSTO, el tope desactivado, y la lista con empates.

func TestFuzzyCortaJustoEnElTope(t *testing.T) {
	// "bx" y "by": los dos matchean la query y los dos tienen la misma
	// puntuación, así que salen los dos sea cual sea la query.
	items := []string{"bx", "by"}

	got := fuzzyFilter(items, "b", 2)
	if len(got) != 2 {
		t.Errorf("con 2 elementos y tope 2 salen %d, want 2: el tope es un máximo, no un objetivo", len(got))
	}

	// Y con query vacía, que va por la otra rama del if.
	got = fuzzyFilter(items, "", 2)
	if len(got) != 2 {
		t.Errorf("con query vacía y tope 2 salen %d, want 2", len(got))
	}

	// Un elemento menos que el tope: no se toca nada.
	if got := fuzzyFilter(items, "b", 5); len(got) != 2 {
		t.Errorf("con tope de sobra salen %d, want 2", len(got))
	}

	// Y uno más que el tope, que es donde el corte sí se nota.
	if got := fuzzyFilter(items, "b", 1); len(got) != 1 {
		t.Errorf("con tope 1 salen %d, want 1", len(got))
	}
}

// El ranking con la query vacía no se reordena: sale en el orden original. Un
// orden alfabético ahí sería una diferencia visible en cuanto la lista no
// coincidiese con el abecedario, que es el caso normal.
func TestFuzzySinQueryRespetaElOrdenOriginal(t *testing.T) {
	items := []string{"@zoe", "@ana", "@maria"}

	got := fuzzyFilter(items, "", 0)
	want := "@zoe,@ana,@maria"
	if strings.Join(got, ",") != want {
		t.Errorf("sin query el orden es %v, want %s (el de entrada)", got, want)
	}
}

// El desempate por score: dos elementos con la misma puntuación se ordenan por
// nombre. Con el comparador mal hecho (`<` en vez de `>`) saldrían al revés, y
// sólo se ve si hay dos con la MISMA puntuación.
func TestFuzzyDesempataPorNombreConIgualPuntuacion(t *testing.T) {
	// El score es 1000 - idx*10 - (len - len(query)), así que dos elementos
	// empatan si tienen la misma longitud y la query aparece en la misma
	// posición. "bx" y "by" empatan a 999; "@ana" y "@ava" también.
	casos := []struct {
		nombre string
		items  []string
		query  string
		want   string
	}{
		{"letras distintas tras el match", []string{"by", "bx"}, "b", "bx,by"},
		{"nombres de persona", []string{"@ava", "@ana"}, "a", "@ana,@ava"},
		{"tres a la vez", []string{"@cxa", "@ana", "@bxa"}, "a", "@ana,@bxa,@cxa"},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			got := fuzzyFilter(tc.items, tc.query, 0)
			if strings.Join(got, ",") != tc.want {
				t.Errorf("fuzzyFilter(%v, %q) = %v, want %s (misma puntuación: alfabético)",
					tc.items, tc.query, got, tc.want)
			}
			// Y al revés: la entrada desordenada da la misma salida, que es
			// lo que prueba que el orden no viene de la entrada.
			invertida := append([]string{tc.items[len(tc.items)-1]}, tc.items[:len(tc.items)-1]...)
			if rev := fuzzyFilter(invertida, tc.query, 0); strings.Join(rev, ",") != tc.want {
				t.Errorf("con la entrada invertida sale %v, want %s", rev, tc.want)
			}
		})
	}
}

// El ranking por puntuación manda sobre el nombre: "api" gana a "hola" aunque
// "hola" sea anterior alfabéticamente. Sin este caso, un desempate bien hecho
// con el ranking mal hecho seguiría dando el mismo resultado.
func TestFuzzyLaPuntuacionMandaSobreElNombre(t *testing.T) {
	// "b" scorea 999 (match en la posición 0, longitud 1) y "ab" scorea 989
	// (match en la 1, longitud 2). Alfabéticamente "ab" va antes, así que este
	// caso es el que separa "ordeno por puntuación" de "ordeno por nombre": si
	// el comparador de score estuviese al revés, saldría "ab,b".
	items := []string{"ab", "b"}

	got := fuzzyFilter(items, "b", 0)
	if strings.Join(got, ",") != "b,ab" {
		t.Errorf("fuzzyFilter(%v, \"b\") = %v, want b,ab: la puntuación manda, aunque el nombre sea otro", items, got)
	}
}

// max a cero significa "sin tope", no "cero resultados". Es la diferencia entre
// un `max > 0` y un `max >= 0`, y con la lista vacía de resultados no se ve.
func TestFuzzyTopeCeroSignificaSinTope(t *testing.T) {
	items := []string{"@ana", "@bxa", "@cxa"}

	for _, max := range []int{0, 1, 5} {
		got := fuzzyFilter(items, "a", max)
		if len(got) == 0 {
			t.Errorf("con tope %d y una coincidencia sale vacío", max)
		}
		if len(got) > len(items) {
			t.Errorf("con tope %d salen %d elementos de %d", max, len(got), len(items))
		}
	}

	// Con tope 0 sale TODO lo que coincide, no nada.
	if got := fuzzyFilter(items, "a", 0); len(got) != 3 {
		t.Errorf("con tope 0 salen %d de 3, want 3: tope cero es sin tope", len(got))
	}

	// Con query vacía no hay ranking, pero el tope sigue cortando.
	if got := fuzzyFilter(items, "", 2); len(got) != 2 {
		t.Errorf("sin query y con tope 2 salen %d, want 2", len(got))
	}
}
