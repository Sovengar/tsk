package cli

import (
	"bytes"
)

// capture is an inspectable io.Writer. The error tests need to read what the
// program wrote to stderr, which is usually not the same
// descriptor as the process's own.
type capture struct {
	buf bytes.Buffer
}

func (c *capture) Write(b []byte) (int, error) { return c.buf.Write(b) }

func (c *capture) String() string { return c.buf.String() }
