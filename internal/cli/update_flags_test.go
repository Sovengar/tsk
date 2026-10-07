package cli

import (
	"strings"
	"testing"
)

// The "is there anything to do" checks of `tsk update`.
//
// update accepts six ways to change a task and applies them in four independent
// blocks: loose fields, --tags, --tag and --untag. Each block sits
// behind its own "is there something", so the temptation is a single flag: without
// --title there are no updates, and the call is skipped entirely.
//
// The problem is that skipping an if that was not touched does not show up in the output: update
// without flags prints the task just as well skipping it as running it. The
// only thing that distinguishes it is whether the database was TOUCHED, and that's why the
// sabotage is a read-only database: if the block ran, the UPDATE
// would trip a trigger and the command would exit with an error.
//
// Without this check, changing `>` to `>=` -- or deleting the whole if -- leaves
// the suite green while update starts writing on every call.

// updateWithoutFlags is an update that changes nothing: not a single one of the flags
// that touch the database.
func updateWithoutFlags() []string { return []string{"update", "1"} }

func TestUpdateWithoutFlagsDoesNotWrite(t *testing.T) {
	withReadOnlyDB(t)

	// Without flags, update only reads and shows. The database being read-only does not
	// affect it: if it wrote, the trigger would trip and this would fail.
	output, code := run(t, updateWithoutFlags()...)
	if code != 0 {
		t.Errorf("update without flags failed on the read-only database: %s", output)
	}
	if !strings.Contains(output, "a task") {
		t.Errorf("update without flags did not show the task: %s", output)
	}
}

// The four blocks separately, with the same argument: each one has to
// be invisible when it is not its turn to write.
func TestEachUpdateBlockWritesOnlyWithItsFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"loose fields", []string{"update", "1", "--title", "other"}},
		{"tags", []string{"update", "1", "--tags", "new"}},
		{"tag", []string{"update", "1", "--tag", "new"}},
		{"untag", []string{"update", "1", "--untag", "new"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withReadOnlyDB(t)
			output, code := runErr(t, tc.args...)
			if code != 1 {
				t.Errorf("%v: code %d, want 1: this block did have to write", tc.args, code)
			}
			if !strings.Contains(output, "read-only database") {
				t.Errorf("%v: the error does not mention the read-only database: %s", tc.args, output)
			}
		})
	}
}

// --weeks ignores anything that is not a positive integer. The "positive" is what
// matters: with `>= 0`, a `--weeks 0` would be accepted and the gantt would be drawn with
// zero weeks, which is not what whoever wrote it asked for.
func TestWeeksIgnoresAnythingThatIsNotAPositiveInteger(t *testing.T) {
	// The three forms that have to be ignored. A negative integer and a zero are
	// discarded by the comparison with 0; the text never even becomes an
	// integer.
	for _, value := range []string{"0", "-3", "abc", "", "2.5"} {
		t.Run("weeks="+value, func(t *testing.T) {
			withRealDB(t)

			args := []string{"gantt", "--weeks", value}
			if value == "" {
				args = []string{"gantt", "--weeks"}
			}
			output, code := run(t, args...)
			if code != 0 {
				t.Fatalf("--weeks %q failed: %s", value, output)
			}
			// What counts is the config's, not the one passed in. With a
			// negative value the gantt would be drawn backwards and with 0 nothing
			// would be drawn, so any positive weeks serves as proof that
			// the value was discarded.
			if !strings.Contains(output, "weeks") {
				t.Fatalf("the output does not look like a gantt: %s", output)
			}
			weeks := weeksFromOutput(output)
			if weeks <= 0 {
				t.Errorf("--weeks %q reached the gantt: drawn with %d weeks",
					value, weeks)
			}
		})
	}

	// And the one that does count, which is the case where the filter does not eat the good one.
	t.Run("a positive integer is applied", func(t *testing.T) {
		withRealDB(t)
		output, code := run(t, "gantt", "--weeks", "3")
		if code != 0 {
			t.Fatalf("--weeks 3 failed: %s", output)
		}
		if got := weeksFromOutput(output); got != 3 {
			t.Errorf("the gantt says %d weeks, want 3", got)
		}
	})
}

// weeksFromOutput extracts the number from the "Gantt · start <date> · N
// weeks" header. Returning 0 when it is not found makes the test fail instead of
// passing through a default.
func weeksFromOutput(output string) int {
	const suffix = " weeks"
	i := strings.Index(output, suffix)
	if i < 0 {
		return 0
	}
	trimmed := strings.TrimRight(output[:i], " ·")
	last := trimmed[strings.LastIndex(trimmed, " ")+1:]
	n := 0
	for _, c := range last {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
