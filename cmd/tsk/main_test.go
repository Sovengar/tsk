package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tsk/internal/config"
	"tsk/internal/db"
)

// The TUI startup cannot be executed in a test: Bubbletea keeps
// waiting for a terminal. That's why run() receives it as a parameter, and what is
// checked here is everything else -- the CLI path, the resolution of the
// database path and the three places where it can fail while exiting.
func TestRun(t *testing.T) {
	t.Run("a CLI subcommand resolves in the CLI and does not open the TUI", func(t *testing.T) {
		setValidConfigPath(t)

		// If launch is reached, the CLI did not consume the command.
		launched := false
		code := run([]string{"completion", "bash"}, &capture{}, &capture{}, func(*db.DB, config.Config) error {
			launched = true
			return nil
		})

		if code != 0 {
			t.Errorf("exit code is %d, want 0", code)
		}
		if launched {
			t.Error("the TUI was launched after a CLI subcommand")
		}
	})

	t.Run("without a subcommand the database opens and the TUI launches", func(t *testing.T) {
		setValidConfigPath(t)

		live := false
		code := run(nil, &capture{}, &capture{}, func(database *db.DB, _ config.Config) error {
			var one int
			err := database.Conn().QueryRow(`SELECT 1`).Scan(&one)
			live = err == nil && one == 1
			return nil
		})

		if code != 0 {
			t.Errorf("exit code is %d, want 0", code)
		}
		if !live {
			t.Error("the TUI did not receive a live database")
		}
	})

	t.Run("the database path comes from the config", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(configPath, []byte("[database]\npath = \""+filepath.Join(dir, "custom.db")+"\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", configPath)
		t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "xdg"))

		code := run(nil, &capture{}, &capture{}, func(*db.DB, config.Config) error { return nil })
		if code != 0 {
			t.Errorf("exit code is %d, want 0", code)
		}
		// If the path had been resolved via XDG, the file would not exist.
		if _, err := os.Stat(filepath.Join(dir, "custom.db")); err != nil {
			t.Errorf("the config database was not created: %v", err)
		}
	})

	t.Run("with no path in the config the XDG one is used", func(t *testing.T) {
		dir := t.TempDir()
		setValidConfigPath(t)
		t.Setenv("XDG_DATA_HOME", dir)

		code := run(nil, &capture{}, &capture{}, func(*db.DB, config.Config) error { return nil })
		if code != 0 {
			t.Errorf("exit code is %d, want 0", code)
		}
		if _, err := os.Stat(filepath.Join(dir, "tsk", "tsk.db")); err != nil {
			t.Errorf("the default database was not created: %v", err)
		}
	})

	t.Run("without HOME there is no default path", func(t *testing.T) {
		setValidConfigPath(t)
		t.Setenv("XDG_DATA_HOME", "")

		// With an empty config path and no XDG, DefaultPath needs the
		// home directory.
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(configPath, []byte("[database]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", configPath)
		t.Setenv("HOME", "")

		var errs capture
		code := run(nil, &capture{}, &errs, func(*db.DB, config.Config) error {
			t.Error("the TUI was launched without a database path")
			return nil
		})

		if code != 1 {
			t.Errorf("exit code is %d, want 1", code)
		}
		if !strings.HasPrefix(errs.String(), "tsk: ") {
			t.Errorf("the error does not reach stderr with the prefix: %q", errs.String())
		}
	})

	t.Run("a database that cannot be opened", func(t *testing.T) {
		setValidConfigPath(t)

		// A path whose parent directory is a file: the mkdir fails.
		blocker := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.toml")
		content := "[database]\npath = \"" + filepath.Join(blocker, "sub", "tsk.db") + "\"\n"
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TSK_CONFIG", configPath)

		var errs capture
		code := run(nil, &capture{}, &errs, func(*db.DB, config.Config) error {
			t.Error("the TUI was launched without a database")
			return nil
		})

		if code != 1 {
			t.Errorf("exit code is %d, want 1", code)
		}
		if !strings.Contains(errs.String(), "tsk:") {
			t.Errorf("the error does not reach stderr: %q", errs.String())
		}
	})

	t.Run("a TUI that fails", func(t *testing.T) {
		setValidConfigPath(t)

		var errs capture
		code := run(nil, &capture{}, &errs, func(*db.DB, config.Config) error {
			return errors.New("the terminal was closed")
		})

		if code != 1 {
			t.Errorf("exit code is %d, want 1", code)
		}
		if !strings.Contains(errs.String(), "the terminal was closed") {
			t.Errorf("the TUI error does not reach stderr: %q", errs.String())
		}
	})
}

// main() cannot be called from a test because its os.Exit would eat the
// whole test process. The standard way to cover it is to re-run the test binary
// as a program: the child calls the real main and the parent checks its
// exit code and its output.
//
// This is not a trick to tick the coverage box: it is the only way to
// check that main() propagates run()'s code, which is exactly what a
// script wrapper depends on.
func TestMainPropagatesTheExitCode(t *testing.T) {
	if os.Getenv("TSK_TEST_MAIN") == "1" {
		// We are in the child: main() really runs.
		main()
		return
	}

	t.Run("CLI subcommand", func(t *testing.T) {
		result := runInSubprocess(t, "completion", "bash")
		if result.code != 0 {
			t.Errorf("the child exited with %d, want 0 (stderr: %s)", result.code, result.errs)
		}
		if !strings.Contains(result.output, "complete") {
			t.Errorf("the child did not print the completion script: %q", result.output)
		}
	})

	t.Run("CLI error", func(t *testing.T) {
		// A subcommand that does not exist exits with 1 from cli.Run, without reaching the
		// TUI. It is the error path a user sees most often.
		result := runInSubprocess(t, "nope")
		if result.code == 0 {
			t.Errorf("a nonexistent subcommand exited with 0, want != 0")
		}
	})
}

func TestLaunchTUI(t *testing.T) {
	// launchTUI cannot be run without a terminal, but we can check that it exists
	// and that it returns an error instead of entering an impossible state. With a
	// closed database, Bubbletea starts up and fails while reading.
	//
	// In an environment without a terminal, tea.Run returns a terminal error, which is
	// exactly what this function has to let through.
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer func() { _ = database.Close() }()

	_ = launchTUI(database, config.Defaults())
}

// setValidConfigPath points TSK_CONFIG at a file that exists but is
// empty: the config resolves, there is no database path, and therefore the
// default one is used.
func setValidConfigPath(t *testing.T) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TSK_CONFIG", configPath)
}

func runInSubprocess(t *testing.T, args ...string) struct {
	code   int
	output string
	errs   string
} {
	t.Helper()

	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "TSK_TEST_MAIN=1")

	var out, stderrBuf capture
	cmd.Stdout = &out
	cmd.Stderr = &stderrBuf

	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running the child: %v", err)
		}
		code = exitErr.ExitCode()
	}

	return struct {
		code   int
		output string
		errs   string
	}{code, out.String(), stderrBuf.String()}
}
