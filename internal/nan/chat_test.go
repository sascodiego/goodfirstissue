package nan_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

const chatModel = "glm5.3-flash"

// The chat envelope is the OpenAI-compatible one the NaN provider answers.
func TestChatSendsTheDocumentedRequest(t *testing.T) {
	var gotBody []byte
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testKey {
			t.Errorf("Authorization = %q, want the bearer key", got)
		}
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = rw.Write([]byte(`{"choices":[]}`))
	})

	temperature := 0.2
	_, err := tc.Client.Chat(context.Background(), nan.ChatRequest{
		Model: chatModel,
		Messages: []nan.Message{
			{Role: "system", Content: "You extract capabilities."},
			{Role: "user", Content: "CV text"},
		},
		Temperature: &temperature,
		ResponseFormat: &nan.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &nan.JSONSchema{
				Name:   "capabilities",
				Schema: json.RawMessage(`{"type":"object"}`),
				Strict: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	assertJSONEqual(t, gotBody, `{
		"model": "glm5.3-flash",
		"messages": [
			{"role": "system", "content": "You extract capabilities."},
			{"role": "user", "content": "CV text"}
		],
		"temperature": 0.2,
		"response_format": {
			"type": "json_schema",
			"json_schema": {"name": "capabilities", "schema": {"type": "object"}, "strict": true}
		}
	}`)
}

func TestChatDecodesTheDocumentedResponse(t *testing.T) {
	const answer = `{"skills":[{"name":"Go","level":"used"}]}`
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		_, _ = rw.Write([]byte(`{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"model": "glm5.3-flash",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": ` + jsonString(answer) + `},
				"finish_reason": "stop"
			}],
			"usage": {"input_tokens": 120, "output_tokens": 34}
		}`))
	})

	got, err := tc.Client.Chat(context.Background(), nan.ChatRequest{
		Model:    chatModel,
		Messages: []nan.Message{{Role: "user", Content: "CV text"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if got.Text() != answer {
		t.Errorf("Text() = %q, want the assistant content", got.Text())
	}
	if len(got.Choices) != 1 || got.Choices[0].FinishReason != "stop" {
		t.Errorf("choices = %+v, want one finished choice", got.Choices)
	}
	if got.Usage.InputTokens != 120 || got.Usage.OutputTokens != 34 {
		t.Errorf("usage = %+v, want 120 input and 34 output", got.Usage)
	}
}

// jsonString quotes a string as a JSON literal without hand-escaping it.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// S2: the chat path is not serialized. Four simultaneous requests were measured working, and
// serializing them would stall the exploration for nothing.
func TestChatRunsConcurrently(t *testing.T) {
	var seen concurrency
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		seen.enter()
		defer seen.leave()
		time.Sleep(20 * time.Millisecond)
		_, _ = rw.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	})

	const callers = 4
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = tc.Client.Chat(context.Background(), nan.ChatRequest{
				Model:    chatModel,
				Messages: []nan.Message{{Role: "user", Content: "CV text"}},
			})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d error = %v", i, err)
		}
	}
	if peak := seen.peak(); peak < 2 {
		t.Errorf("the server saw %d requests in flight, want more than 1: chat must not serialize", peak)
	}
}

// S6: the package never picks a model, and it refuses to guess.
func TestChatRefusesAnIncompleteRequest(t *testing.T) {
	tc := newTestClient(t, func(rw http.ResponseWriter, r *http.Request) {
		t.Error("the server was called with an invalid request")
	})

	cases := map[string]nan.ChatRequest{
		"no model":    {Messages: []nan.Message{{Role: "user", Content: "x"}}},
		"no messages": {Model: chatModel},
	}
	for name, req := range cases {
		_, err := tc.Client.Chat(context.Background(), req)
		if e := asError(t, err); e.Kind != nan.KindRequest {
			t.Errorf("%s: Kind = %q, want %q", name, e.Kind, nan.KindRequest)
		}
	}
}
