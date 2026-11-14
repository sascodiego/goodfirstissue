package nan

import (
	"context"
	"errors"
	"fmt"
)

// MaxQuestions is the API's documented ceiling per request. Grouping questions about the
// same state is free; going past the ceiling is a guaranteed 400.
const MaxQuestions = 64

// QuestionType is one of the three answer shapes Clef documents.
type QuestionType string

const (
	// Noul answers with the probability that the answer is "yes".
	Noul QuestionType = "noul"
	// Choice answers with one of the criteria ids.
	Choice QuestionType = "choice"
	// Score answers with the weighted mean of an ordered list of levels.
	Score QuestionType = "score"
)

// Question is one thing to ask about the state. A batch of questions over the same state is
// one request, which is both faster and gentler on the single-in-flight rule.
type Question struct {
	Type         QuestionType `json:"type"`
	Instructions string       `json:"instructions"`
	// Criteria is a map of id to description for choice, an ordered list for score, and
	// absent for noul. Descriptions that do not overlap are what raise Confidence.
	Criteria any `json:"criteria,omitempty"`
}

// NoulQuestion asks a yes/no question.
func NoulQuestion(instructions string) Question {
	return Question{Type: Noul, Instructions: instructions}
}

// ChoiceQuestion asks to pick one of the criteria ids. The descriptions matter: they are
// what tells the options apart.
func ChoiceQuestion(instructions string, criteria map[string]string) Question {
	return Question{Type: Choice, Instructions: instructions, Criteria: criteria}
}

// ScoreQuestion asks for a level on an ordered scale, from lowest to highest.
func ScoreQuestion(instructions string, levels []string) Question {
	return Question{Type: Score, Instructions: instructions, Criteria: levels}
}

// SystemOneRequest is one decision request: a state and the questions about it.
type SystemOneRequest struct {
	// Model is the decision model. It is always clef because the constitution says so;
	// this package does not enforce that list, the caller does (I13).
	Model string `json:"model"`
	// State is what gets evaluated: a ticket, a thread, a part of a profile.
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// SystemOneResponse is the structured decision the service returns.
type SystemOneResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// Answer is one answer. Which fields carry the answer depends on Type; the rest stay zero.
//
// Two facts from the measured responses (§17.6) shape how this is read by callers: for
// choice and noul, Confidence is exactly the probability of the winner and adds nothing;
// for score, Score is a weighted mean and its decimals fake a precision the model does not
// have, which is why the UI shows levels and never the decimal.
type Answer struct {
	Type QuestionType `json:"type"`
	// Noul is the probability that the answer is "yes", for Noul.
	Noul float64 `json:"noul,omitempty"`
	// Choice is the winning criteria id, for Choice.
	Choice string `json:"choice,omitempty"`
	// Score is the probability-weighted level, for Score.
	Score float64 `json:"score,omitempty"`
	// Legend maps each level index, as a string, to its label, for Score.
	Legend map[string]string `json:"legend,omitempty"`
	// Probabilities holds one probability per criteria id or level index. Levels come
	// keyed as strings, not numbers.
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
}

// Usage is the token consumption the service reports. Decision requests answer with zero
// output tokens.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// SystemOne asks Clef a batch of questions about one state.
func (c *Client) SystemOne(ctx context.Context, req SystemOneRequest) (*SystemOneResponse, error) {
	switch {
	case req.Model == "":
		return nil, &Error{
			Op:   "systemone",
			Kind: KindRequest,
			Err:  errors.New("model is required: this package never picks one (I13)"),
		}
	case req.State == "":
		return nil, &Error{Op: "systemone", Kind: KindRequest, Err: errors.New("state is required")}
	case len(req.Questions) == 0:
		return nil, &Error{Op: "systemone", Kind: KindRequest, Err: errors.New("questions is empty")}
	case len(req.Questions) > MaxQuestions:
		return nil, &Error{
			Op:   "systemone",
			Kind: KindRequest,
			Err:  fmt.Errorf("%d questions, the API takes at most %d", len(req.Questions), MaxQuestions),
		}
	}

	var out SystemOneResponse
	if err := c.post(ctx, "systemone", c.systemOneURL, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
