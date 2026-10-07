package bordered

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// timeout is a generous deadline: if the parsing loop hangs, what is wanted
// is for the test to say so, not for the whole suite to hang.
func timeout() <-chan struct{} {
	ch := make(chan struct{})
	time.AfterFunc(2*time.Second, func() { close(ch) })
	return ch
}

// The borders have three "nothing to draw" paths that no test stepped on:
// a box whose content stays empty, a lone ESC in the middle of the text (which
// does not open a CSI and therefore is text), and a text with no styles that
// is split into a single piece.

// A box with no content has to be a box, not an empty string: the border is
// what makes it recognizable.
func TestEmptyBox(t *testing.T) {
	for _, content := range []string{"", "\n", "\n\n\n", "   ", "\t"} {
		t.Run(strings.ReplaceAll(content, "\n", "nl"), func(t *testing.T) {
			out := RenderWithTitleEx(lipgloss.RoundedBorder(), lipgloss.Color("8"),
				AlignLeft, " Título ", content, 20)

			if out == "" {
				t.Fatal("a box with no content came out empty")
			}
			if !strings.Contains(out, "╭") || !strings.Contains(out, "╰") {
				t.Errorf("a box with no content has no borders:\n%s", out)
			}
			// And all the lines have the requested width: a shorter line is
			// exactly what repeats with the empty block.
			for _, line := range strings.Split(out, "\n") {
				if n := ansi.StringWidth(line); n != 20 {
					t.Errorf("a line measures %d, want 20:\n%s", n, line)
				}
			}
		})
	}
}

// An ESC that does not open a CSI sequence is text, and the loop has to
// advance one or it would never come out. The most direct way to check it is
// for the function to finish.
func TestLoneEscIsText(t *testing.T) {
	for _, s := range []string{
		"\x1b",              // ESC at the start
		"a\x1bb",            // ESC in the middle
		"\x1b\x1b",          // ESC ESC
		"text\x1bmore",      // lone ESC inside
		"\x1b[",             // ESC without the rest of the sequence
		"\x1b]0;título\x07", // OSC: not CSI but it is text
	} {
		t.Run(strings.ReplaceAll(strings.ReplaceAll(s, "\x1b", "ESC"), "\x07", "BEL"), func(t *testing.T) {
			done := make(chan []ansiSegment, 1)
			go func() { done <- parseAnsiSegments(s) }()

			select {
			case segs := <-done:
				if len(segs) == 0 {
					t.Errorf("%q came out with no segments, want at least one", s)
				}
				// And the segments have to cover the whole string between
				// text and style. A truncated "\x1b[" goes entirely to the style
				// and none to the text, so both have to be looked at.
				var joined strings.Builder
				for _, seg := range segs {
					joined.WriteString(seg.style)
					joined.WriteString(seg.text)
				}
				// ansi.Strip is no use here: it also takes the lone ESC, which is
				// exactly what is being checked. What matters is that the segments
				// return all the bytes.
				if joined.Len() != len(s) {
					t.Errorf("the segments cover %d bytes of %d", joined.Len(), len(s))
				}
			case <-timeout():
				t.Fatalf("%q hung the parsing loop", s)
			}
		})
	}
}

// A text with no style at all splits into a single piece. The empty `chunks`
// at the end is the defense for when the text is exactly "".
func TestTextWithoutStyles(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := wrapLine("", 20); len(got) != 1 || got[0] != "" {
			t.Errorf("wrapLine(%q) = %q, want a single empty chunk", "", got)
		}
	})

	t.Run("normal", func(t *testing.T) {
		if got := wrapLine("hello", 20); len(got) != 1 || got[0] != "hello" {
			t.Errorf("wrapLine(%q) = %q, want a single chunk", "hello", got)
		}
	})

	t.Run("with styles", func(t *testing.T) {
		styled := "\x1b[31mred\x1b[0m"
		got := wrapLine(styled, 20)
		if len(got) != 1 {
			t.Fatalf("wrapLine with styles gave %d chunks (%q), want 1", len(got), got)
		}
		if clean := ansi.Strip(got[0]); clean != "red" {
			t.Errorf("the styled text is %q, want red", clean)
		}
	})
}
