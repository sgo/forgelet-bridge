package matrix

import (
	"testing"

	"maunium.net/go/mautrix/event"
)

func TestMatchingProjectFactsSelectsOnlyTheRequestedProjectFacts(t *testing.T) {
	state := map[event.Type]map[string]*event.Event{
		{Type: "com.forgelet.board", Class: event.StateEventType}: {
			"target": {
				Content: event.Content{Raw: map[string]interface{}{"project": "target"}},
			},
			"sibling": {
				Content: event.Content{Raw: map[string]interface{}{"project": "sibling"}},
			},
		},
		{Type: "com.forgelet.approval", Class: event.StateEventType}: {
			"target/approval": {
				Content: event.Content{Raw: map[string]interface{}{"project": "target"}},
			},
		},
		{Type: "m.room.name", Class: event.StateEventType}: {
			"": {},
		},
	}

	facts := matchingProjectFacts(state, func(eventType event.Type, stateKey string, evt *event.Event) bool {
		return eventType.Type == "com.forgelet.board" && stateKey == "target" ||
			eventType.Type == "com.forgelet.approval" && factProject(evt) == "target"
	})
	if len(facts) != 2 {
		t.Fatalf("facts = %+v, want the board and approval only", facts)
	}
	if facts[0].eventType.Type != "com.forgelet.approval" ||
		facts[0].stateKey != "target/approval" ||
		facts[1].eventType.Type != "com.forgelet.board" ||
		facts[1].stateKey != "target" {
		t.Fatalf("facts = %+v, want sorted target facts", facts)
	}
}

func TestFactProjectRejectsMissingAndMalformedContent(t *testing.T) {
	if got := factProject(nil); got != "" {
		t.Fatalf("nil fact project = %q", got)
	}
	if got := factProject(&event.Event{}); got != "" {
		t.Fatalf("missing project = %q", got)
	}
	if got := decodedProject(map[string]interface{}{"project": 1}); got != "" {
		t.Fatalf("malformed project = %q", got)
	}
	if got := decodedProject(map[string]interface{}{"project": "target"}); got != "target" {
		t.Fatalf("project = %q, want target", got)
	}
}

func TestProjectFactPredicatesNameOnlyTheTargetProject(t *testing.T) {
	board := event.Type{Type: "com.forgelet.board", Class: event.StateEventType}
	approval := event.Type{Type: "com.forgelet.approval", Class: event.StateEventType}
	target := &event.Event{Content: event.Content{Raw: map[string]interface{}{"project": "target"}}}
	sibling := &event.Event{Content: event.Content{Raw: map[string]interface{}{"project": "sibling"}}}

	if !allProjectFacts(board, "anything", target) {
		t.Fatal("the project room did not match every forgelet fact")
	}
	if !isProjectBoard("target", board, "target", target) ||
		isProjectBoard("target", board, "sibling", sibling) ||
		isProjectBoard("target", approval, "target", target) {
		t.Fatal("the activity board predicate did not select just the target")
	}
	if !isProjectApproval("target", approval, "target/approval", target) ||
		isProjectApproval("target", approval, "sibling/approval", sibling) ||
		isProjectApproval("target", board, "target/approval", target) {
		t.Fatal("the approval predicate did not select just the target")
	}
}
