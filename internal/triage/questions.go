package triage

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/sascodiego/nan-hackathon-clef/internal/nan"
)

// Question ids. They are wire keys, not prose: the answer map is keyed by them, the policy
// switches on them, and the report names them. Renaming one is a contract change.
const (
	QKind               = "kind"
	QSelfContained      = "self_contained"
	QHasEvidence        = "has_evidence"
	QUnambiguous        = "unambiguous"
	QDoneDefined        = "done_defined"
	QReportsDefect      = "reports_defect"
	QEvidenceSufficient = "evidence_sufficient"

	// QOnlyForRequester and QFitsDirection belong to the deferred alignment node (§7.5).
	QOnlyForRequester = "only_for_requester"
	QFitsDirection    = "fits_direction"
)

// The categories of the kind question. The policy switches on these exact strings, so they
// are constants rather than literals scattered through the chain.
const (
	KindBug      = "bug"
	KindFeature  = "feature"
	KindQuestion = "question"
	KindSpam     = "spam"
	KindOther    = "other"
)

// kindCriteria is what tells the categories apart. The descriptions travel with the question
// because the service answers better when the options are distinguishable.
var kindCriteria = map[string]string{
	KindBug:      "It reports behaviour that already exists and says that behaviour is wrong or broken.",
	KindFeature:  "It asks for behaviour that does not exist yet.",
	KindQuestion: "It asks for information, an explanation, or how to do something, and does not ask for a change.",
	KindSpam:     "It contains no real request and no usable information.",
	KindOther:    "It describes work that fits none of the above, such as documentation, tests, or project process.",
}

// The wording below is the frozen contract of prompt.md, verbatim: it is the wording that was
// measured against the labelled bank, and it is the only copy in the program. The benchmark
// asks these and the product asks these, because there is no second set to diverge from.
var substanceQuestions = map[string]nan.Question{
	QKind: nan.ChoiceQuestion("What is this issue asking for?", kindCriteria),
	QSelfContained: nan.NoulQuestion("Does this text state what is being asked clearly enough that " +
		"someone who has not spoken to the author could start, without material that is not here: " +
		"a recording, a screenshot, or a private conversation?"),
	QHasEvidence: nan.NoulQuestion("Does the issue contain the material someone would need to " +
		"reproduce or verify the claim: steps, a code sample, a log, a version, an environment, " +
		"or a concrete use case?"),
	QUnambiguous: nan.NoulQuestion("Does the expected result admit exactly one reading, so that " +
		"two people would agree on what a correct outcome looks like?"),
	QDoneDefined: nan.NoulQuestion("Does the issue define what would make it finished, so that a " +
		"maintainer could accept or reject the result without another conversation?"),
	QReportsDefect: nan.NoulQuestion("Does the text report behaviour that already exists as " +
		"defective, rather than asking for behaviour that does not exist yet?"),
	QEvidenceSufficient: nan.NoulQuestion("Is the text enough to decide what kind of request " +
		"this is, regardless of how much work the request would be or how thin or unwelcome it looks?"),
}

// AlignmentQuestions is the deferred node of §7.5: a feature can be clear, approved as a
// topic, and still be worth nothing to anyone but the person asking. It is kept beside the
// frozen set, not inside it, because P1 does not send it.
var alignmentQuestions = map[string]nan.Question{
	QOnlyForRequester: nan.NoulQuestion("Is this request driven by a situation specific to the " +
		"person asking, such that the project's other users would not get anything out of it?"),
	QFitsDirection: nan.NoulQuestion("Judged against what the project's documentation and recent " +
		"history show it is trying to be, would this request move it further in that direction, " +
		"or away from it?"),
}

// Questions is what P1 asks, in one request. The map is a fresh copy on every call, so a
// caller cannot rewrite the frozen contract for the rest of the process.
func Questions() map[string]nan.Question {
	return cloneQuestions(substanceQuestions)
}

// AlignmentQuestions is the deferred node's set. Nothing in P1 calls it: it exists so the
// node, when it arrives, inherits the measured wording instead of a retyped one.
func AlignmentQuestions() map[string]nan.Question {
	return cloneQuestions(alignmentQuestions)
}

func cloneQuestions(src map[string]nan.Question) map[string]nan.Question {
	out := make(map[string]nan.Question, len(src))
	for id, q := range src {
		out[id] = q
	}
	return out
}

// Version fingerprints the contract and the mechanism. It is part of the cache key, so a
// verdict produced by another prompt misses instead of being reused as fresh (I14).
func Version() string {
	sum := sha256.Sum256([]byte(Prompt + "\x00" + Criteria))
	return hex.EncodeToString(sum[:])[:12]
}
