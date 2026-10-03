package cli

import (
	"strings"
	"testing"
)

// Los "si hay algo que hacer" de `tsk update`.
//
// update acepta seis formas de cambiar una tarea y las aplica en cuatro bloques
// independientes: campos sueltos, --tags, --tag y --untag. Cada bloque está
// detrás de su propio "si hay algo", así que la tentación es un solo flag: sin
// --title no hay updates, y la llamada se salta entera.
//
// El problema es que saltarse un if que no tocaba no se ve en la salida: update
// sin flags imprime la tarea igual de bien saltándoselo que ejecutándolo. Lo
// único que lo distingue es si se ha TOCADO la base de datos, y por eso el
// sabotaje es una base de sólo lectura: si el bloque se ejecutara, el UPDATE
// saltaría un trigger y el comando saldría con error.
//
// Sin esta comprobación, cambiar `>` por `>=` -- o borrar el if entero -- deja
// la suite en verde mientras update empieza a escribir en cada llamada.

// sinNadaQueHacer es un update que no cambia nada: ni un solo flag de los que
// tocan la base.
func sinNadaQueHacer() []string { return []string{"update", "1"} }

func TestUpdateSinFlagsNoEscribe(t *testing.T) {
	conDBSoloLectura(t)

	// Sin flags, update sólo lee y muestra. Que la base sea de sólo lectura no
	// lo afecta: si escribiera, el trigger saltaría y esto fallaría.
	salida, code := run(t, sinNadaQueHacer()...)
	if code != 0 {
		t.Errorf("update sin flags ha fallado con la base de sólo lectura: %s", salida)
	}
	if !strings.Contains(salida, "una tarea") {
		t.Errorf("update sin flags no ha mostrado la tarea: %s", salida)
	}
}

// Los cuatro bloques por separado, con el mismo argumento: cada uno tiene que
// ser invisible cuando no le toca escribir.
func TestCadaBloqueDeUpdateSoloEscribeConSuFlag(t *testing.T) {
	casos := []struct {
		nombre string
		args   []string
	}{
		{"campos sueltos", []string{"update", "1", "--title", "otro"}},
		{"tags", []string{"update", "1", "--tags", "nueva"}},
		{"tag", []string{"update", "1", "--tag", "nueva"}},
		{"untag", []string{"update", "1", "--untag", "nueva"}},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			conDBSoloLectura(t)
			salida, code := runErr(t, tc.args...)
			if code != 1 {
				t.Errorf("%v: código %d, want 1: este bloque sí tenía que escribir", tc.args, code)
			}
			if !strings.Contains(salida, "sólo lectura") {
				t.Errorf("%v: el error no menciona la base de sólo lectura: %s", tc.args, salida)
			}
		})
	}
}

// --weeks ignora lo que no sea un entero positivo. El "positivo" es lo que
// importa: con `>= 0`, un `--weeks 0` se aceptaría y el gantt se dibujaría con
// cero semanas, que no es lo que pidió quien lo escribió.
func TestWeeksIgnoraLoQueNoSeaUnEnteroPositivo(t *testing.T) {
	// Las tres formas que tienen que ignorarse. Un entero negativo y un cero se
	// descartan por la comparación con 0; el texto ni siquiera llega a ser un
	// entero.
	for _, valor := range []string{"0", "-3", "abc", "", "2.5"} {
		t.Run("weeks="+valor, func(t *testing.T) {
			conDBReal(t)

			args := []string{"gantt", "--weeks", valor}
			if valor == "" {
				args = []string{"gantt", "--weeks"}
			}
			salida, code := run(t, args...)
			if code != 0 {
				t.Fatalf("--weeks %q ha fallado: %s", valor, salida)
			}
			// Lo que vale es el de la config, no el que se pasó. Con un valor
			// negativo el gantt se dibujaría al revés y con 0 no se dibujaría
			// nada, así que cualquier semanas positivo vale como prueba de que
			// el valor se descartó.
			if !strings.Contains(salida, "weeks") {
				t.Fatalf("la salida no parece un gantt: %s", salida)
			}
			semanas := semanasDeLaSalida(salida)
			if semanas <= 0 {
				t.Errorf("--weeks %q ha llegado al gantt: se画出 con %d semanas",
					valor, semanas)
			}
		})
	}

	// Y el que sí vale, que es el caso de que el filtro no se coma lo bueno.
	t.Run("un entero positivo se aplica", func(t *testing.T) {
		conDBReal(t)
		salida, code := run(t, "gantt", "--weeks", "3")
		if code != 0 {
			t.Fatalf("--weeks 3 ha fallado: %s", salida)
		}
		if got := semanasDeLaSalida(salida); got != 3 {
			t.Errorf("el gantt dice %d semanas, want 3", got)
		}
	})
}

// semanasDeLaSalida saca el número de la cabecera "Gantt · start <fecha> · N
// weeks". Devolver 0 cuando no lo encuentra hace que el test falle en vez de
// pasar por un default.
func semanasDeLaSalida(salida string) int {
	const sufijo = " weeks"
	i := strings.Index(salida, sufijo)
	if i < 0 {
		return 0
	}
	antes := strings.TrimRight(salida[:i], " ·")
	ultimo := antes[strings.LastIndex(antes, " ")+1:]
	n := 0
	for _, c := range ultimo {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
