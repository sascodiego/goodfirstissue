package triage

import (
	"strings"
	"testing"
)

// The two dossiers are the whole point of this file: one must be unable to carry a
// label, the other must carry it as a claim, and neither may present repository text
// as anything but evidence.

func sampleIssue() Issue {
	return Issue{
		Repository: "acme/widget",
		Number:     42,
		Author:     "reporter",
		Title:      "Bug: crash when the config file is empty",
		Body:       "Running `widget run` panics with an empty config file.",
		Comments: []Comment{
			{Author: "passerby", Body: "same here", AuthorAssociation: "NONE"},
			{Author: "maintainer", Body: "Reproduced on 3.4.1.", AuthorAssociation: "MEMBER"},
		},
		Labels:            []string{"kind/secret-claim", "good-first-issue"},
		GitHubState:       "OPEN",
		GitHubStateReason: "",
	}
}

// A label is a claim, and a claim in the substance dossier contaminates the kind it is
// supposed to help judge (§7.2.1). So it cannot be there at all.
func TestSubstanceDossierCannotCarryLabels(t *testing.T) {
	got := SubstanceDossier(sampleIssue())
	for _, label := range sampleIssue().Labels {
		if strings.Contains(got, label) {
			t.Errorf("substance dossier leaks the label %q:\n%s", label, got)
		}
	}
}

// The kind the author declared is the same claim in another form, so the normalised
// title travels and the declaration does not.
func TestSubstanceDossierCarriesNormalisedTitleOnly(t *testing.T) {
	got := SubstanceDossier(sampleIssue())
	if !strings.Contains(got, "crash when the config file is empty") {
		t.Errorf("substance dossier lost the title:\n%s", got)
	}
	if strings.Contains(got, "Bug:") {
		t.Errorf("substance dossier kept the declared prefix:\n%s", got)
	}
}

// Intent is the one question that legitimately reads the labels, and it must read them
// verbatim: normalising them would be inventing a vocabulary no repository promised.
func TestIntentDossierCarriesLabelsVerbatim(t *testing.T) {
	got := IntentDossier(sampleIssue())
	for _, label := range sampleIssue().Labels {
		if !strings.Contains(got, label) {
			t.Errorf("intent dossier lost the label %q:\n%s", label, got)
		}
	}
}

// A label handed to the judge without that warning invites it to treat an unverified
// claim as a fact.
func TestIntentDossierMarksLabelsAsAClaim(t *testing.T) {
	got := strings.ToLower(IntentDossier(sampleIssue()))
	if !strings.Contains(got, "claim") || !strings.Contains(got, "unknown") {
		t.Errorf("intent dossier does not mark labels as an unknown author's claim:\n%s", got)
	}
}

// Repository text is evidence, never instruction (I3). The delimiter has to be in the
// dossier the judge reads, not only in the prompt.
func TestDossiersDelimitThirdPartyText(t *testing.T) {
	for name, dossier := range map[string]string{
		"substance": SubstanceDossier(sampleIssue()),
		"intent":    IntentDossier(sampleIssue()),
	} {
		lower := strings.ToLower(dossier)
		if !strings.Contains(lower, "not instructions") {
			t.Errorf("%s dossier does not mark its comment block as evidence:\n%s", name, dossier)
		}
		if !strings.Contains(dossier, "Reproduced on 3.4.1.") {
			t.Errorf("%s dossier dropped a maintainer comment:\n%s", name, dossier)
		}
		if strings.Contains(dossier, "same here") {
			t.Errorf("%s dossier kept a comment from a passer-by:\n%s", name, dossier)
		}
	}
}

// The state has a token ceiling, so the body is cut once, visibly, rather than silently.
func TestDossierTruncatesTheBody(t *testing.T) {
	issue := sampleIssue()
	issue.Body = strings.Repeat("word ", 5000)
	got := SubstanceDossier(issue)
	if len(got) > maxDossierChars {
		t.Errorf("dossier is %d characters, over the %d ceiling", len(got), maxDossierChars)
	}
	if !strings.Contains(got, "[truncated]") {
		t.Errorf("dossier truncated the body without saying so")
	}
}

// Layer 1 and layer 2 facts decide on their own, before any question is asked.
func TestPrescreenResolvesNativeFacts(t *testing.T) {
	openPR := []LinkedPR{{Number: 7, State: "OPEN"}}
	mergedPR := []LinkedPR{{Number: 7, State: "MERGED", Merged: true}}

	tests := []struct {
		name       string
		state      string
		reason     string
		prs        []LinkedPR
		wantSkip   bool
		wantReason string
	}{
		{"open with nothing else", "OPEN", "", nil, false, ""},
		{"closed as completed", "CLOSED", "COMPLETED", mergedPR, true, ReasonAlreadyDone},
		{"closed as not planned", "CLOSED", "NOT_PLANNED", nil, true, ReasonDeclined},
		{"closed for another reason", "CLOSED", "REOPENED", nil, true, ReasonClosed},
		{"open with an open pull request", "OPEN", "", openPR, true, ReasonAlreadyWorking},
		{"lowercase native values", "closed", "not_planned", nil, true, ReasonDeclined},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := sampleIssue()
			issue.GitHubState = tt.state
			issue.GitHubStateReason = tt.reason
			issue.LinkedPRs = tt.prs

			got := Prescreen(issue)
			if got.Skip != tt.wantSkip || got.Reason != tt.wantReason {
				t.Errorf("Prescreen = {%v %q}, want {%v %q}", got.Skip, got.Reason, tt.wantSkip, tt.wantReason)
			}
		})
	}
}

// The framing the author chose is the measured contaminant, so the prefix is stripped
// and reported separately as a claim.
func TestNormalizeTitleStripsDeclaredPrefixes(t *testing.T) {
	tests := []struct {
		title        string
		wantTitle    string
		wantDeclared string
	}{
		{"Bug: crash on start", "crash on start", "bug"},
		{"[BUG] crash on start", "crash on start", "bug"},
		{"feature: add yaml output", "add yaml output", "feature"},
		{"FEAT: add yaml output", "add yaml output", "feature"},
		{"[FEATURE] add yaml output", "add yaml output", "feature"},
		{"enhancement: add yaml output", "add yaml output", "feature"},
		{"question: how do I bind an IP", "how do I bind an IP", "question"},
		{"help: how do I bind an IP", "how do I bind an IP", "question"},
		{"proposal: make it configurable", "make it configurable", "feature"},
		{"RFC: make it configurable", "make it configurable", "feature"},
		{"Bug: [FEATURE] two prefixes", "two prefixes", "bug"},
		{"Nushell completion", "Nushell completion", ""},
		{"Bugs are not features", "Bugs are not features", ""},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			title, declared := NormalizeTitle(tt.title)
			if title != tt.wantTitle || declared != tt.wantDeclared {
				t.Errorf("NormalizeTitle = {%q %q}, want {%q %q}", title, declared, tt.wantTitle, tt.wantDeclared)
			}
		})
	}
}
