package main

import (
	"bytes"
	"fmt"
	"io"
)

// pila es un io.Writer que se puede inspeccionar: los tests necesitan leer lo
// que el programa escribe en stdout y stderr sin tocar los descriptores del
// proceso.
type pila struct {
	buf bytes.Buffer
}

var (
	_ io.Writer    = (*pila)(nil)
	_ fmt.Stringer = (*pila)(nil)
)

func (p *pila) Write(b []byte) (int, error) { return p.buf.Write(b) }

func (p *pila) String() string { return p.buf.String() }
