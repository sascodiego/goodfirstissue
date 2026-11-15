package pig

import (
	"strings"
	"testing"
)

const (
	lineStart   = `{"type":"tool_execution_start","toolCallId":"call_1","toolName":"read","args":{"path":"command.go","offset":1590}}`
	lineEnd     = `{"type":"tool_execution_end","toolCallId":"call_1","toolName":"read","result":{"content":[{"type":"text","text":"package cobra"}]},"isError":false,"durationMs":3}`
	lineStart2  = `{"type":"tool_execution_start","toolCallId":"call_2","toolName":"mcp__codegraph__codegraph_explore","args":{"query":"usage width"}}`
	lineEnd2    = `{"type":"tool_execution_end","toolCallId":"call_2","toolName":"mcp__codegraph__codegraph_explore","result":{"content":[{"type":"text","text":"Commands at command.go:1332"}]},"isError":true,"durationMs":138}`
	lineAsk     = `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"{\"intent\":\"x\"}"}]}}`
	lineAskStr  = `{"type":"message_end","message":{"role":"assistant","content":"plain answer"}}`
	lineUser    = `{"type":"message_end","message":{"role":"user","content":"do not take me"}}`
	lineNoise   = `{"type":"message_update","delta":"streaming noise"}`
	lineGarbage = `not json at all`
)

// The stream is JSONL and most of it is noise; only the trace and the final answer count.
func TestResultConsumeBuildsTraceAndAnswer(t *testing.T) {
	var r Result
	for _, l := range []string{
		lineNoise, lineGarbage, lineStart, lineEnd,
		lineStart2, lineEnd2, lineUser, lineAsk,
	} {
		r.consume([]byte(l))
	}

	if len(r.ToolCalls) != 2 {
		t.Fatalf("got %d tool calls, want 2: %+v", len(r.ToolCalls), r.ToolCalls)
	}

	first := r.ToolCalls[0]
	if first.Name != "read" || first.IsError || first.Duration.Milliseconds() != 3 {
		t.Errorf("first call = %+v, want read/ok/3ms", first)
	}
	if first.Result != "package cobra" {
		t.Errorf("first result = %q", first.Result)
	}
	if string(first.Args) != `{"path":"command.go","offset":1590}` {
		t.Errorf("args not preserved: %s", first.Args)
	}

	second := r.ToolCalls[1]
	if !second.IsError {
		t.Error("second call should be marked as error")
	}
	if second.Duration.Milliseconds() != 138 {
		t.Errorf("second duration = %v, want 138ms", second.Duration)
	}

	// The user message must not replace the assistant answer.
	if r.Answer != `{"intent":"x"}` {
		t.Errorf("answer = %q, want the assistant JSON", r.Answer)
	}
}

// A message content may be a plain string instead of a list of parts.
func TestResultConsumeAcceptsStringContent(t *testing.T) {
	var r Result
	r.consume([]byte(lineAskStr))
	if r.Answer != "plain answer" {
		t.Errorf("answer = %q, want %q", r.Answer, "plain answer")
	}
}

// An end event without a start must still be recorded: the trace may be truncated.
func TestResultConsumeEndWithoutStart(t *testing.T) {
	var r Result
	r.consume([]byte(lineEnd))
	if len(r.ToolCalls) != 1 || r.ToolCalls[0].Name != "read" {
		t.Fatalf("got %+v, want one read call", r.ToolCalls)
	}
}

func TestToolsUsedIsDeduplicatedInOrder(t *testing.T) {
	var r Result
	for _, l := range []string{lineStart, lineEnd, lineStart2, lineEnd2, lineStart, lineEnd} {
		r.consume([]byte(l))
	}
	got := r.ToolsUsed()
	want := []string{"read", "mcp__codegraph__codegraph_explore"}
	if len(got) != len(want) {
		t.Fatalf("ToolsUsed() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ToolsUsed() = %v, want %v", got, want)
		}
	}
}

// La API rechazando la clave llega como un mensaje de asistente con stopReason "error" y el
// motivo en errorMessage: el turno termina bien y sin contenido, así que descartarlo deja una
// corrida que se ve exitosa y vacía.
func TestResultConsumeKeepsTheErrorMessage(t *testing.T) {
	const lineError = `{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"401: {\"message\":\"Invalid API key.\",\"type\":\"auth_error\",\"code\":\"401\"}"}}`

	var r Result
	r.consume([]byte(lineError))

	if r.Error == "" {
		t.Fatal("the error message was dropped: the run looks successful and empty")
	}
	if !strings.Contains(r.Error, "Invalid API key") {
		t.Errorf("error = %q, want the API's own words", r.Error)
	}
	if r.Answer != "" {
		t.Errorf("answer = %q, want none", r.Answer)
	}
}

// Un turno sin error no inventa uno.
func TestResultConsumeLeavesTheErrorEmptyOnSuccess(t *testing.T) {
	var r Result
	r.consume([]byte(lineAsk))

	if r.Error != "" {
		t.Errorf("error = %q, want none", r.Error)
	}
}
