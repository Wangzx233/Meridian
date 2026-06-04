package control

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestResolveCreateRunMode(t *testing.T) {
	task := Task{Status: TaskStatusOpen}
	mode, err := resolveCreateRunMode("auto", task)
	if err != nil {
		t.Fatalf("auto without session returned error: %v", err)
	}
	if mode != RunModeNew {
		t.Fatalf("auto without session = %q, want %q", mode, RunModeNew)
	}

	sessionID := "sess_123"
	task.CodexSessionID = &sessionID
	mode, err = resolveCreateRunMode("auto", task)
	if err != nil {
		t.Fatalf("auto with session returned error: %v", err)
	}
	if mode != RunModeResume {
		t.Fatalf("auto with session = %q, want %q", mode, RunModeResume)
	}

	task.CodexSessionID = nil
	_, err = resolveCreateRunMode(RunModeResume, task)
	if !errors.Is(err, ErrMissingCodexSession) {
		t.Fatalf("resume without session error = %v, want %v", err, ErrMissingCodexSession)
	}
}

func TestCreateRunStateSemantics(t *testing.T) {
	activeID := "run_active"
	task := Task{Status: TaskStatusWaitingUser, ActiveRunID: &activeID}
	if err := validateCreateRunState(task); !errors.Is(err, ErrActiveRunExists) {
		t.Fatalf("active run validation error = %v, want %v", err, ErrActiveRunExists)
	}

	for _, status := range []string{TaskStatusDone, TaskStatusArchived} {
		task := Task{Status: status}
		if err := validateCreateRunState(task); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("create run for task status %q error = %v, want %v", status, err, ErrInvalidState)
		}
	}

	task = Task{Status: TaskStatusOpen}
	if err := validateCreateRunState(task); err != nil {
		t.Fatalf("create run for open task returned error: %v", err)
	}
}

func TestRunStatusPredicatesAndTerminalTaskTransition(t *testing.T) {
	for _, status := range []string{RunStatusQueued, RunStatusRunning} {
		if !isActiveRunStatus(status) {
			t.Fatalf("%q should be active", status)
		}
	}
	for _, status := range []string{RunStatusSucceeded, RunStatusFailed, RunStatusCanceled} {
		if !isTerminalRunStatus(status) {
			t.Fatalf("%q should be terminal", status)
		}
		if isActiveRunStatus(status) {
			t.Fatalf("%q should not be active", status)
		}
	}
	if got := taskStatusAfterTerminalRun(TaskStatusRunning); got != TaskStatusWaitingUser {
		t.Fatalf("terminal run from running task = %q, want %q", got, TaskStatusWaitingUser)
	}
	if got := taskStatusAfterTerminalRun(TaskStatusDone); got != TaskStatusDone {
		t.Fatalf("terminal run from done task = %q, want done unchanged", got)
	}
}

func TestNormalizeCodexRunOptions(t *testing.T) {
	model, effort, tier, err := normalizeCodexRunOptions(" gpt-5.5 ", "high", " fast ")
	if err != nil {
		t.Fatalf("normalize options returned error: %v", err)
	}
	if model == nil || *model != "gpt-5.5" {
		t.Fatalf("model = %v, want gpt-5.5", model)
	}
	if effort == nil || *effort != "high" {
		t.Fatalf("effort = %v, want high", effort)
	}
	if tier == nil || *tier != "fast" {
		t.Fatalf("tier = %v, want fast", tier)
	}

	model, effort, tier, err = normalizeCodexRunOptions("", "", "")
	if err != nil {
		t.Fatalf("empty options returned error: %v", err)
	}
	if model != nil || effort != nil || tier != nil {
		t.Fatalf("empty options = %v/%v/%v, want nil/nil/nil", model, effort, tier)
	}

	if _, _, _, err = normalizeCodexRunOptions("gpt-5.5", "extreme", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid reasoning error = %v, want %v", err, ErrValidation)
	}
	if _, _, _, err = normalizeCodexRunOptions("gpt-5.5", "high", "slow"); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid service tier error = %v, want %v", err, ErrValidation)
	}
}

func TestBuildArgvIncludesCodexOptions(t *testing.T) {
	t.Setenv("CODEX_BYPASS_APPROVALS_AND_SANDBOX", "true")
	model := "gpt-5.5"
	effort := "xhigh"
	tier := "fast"
	argv := buildArgv("codex", "D:\\go\\workplace", RunModeNew, nil, &model, &effort, &tier, nil)
	got := strings.Join(argv, "\n")
	for _, want := range []string{"--model\ngpt-5.5", "--config\nmodel_reasoning_effort=\"xhigh\"", "--config\nservice_tier=\"fast\"", "exec\n--dangerously-bypass-approvals-and-sandbox\n--skip-git-repo-check\n--json\n-"} {
		if !strings.Contains(got, want) {
			t.Fatalf("argv %q missing %q", argv, want)
		}
	}
}

func TestBuildArgvCanPreserveCodexSandbox(t *testing.T) {
	t.Setenv("CODEX_BYPASS_APPROVALS_AND_SANDBOX", "false")
	argv := buildArgv("codex", "D:\\go\\workplace", RunModeNew, nil, nil, nil, nil, nil)
	got := strings.Join(argv, "\n")
	if strings.Contains(got, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("argv %q should not bypass approvals and sandbox", argv)
	}
	if !strings.Contains(got, "exec\n--skip-git-repo-check\n--json\n-") {
		t.Fatalf("argv %q missing normal exec flags", argv)
	}
}

