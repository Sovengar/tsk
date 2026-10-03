package cli

import (
	"bytes"
)

// cola es un io.Writer que se puede inspeccionar después. Los tests de error
// necesitan leer lo que el programa escribió en stderr, que normalmente no es
// el mismo descriptor que el del proceso.
type cola struct {
	buf bytes.Buffer
}

func (c *cola) Write(b []byte) (int, error) { return c.buf.Write(b) }

func (c *cola) String() string { return c.buf.String() }
