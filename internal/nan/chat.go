package nan

import (
	"context"
	"encoding/json"
	"errors"
)

// Message is one turn of a chat conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is one completion request.
type ChatRequest struct {
	// Model is one of the allowlisted chat models. The list lives in internal/config
	// (I13); this package only sends what it is given.
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	// ResponseFormat asks for strict JSON when the answer has to be parsed.
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat asks the model for a shape. The schema is passed through unchanged: this
// package does not validate the caller's schema.
type ResponseFormat struct {
	// Type is "json_schema" or "text".
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema names and describes the JSON the model must produce.
type JSONSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict,omitempty"`
}

// ChatResponse is the OpenAI-compatible completion envelope.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

// ChatChoice is one candidate completion.
type ChatChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Text returns the content of the first choice, which is what a single-completion caller
// wants. Empty when the service returned no choices.
func (r *ChatResponse) Text() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

// Chat runs one completion.
//
// Calls are not serialized and there is no internal concurrency cap: four simultaneous
// requests were measured answering 200, and the exploration funnel already bounds how many
// callers exist. The ceiling belongs to the caller, not to the transport.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	switch {
	case req.Model == "":
		return nil, &Error{
			Op:   "chat",
			Kind: KindRequest,
			Err:  errors.New("model is required: this package never picks one (I13)"),
		}
	case len(req.Messages) == 0:
		return nil, &Error{Op: "chat", Kind: KindRequest, Err: errors.New("messages is empty")}
	}

	var out ChatResponse
	if err := c.post(ctx, "chat", c.chatURL, req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}
