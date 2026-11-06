package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSpec = `{"title":"Resume voice-gate","project":"voice-gate","assignee":"@sovengar",` +
	`"description":"Read the recovery notes.","tags":["voice"],"priority":2,"estimate":1}`

func TestParseSpecReadsProseAndFence(t *testing.T) {
	output := "Thought for a moment.\n\n```json\n" + validSpec + "\n```\n"
	spec, err := ParseSpec(output)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Title != "Resume voice-gate" || spec.Project != "voice-gate" || spec.Priority != 2 {
		t.Fatalf("spec = %+v", spec)
	}
	if len(spec.Tags) != 1 || spec.Tags[0] != "voice" {
		t.Fatalf("tags = %v", spec.Tags)
	}
}

func TestParseSpecPrefersTheLastObject(t *testing.T) {
	output := `{"title":"first","project":"a"}` + "\nnow the real one\n" + validSpec
	spec, err := ParseSpec(output)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Title != "Resume voice-gate" {
		t.Fatalf("title = %q, want the last object", spec.Title)
	}
}

func TestParseSpecSkipsObjectsWithoutTitle(t *testing.T) {
	output := `{"project":"no title"}` + "\n" + validSpec
	spec, err := ParseSpec(output)
	if err != nil || spec.Title != "Resume voice-gate" {
		t.Fatalf("spec = %+v, err = %v", spec, err)
	}
}

func TestParseSpecHandlesBracesAndQuotesInsideStrings(t *testing.T) {
	output := `{"title":"Fix } parser","project":"tsk","description":"a \" quoted } brace"}`
	spec, err := ParseSpec(output)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Description != `a " quoted } brace` {
		t.Fatalf("description = %q", spec.Description)
	}
}

func TestParseSpecRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseSpec(`{"title": }`); !errors.Is(err, ErrNoJSONObject) {
		t.Fatalf("err = %v, want ErrNoJSONObject", err)
	}
}

func TestParseSpecRejectsUnclosedObject(t *testing.T) {
	if _, err := ParseSpec(`{"title":"x"`); !errors.Is(err, ErrNoJSONObject) {
		t.Fatalf("err = %v, want ErrNoJSONObject", err)
	}
}

func TestParseSpecIgnoresStrayCloseBrace(t *testing.T) {
	if _, err := ParseSpec(`} {}`); !errors.Is(err, ErrNoJSONObject) {
		t.Fatalf("err = %v, want ErrNoJSONObject", err)
	}
}

func TestValidateAcceptance(t *testing.T) {
	spec := Spec{
		Title:       "  Resume voice-gate  ",
		Project:     " voice-gate ",
		Assignee:    "sovengar",
		Description: "  steps  ",
	}
	v := Validation{Projects: []string{"voice-gate"}, Assignees: []string{"@sovengar"}}
	if err := spec.Validate(v); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if spec.Title != "Resume voice-gate" || spec.Project != "voice-gate" {
		t.Fatalf("spec not trimmed: %+v", spec)
	}
	if spec.Assignee != "@sovengar" || spec.Description != "steps" {
		t.Fatalf("spec not normalised: %+v", spec)
	}
}

func TestValidateAppliesTheDefaultAssignee(t *testing.T) {
	spec := Spec{Title: "T", Project: "p", Description: "d"}
	v := Validation{Projects: []string{"p"}, Assignees: []string{"@sovengar"}, DefaultAssignee: "@sovengar"}
	if err := spec.Validate(v); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if spec.Assignee != "@sovengar" {
		t.Fatalf("assignee = %q", spec.Assignee)
	}
}

