package vocab

// ClarificationQuestion is one question from VOCAB.yaml's
// clarification_questions banks: the question text and what answering
// it decides.
type ClarificationQuestion struct {
	Question string
	Decides  string
}

// InsufficientInfoQuestions returns the insufficient_info bank
// entries in VOCAB.yaml order (char-exact).
func InsufficientInfoQuestions() []ClarificationQuestion {
	return []ClarificationQuestion{
		{
			Question: "Does <X> have an identity that survives attribute changes, or are two <X> with the same values interchangeable?",
			Decides:  "entity vs value_object",
		},
		{
			Question: "What must never be violated about <X>, even for an instant?",
			Decides:  "invariants and owner",
		},
		{
			Question: "Does this hold at all times, or only when a named operation occurs?",
			Decides:  "invariant vs assertion",
		},
		{
			Question: "Do experts pass this predicate around as a thing they name, rather than as a rule that simply holds?",
			Decides:  "specifications[] vs invariant or assertion",
		},
		{
			Question: "Is this a property of the value itself (constructor) or of the cluster that must stay consistent on every command?",
			Decides:  "value integrity vs cluster invariant",
		},
		{
			Question: "Would you say this to an expert who never saw the language?",
			Decides:  "domain contract vs programming-only guard",
		},
		{
			Question: "When <X> changes, what else must change in the same transaction?",
			Decides:  "aggregate boundary",
		},
		{
			Question: "Does the business care about <X>'s lifecycle (created, changed, closed)?",
			Decides:  "entity vs value_object vs domain_event",
		},
		{
			Question: "Which people/teams use the term <X>, and do they mean the same thing?",
			Decides:  "bounded_context assignment",
		},
	}
}

// ConflictQuestions returns the two conflict bank entries in
// VOCAB.yaml order (char-exact).
func ConflictQuestions() []ClarificationQuestion {
	return []ClarificationQuestion{
		{
			Question: "<X> is already defined as <def> in context <C>: (a) same concept, (b) different concept needing its own context, or (c) a correction?",
			Decides:  "reuse vs split vs update",
		},
		{
			Question: "This change breaks invariant <rule> owned by <owner>. Is the invariant wrong, or is this a different concept?",
			Decides:  "revision vs new term",
		},
	}
}

// ClarificationPolicy is the single-question policy line from VOCAB.yaml.
const ClarificationPolicy = "Ask the single question that unblocks; batch only when several concepts block at once; never ask what the input already answers."
