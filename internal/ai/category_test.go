package ai

import (
	"os"
	"path/filepath"
	"testing"
)

// writeCatalog drops one catalog file under a temporary agent directory and
// returns that directory.
func writeCatalog(t *testing.T, file, body string) string {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, categoryDir)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolveModelReadsTheStore(t *testing.T) {
	dir := writeCatalog(t, categoryStoreFile, `{"categories":{"dumb":{"model":"opencode-go/beam-free","thinking":"medium"}}}`)
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	model, thinking := ResolveModel("dumb", "")
	if model != "opencode-go/beam-free" || thinking != "medium" {
		t.Fatalf("model = %q, thinking = %q", model, thinking)
	}
}

func TestResolveModelKeepsTheExplicitThinking(t *testing.T) {
	dir := writeCatalog(t, categoryStoreFile, `{"categories":{"free":{"model":"p/free","thinking":"medium"}}}`)
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	if model, thinking := ResolveModel("free", "high"); model != "p/free" || thinking != "high" {
		t.Fatalf("model = %q, thinking = %q", model, thinking)
	}
}

func TestResolveModelReadsTheSeed(t *testing.T) {
	dir := writeCatalog(t, categorySeedFile, `{"categories":{"smart":{"model":"p/smart","effort":"low"}}}`)
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	if model, thinking := ResolveModel("smart", ""); model != "p/smart" || thinking != "low" {
		t.Fatalf("model = %q, thinking = %q", model, thinking)
	}
}

func TestResolveModelPrefersTheStoreOverTheSeed(t *testing.T) {
	dir := writeCatalog(t, categorySeedFile, `{"categories":{"dumb":{"model":"p/seed"}}}`)
	store := filepath.Join(dir, categoryDir, categoryStoreFile)
	if err := os.WriteFile(store, []byte(`{"categories":{"dumb":{"model":"p/store"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	if model, _ := ResolveModel("dumb", ""); model != "p/store" {
		t.Fatalf("model = %q, want the store entry", model)
	}
}

func TestResolveModelPassesThroughRawIDs(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", writeCatalog(t, categoryStoreFile, `{"categories":{}}`))

	if model, thinking := ResolveModel("provider/model", "high"); model != "provider/model" || thinking != "high" {
		t.Fatalf("model = %q, thinking = %q", model, thinking)
	}
}

func TestResolveModelPassesThroughEmptyAndUnknown(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", writeCatalog(t, categoryStoreFile, `{"categories":{}}`))

	if model, thinking := ResolveModel("", ""); model != "" || thinking != "" {
		t.Fatalf("model = %q, thinking = %q", model, thinking)
	}
	if model, _ := ResolveModel("nope", ""); model != "nope" {
		t.Fatalf("model = %q, want the name untouched", model)
	}
}

func TestResolveModelIgnoresAMalformedEntry(t *testing.T) {
	dir := writeCatalog(t, categoryStoreFile, `{"categories":{"broken":{"model":"not-a-model"}}}`)
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	if model, _ := ResolveModel("broken", ""); model != "broken" {
		t.Fatalf("model = %q, want the name untouched", model)
	}
}

func TestResolveModelIgnoresAnUnreadableCatalog(t *testing.T) {
	dir := writeCatalog(t, categoryStoreFile, `{not json`)
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	if model, _ := ResolveModel("dumb", ""); model != "dumb" {
		t.Fatalf("model = %q, want the name untouched", model)
	}
}

func TestResolveModelWithoutACatalog(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())

	if model, _ := ResolveModel("dumb", ""); model != "dumb" {
		t.Fatalf("model = %q, want the name untouched", model)
	}
}

func TestAgentDirHonoursTheOverride(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "/custom/agent")
	if got := AgentDir(); got != "/custom/agent" {
		t.Fatalf("dir = %q", got)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "~/piagent")
	if got := AgentDir(); got != filepath.Join(home, "piagent") {
		t.Fatalf("dir = %q, want the expanded home", got)
	}

	t.Setenv("PI_CODING_AGENT_DIR", "")
	if got := AgentDir(); got != filepath.Join(home, ".pi", "agent") {
		t.Fatalf("dir = %q, want the default under home", got)
	}

	t.Setenv("HOME", "")
	if got := AgentDir(); got != "" {
		t.Fatalf("dir = %q, want empty when home is unresolved", got)
	}
}

func TestResolveModelWithoutAnAgentDir(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")

	if model, _ := ResolveModel("dumb", ""); model != "dumb" {
		t.Fatalf("model = %q, want the name untouched", model)
	}
}

func TestIsModelID(t *testing.T) {
	cases := map[string]bool{
		"p/m":     true,
		"p/":      false,
		"/m":      false,
		"pm":      false,
		" /m":     false,
		"p/ ":     false,
		"p/q/r":   true,
		"":        false,
	}
	for value, want := range cases {
		if got := isModelID(value); got != want {
			t.Errorf("isModelID(%q) = %v, want %v", value, got, want)
		}
	}
}
