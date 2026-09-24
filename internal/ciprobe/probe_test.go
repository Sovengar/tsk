// Package ciprobe existe únicamente para probar el bloqueo de CI del
// repositorio. No contiene código de producción.
package ciprobe

import "testing"

// TestProbeAlwaysFails falla a propósito: demuestra que un PR en rojo queda
// BLOCKED por el ruleset protect-main. Este PR jamás debe mergearse.
func TestProbeAlwaysFails(t *testing.T) {
	t.Fatal("intentional failure: CI blocking proof")
}