func TestValidateRejections(t *testing.T) {
	v := Validation{Projects: []string{"p"}, Assignees: []string{"@sovengar"}}
	cases := map[string]struct {
		spec Spec
		want error
	}{
		"empty title":       {Spec{Project: "p", Description: "d"}, ErrEmptyTitle},
		"title too long":    {Spec{Title: strings.Repeat("a", MaxTitleRunes+1), Project: "p", Description: "d"}, ErrTitleTooLong},
		"empty description": {Spec{Title: "T", Project: "p"}, ErrEmptyDescription},
		"unknown project":   {Spec{Title: "T", Project: "nope", Description: "d"}, ErrUnknownProject},
		"unknown assignee":  {Spec{Title: "T", Project: "p", Description: "d", Assignee: "@x"}, ErrUnknownAssignee},
		"title of 70 runes": {Spec{Title: strings.Repeat("a", MaxTitleRunes-1), Project: "p", Description: "d"}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			spec := tc.spec
			err := spec.Validate(v)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRunBuildsTheAgentbridgeArgv(t *testing.T) {
	var gotName string
	var gotArgv []string
	restore := runAgent
	runAgent = func(_ context.Context, name string, argv []string) (string, error) {
		gotName, gotArgv = name, argv
		return validSpec, nil
	}
	defer func() { runAgent = restore }()

	out, err := Run(context.Background(), Request{Harness: "pi", Model: "m", Thinking: "high", Prompt: "P"})
	if err != nil || out != validSpec {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if gotName != "agentbridge" {
		t.Fatalf("name = %q", gotName)
	}
	want := []string{"exec", "--harness", "pi", "--model", "m", "--effort", "high", "--prompt", "P"}
	if strings.Join(gotArgv, " ") != strings.Join(want, " ") {
		t.Fatalf("argv = %v, want %v", gotArgv, want)
	}
}

func TestRunOmitsEmptyModelAndEffort(t *testing.T) {
	var gotArgv []string
	restore := runAgent
	runAgent = func(_ context.Context, _ string, argv []string) (string, error) {
		gotArgv = argv
		return "", nil
	}
	defer func() { runAgent = restore }()

	if _, err := Run(context.Background(), Request{Harness: "pi", Prompt: "P"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := "exec --harness pi --prompt P"
	if strings.Join(gotArgv, " ") != want {
		t.Fatalf("argv = %v, want %q", gotArgv, want)
	}
}

func TestRunPassesTheWorkingDirectory(t *testing.T) {
	var gotArgv []string
	restore := runAgent
	runAgent = func(_ context.Context, _ string, argv []string) (string, error) {
		gotArgv = argv
		return "", nil
	}
	defer func() { runAgent = restore }()

	if _, err := Run(context.Background(), Request{Harness: "pi", Prompt: "P", Cwd: "/work"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := "exec --harness pi --cwd /work --prompt P"
	if strings.Join(gotArgv, " ") != want {
		t.Fatalf("argv = %v, want %q", gotArgv, want)
	}
}

func TestRunReturnsTheHarnessFailure(t *testing.T) {
	restore := runAgent
	runAgent = func(_ context.Context, _ string, _ []string) (string, error) {
		return "boom", ErrHarnessNotStarted
	}
	defer func() { runAgent = restore }()

	if _, err := Run(context.Background(), Request{Harness: "pi"}); !errors.Is(err, ErrHarnessNotStarted) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunAgentReturnsTheOutputOnSuccess(t *testing.T) {
	out, err := runAgent(context.Background(), "sh", []string{"-c", "printf ok"})
	if err != nil || out != "ok" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}

func TestRunAgentReportsTheStderrTail(t *testing.T) {
	_, err := runAgent(context.Background(), "sh", []string{"-c", "echo nope >&2; exit 3"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v, want the stderr tail", err)
	}
}

func TestRunWithRepairAcceptsTheFirstAnswer(t *testing.T) {
	calls := 0
	restore := runAgent
	runAgent = func(_ context.Context, _ string, _ []string) (string, error) {
		calls++
		return validSpec, nil
	}
	defer func() { runAgent = restore }()

	spec, out, err := RunWithRepair(context.Background(), Request{Harness: "pi", Prompt: "P"})
	if err != nil || spec.Title != "Resume voice-gate" || out != validSpec {
		t.Fatalf("spec = %+v, out = %q, err = %v", spec, out, err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRunWithRepairRetriesOnce(t *testing.T) {
	var prompts []string
	restore := runAgent
	runAgent = func(_ context.Context, _ string, argv []string) (string, error) {
		prompts = append(prompts, argv[len(argv)-1])
		if len(prompts) == 1 {
			return "no json here", nil
		}
		return validSpec, nil
	}
	defer func() { runAgent = restore }()

	spec, _, err := RunWithRepair(context.Background(), Request{Harness: "pi", Prompt: "P"})
	if err != nil || spec.Title != "Resume voice-gate" {
		t.Fatalf("spec = %+v, err = %v", spec, err)
	}
	if len(prompts) != 2 {
		t.Fatalf("prompts = %d, want 2", len(prompts))
	}
	if !strings.Contains(prompts[1], "# Repair") || !strings.Contains(prompts[1], ErrNoJSONObject.Error()) {
		t.Fatalf("repair prompt = %q", prompts[1])
	}
}

func TestRunWithRepairFailsWhenTheFirstRunFails(t *testing.T) {
	restore := runAgent
	runAgent = func(_ context.Context, _ string, _ []string) (string, error) {
		return "boom", ErrHarnessNotStarted
	}
	defer func() { runAgent = restore }()

	if _, _, err := RunWithRepair(context.Background(), Request{Harness: "pi"}); !errors.Is(err, ErrHarnessNotStarted) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunWithRepairFailsWhenTheSecondRunFails(t *testing.T) {
	calls := 0
	restore := runAgent
	runAgent = func(_ context.Context, _ string, _ []string) (string, error) {
		calls++
		if calls == 1 {
			return "no json", nil
		}
		return "boom", ErrHarnessNotStarted
	}
	defer func() { runAgent = restore }()

	if _, _, err := RunWithRepair(context.Background(), Request{Harness: "pi"}); !errors.Is(err, ErrHarnessNotStarted) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunWithRepairFailsWhenBothAnswersAreInvalid(t *testing.T) {
	restore := runAgent
	runAgent = func(_ context.Context, _ string, _ []string) (string, error) {
		return "no json", nil
	}
	defer func() { runAgent = restore }()

	if _, _, err := RunWithRepair(context.Background(), Request{Harness: "pi"}); !errors.Is(err, ErrNoJSONObject) {
		t.Fatalf("err = %v", err)
	}
}

func TestNotifySkippedWhenDisabled(t *testing.T) {
	called := false
	restore := sendNotification
	sendNotification = func(_ string, _ ...string) error {
		called = true
		return nil
	}
	defer func() { sendNotification = restore }()

	if err := Notify(false, "summary", "body"); err != nil {
		t.Fatalf("err = %v", err)
	}
	if called {
		t.Fatal("a disabled notification must not call notify-send")
	}
}

func TestNotifyCallsNotifySend(t *testing.T) {
	var got []string
	restore := sendNotification
	sendNotification = func(name string, argv ...string) error {
		got = append([]string{name}, argv...)
		return nil
	}
	defer func() { sendNotification = restore }()

	if err := Notify(true, "tsk", "created"); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "notify-send -a tsk tsk created"
	if strings.Join(got, " ") != want {
		t.Fatalf("call = %v, want %q", got, want)
	}
}

func TestNotifyPropagatesTheFailure(t *testing.T) {
	restore := sendNotification
	sendNotification = func(_ string, _ ...string) error { return errors.New("no daemon") }
	defer func() { sendNotification = restore }()

	if err := Notify(true, "a", "b"); err == nil {
		t.Fatal("want the notify-send failure")
	}
}

func TestSendNotificationRunsTheCommand(t *testing.T) {
	if err := sendNotification("true"); err != nil {
		t.Fatalf("err = %v", err)
	}
	if err := sendNotification("false"); err == nil {
		t.Fatal("want an error from a failing command")
	}
}

func TestStateDirHonoursEnvironment(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	got, err := StateDir()
	if err != nil || got != filepath.Join("/state", "tsk", "ai") {
		t.Fatalf("dir = %q, err = %v", got, err)
	}

	t.Setenv("XDG_STATE_HOME", "")
	got, err = StateDir()
	if err != nil || !strings.HasSuffix(got, filepath.Join(".local", "state", "tsk", "ai")) {
		t.Fatalf("dir = %q, err = %v", got, err)
	}

	t.Setenv("HOME", "")
	if _, err := StateDir(); err == nil {
		t.Fatal("an unresolved home directory must be an error")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
