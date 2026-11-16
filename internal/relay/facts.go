package relay

// The event types a forge's rooms carry the waiting facts as: one event per
// item, the item's own id as its state key, in the room the item belongs to.
// The content is the same facts the dashboard serves, in the words the shared
// reading names, so a phone and a desk cannot name the same card differently.
const (
	// BoardFactType carries one project's board: its name, its lanes, and the
	// cards each lane holds.
	BoardFactType = "com.forgelet.board"
	// ApprovalFactType carries one approval the forge is holding.
	ApprovalFactType = "com.forgelet.approval"
	// ClarificationFactType carries one question an agent is blocked on.
	ClarificationFactType = "com.forgelet.clarification"
)

// Fact is one waiting fact as a room carries it: the event type, the item's own
// key, and the content a reader finds the facts in.
type Fact struct {
	Type    string
	Key     string
	Content map[string]any
}

// BoardFact is the state one project's board is carried as: the project, the
// lanes it runs, and the cards each lane holds. Its key is the project, so one
// project's board is one event and a project that stops being open is cleared.
func BoardFact(project string, lanes []string, cards []Card) Fact {
	tasks := make([]map[string]any, 0, len(cards))
	for _, card := range cards {
		tasks = append(tasks, map[string]any{"name": card.Name, "lane": card.Lane})
	}
	return Fact{
		Type: BoardFactType,
		Key:  project,
		Content: map[string]any{
			"projects": []map[string]any{{
				"name":  project,
				"lanes": stringsOrEmpty(lanes),
				"tasks": tasks,
			}},
		},
	}
}

// ApprovalFact is the state one approval is carried as: the four things a
// decision about it needs, and what changed. Its key is the approval's own id.
func ApprovalFact(approval Approval) Fact {
	return Fact{
		Type: ApprovalFactType,
		Key:  approval.ID,
		Content: map[string]any{
			"id":        approval.ID,
			"project":   approval.Project,
			"task":      approval.Card,
			"gate":      approval.Gate,
			"artifacts": stringsOrEmpty(approval.Artifacts),
		},
	}
}

// ClarificationFact is the state one question is carried as: the project it
// belongs to, the blocked role, and the question itself. Its key is the
// question's own id.
func ClarificationFact(clarification Clarification) Fact {
	key := clarification.ID
	if clarification.Source == ClarificationSourceTyped {
		key = ClarificationSourceTyped + ":" + clarification.Project + ":" + clarification.ID
	}
	return Fact{
		Type: ClarificationFactType,
		Key:  key,
		Content: map[string]any{
			"id":        clarification.ID,
			"project":   clarification.Project,
			"role":      clarification.Role,
			"body":      clarification.Question,
			"status":    ClarificationWaiting,
			"source":    clarification.Source,
			"lifecycle": clarification.Lifecycle,
			"attention": clarification.Attention,
		},
	}
}

// ClarificationWaiting is the status an unanswered question carries, the word
// the shared reading knows: a question marked done is one the operator has
// already answered and is no longer waiting on.
const ClarificationWaiting = "pending"

// stringsOrEmpty keeps a list a valid JSON array rather than null, so two
// readings of the same empty facts are the same facts.
func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-16T18:55:10+01:00","module_hash":"fb6eeb16dade37c42f204d983fa4df7dc1a2d6f3e7fac4a38ac3ddbe84bb8575","functions":[{"id":"func/BoardFact","name":"BoardFact","line":28,"end_line":44,"hash":"89e6d1cb4a6e9905ebbeb69445d57af8531681eaf862d6583888fb58a8247cff"},{"id":"func/ApprovalFact","name":"ApprovalFact","line":48,"end_line":60,"hash":"185f163dc237c2c7bf6a9e231d7c79ab179db0c11620179f741b2f3a312cbc4d"},{"id":"func/ClarificationFact","name":"ClarificationFact","line":65,"end_line":84,"hash":"afc057eb037dfa5e0e4b1c425dde7fa6561353a4ada69992d6584acfbd75513d"},{"id":"func/stringsOrEmpty","name":"stringsOrEmpty","line":93,"end_line":98,"hash":"4ec545259295898336e2f26d886ad1212ea8ec5e674f3d86d6525bb5907ecb51"}]}
// mutate4go-manifest-end
