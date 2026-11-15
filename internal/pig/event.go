package pig

import (
	"encoding/json"
	"time"
)

// ToolCall is one tool the agent asked for, with what came back.
type ToolCall struct {
	ID       string
	Name     string
	Args     json.RawMessage
	Result   string
	IsError  bool
	Duration time.Duration
}

// Result accumulates what matters from the agent's event stream.
//
// PiG emits JSON Lines and most of the volume is streamed message updates, which we
// deliberately drop: only the tool trace and the final assistant message are kept.
type Result struct {
	Answer    string
	ToolCalls []ToolCall
	// Error is the agent's own reason when a turn ended in failure. It arrives on the same
	// assistant message that would have carried the answer, so dropping it leaves a run that
	// looks successful and empty.
	Error string

	byID map[string]int
}

// rawEvent is the union of the event shapes we read. Everything else is ignored.
type rawEvent struct {
	Type       string          `json:"type"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
	Args       json.RawMessage `json:"args"`
	Result     *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	IsError    bool  `json:"isError"`
	DurationMs int64 `json:"durationMs"`
	Message    *struct {
		Role         string          `json:"role"`
		Content      json.RawMessage `json:"content"`
		StopReason   string          `json:"stopReason"`
		ErrorMessage string          `json:"errorMessage"`
	} `json:"message"`
}

// consume parses one JSONL line and folds it into the result.
//
// A line that is not JSON is not an error: the agent may write diagnostics to the same
// stream. Silence about garbage is better than failing a whole exploration over it.
func (r *Result) consume(line []byte) {
	if len(line) == 0 {
		return
	}
	var ev rawEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}

	switch ev.Type {
	case "tool_execution_start":
		r.upsert(ev, false)
	case "tool_execution_end":
		r.upsert(ev, true)
	case "message_end":
		if ev.Message == nil || ev.Message.Role != "assistant" {
			return
		}
		if ev.Message.ErrorMessage != "" {
			r.Error = ev.Message.ErrorMessage
		}
		if text := messageText(ev.Message.Content); text != "" {
			r.Answer = text
		}
	}
}

func (r *Result) upsert(ev rawEvent, done bool) {
	if r.byID == nil {
		r.byID = make(map[string]int)
	}
	idx, ok := r.byID[ev.ToolCallID]
	if !ok {
		r.ToolCalls = append(r.ToolCalls, ToolCall{ID: ev.ToolCallID, Name: ev.ToolName, Args: ev.Args})
		idx = len(r.ToolCalls) - 1
		r.byID[ev.ToolCallID] = idx
	}
	if !done {
		return
	}
	tc := &r.ToolCalls[idx]
	tc.Name = ev.ToolName
	tc.IsError = ev.IsError
	tc.Duration = time.Duration(ev.DurationMs) * time.Millisecond
	if ev.Result != nil {
		for _, part := range ev.Result.Content {
			if part.Type == "text" {
				tc.Result += part.Text
			}
		}
	}
}

// messageText reads a message content that may be a plain string or a list of parts.
func messageText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var out string
	for _, p := range parts {
		if p.Type == "text" {
			out += p.Text
		}
	}
	return out
}

// ToolsUsed returns the tool names the agent actually invoked, in first-use order.
func (r *Result) ToolsUsed() []string {
	seen := make(map[string]bool, len(r.ToolCalls))
	out := make([]string, 0, len(r.ToolCalls))
	for _, tc := range r.ToolCalls {
		if seen[tc.Name] {
			continue
		}
		seen[tc.Name] = true
		out = append(out, tc.Name)
	}
	return out
}