func TestBuildArgvIncludesImagesForNewAndResume(t *testing.T) {
	t.Setenv("CODEX_BYPASS_APPROVALS_AND_SANDBOX", "true")
	sessionID := "sess_123"
	images := []RunInputImageAttachment{{Filename: "one.png"}, {Filename: "two.jpg"}}

	newArgv := buildArgv("codex", "D:\\go\\workplace", RunModeNew, nil, nil, nil, nil, images)
	gotNew := strings.Join(newArgv, "\n")
	if !strings.Contains(gotNew, "exec\n--dangerously-bypass-approvals-and-sandbox\n--image\none.png\n--image\ntwo.jpg\n--skip-git-repo-check\n--json\n-") {
		t.Fatalf("new argv %q missing image flags", newArgv)
	}

	resumeArgv := buildArgv("codex", "D:\\go\\workplace", RunModeResume, &sessionID, nil, nil, nil, images)
	gotResume := strings.Join(resumeArgv, "\n")
	if !strings.Contains(gotResume, "exec\nresume\n--dangerously-bypass-approvals-and-sandbox\n--image\none.png\n--image\ntwo.jpg\n--skip-git-repo-check\n--json\nsess_123\n-") {
		t.Fatalf("resume argv %q missing image flags", resumeArgv)
	}
}

func TestBuildRunPromptAllowsRawSlashCommands(t *testing.T) {
	prompt, err := buildRunPrompt(RunModeResume, Task{Title: "Compact"}, " /compact ", nil, true, false)
	if err != nil {
		t.Fatalf("build raw prompt returned error: %v", err)
	}
	if prompt != "/compact" {
		t.Fatalf("raw prompt = %q, want /compact", prompt)
	}
	prompt, err = buildRunPrompt(RunModeResume, Task{Title: "Goal"}, " /goal Finish the task ", nil, true, false)
	if err != nil {
		t.Fatalf("build goal prompt returned error: %v", err)
	}
	if prompt != "/goal Finish the task" {
		t.Fatalf("raw prompt = %q, want /goal Finish the task", prompt)
	}
	prompt, err = buildRunPrompt(RunModeResume, Task{Title: "Goal"}, " /goal clear ", nil, true, false)
	if err != nil {
		t.Fatalf("build goal clear prompt returned error: %v", err)
	}
	if prompt != "/goal clear" {
		t.Fatalf("raw prompt = %q, want /goal clear", prompt)
	}
	prompt, err = buildRunPrompt(RunModeResume, Task{Title: "Goal"}, " /goal ", nil, true, false)
	if err != nil {
		t.Fatalf("build bare goal prompt returned error: %v", err)
	}
	if prompt != "/goal" {
		t.Fatalf("raw prompt = %q, want /goal", prompt)
	}
	if _, err := buildRunPrompt(RunModeResume, Task{Title: "Compact"}, "/fast", nil, true, false); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid raw prompt error = %v, want %v", err, ErrValidation)
	}
}

func TestStatusForRunnerErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "unavailable", err: ErrRunnerUnavailable, status: http.StatusConflict, code: "runner_unavailable"},
		{name: "unsupported", err: ErrRunnerUnsupported, status: http.StatusConflict, code: "runner_unsupported"},
		{name: "timeout", err: ErrRunnerRequestTimeout, status: http.StatusGatewayTimeout, code: "runner_request_timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, _ := statusForError(tt.err)
			if status != tt.status || code != tt.code {
				t.Fatalf("statusForError(%v) = (%d, %q), want (%d, %q)", tt.err, status, code, tt.status, tt.code)
			}
		})
	}
}

func TestBuildPromptAlwaysIncludesTaskDescription(t *testing.T) {
	task := Task{Title: "Fix workbench UX", Description: "Make output primary."}

	newPrompt := buildPrompt(RunModeNew, task, "Implement", nil, false)
	if !strings.Contains(newPrompt, "Description:\nMake output primary.") {
		t.Fatalf("new prompt missing description:\n%s", newPrompt)
	}

	resumePrompt := buildPrompt(RunModeResume, task, "Continue", nil, false)
	if !strings.Contains(resumePrompt, "Description:\nMake output primary.") {
		t.Fatalf("resume prompt missing description:\n%s", resumePrompt)
	}

	emptyPrompt := buildPrompt(RunModeNew, Task{Title: "No description"}, "Implement", nil, false)
	if !strings.Contains(emptyPrompt, "Description:\n(no description provided)") {
		t.Fatalf("empty description prompt missing fallback:\n%s", emptyPrompt)
	}
}

func TestBuildPromptAddsLightReminderInstructionWhenEnabled(t *testing.T) {
	prompt := buildPrompt(RunModeNew, Task{Title: "Long task"}, "Run checks", nil, true)
	if count := strings.Count(prompt, "send-back"); count != 1 {
		t.Fatalf("prompt send-back count = %d, want 1:\n%s", count, prompt)
	}
	if strings.Contains(prompt, "MERIDIAN_NOTIFY_TOKEN") || strings.Contains(prompt, "127.0.0.1") {
		t.Fatalf("prompt leaked callback internals:\n%s", prompt)
	}
}
