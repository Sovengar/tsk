package main

import (
	"bytes"
	"fmt"
	"io"
)

// capture is an inspectable io.Writer: tests need to read what the program
// writes to stdout and stderr without touching the process descriptors.
type capture struct {
	buf bytes.Buffer
}

var (
	_ io.Writer    = (*capture)(nil)
	_ fmt.Stringer = (*capture)(nil)
)

func (c *capture) Write(b []byte) (int, error) { return c.buf.Write(b) }

func (c *capture) String() string { return c.buf.String() }
