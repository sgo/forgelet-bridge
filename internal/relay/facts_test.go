package relay

import "testing"

func TestBoardFactCarriesTheBoardsWordsUnderItsOwnKey(t *testing.T) {
	fact := BoardFact("forgelet-bridge", []string{"specifier", "coder"},
		[]Card{{Name: "card-a", Lane: "coder"}, {Name: "card-b", Lane: "done"}})

	if fact.Type != BoardFactType || fact.Key != "forgelet-bridge" {
		t.Fatalf("fact = %+v, want the board's own type and key", fact)
	}
	projects, ok := fact.Content["projects"].([]map[string]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("projects = %v, want one project", fact.Content["projects"])
	}
	project := projects[0]
	if project["name"] != "forgelet-bridge" {
		t.Errorf("name = %v, want the project", project["name"])
	}
	lanes, _ := project["lanes"].([]string)
	if len(lanes) != 2 || lanes[0] != "specifier" || lanes[1] != "coder" {
		t.Errorf("lanes = %v, want the project's lanes", project["lanes"])
	}
	tasks, _ := project["tasks"].([]map[string]any)
	if len(tasks) != 2 || tasks[0]["name"] != "card-a" || tasks[0]["lane"] != "coder" {
		t.Errorf("tasks = %v, want the cards in their lanes", project["tasks"])
	}
}

func TestBoardFactKeepsAnEmptyBoardAnArray(t *testing.T) {
	fact := BoardFact("forgelet-bridge", nil, nil)

	project := fact.Content["projects"].([]map[string]any)[0]
	if _, ok := project["lanes"].([]string); !ok {
		t.Errorf("lanes = %v, want an array rather than null", project["lanes"])
	}
	if _, ok := project["tasks"].([]map[string]any); !ok {
		t.Errorf("tasks = %v, want an array rather than null", project["tasks"])
	}
}

func TestApprovalFactNamesWhatADecisionNeedsUnderItsOwnId(t *testing.T) {
	fact := ApprovalFact(approval())

	if fact.Type != ApprovalFactType || fact.Key != "approval-1" {
		t.Fatalf("fact = %+v, want the approval's own type and key", fact)
	}
	for field, want := range map[string]string{
		"id": "approval-1", "project": "forgelet-bridge", "task": card, "gate": "coder → refactorer",
	} {
		if got := fact.Content[field]; got != want {
			t.Errorf("%s = %v, want %q", field, got, want)
		}
	}
	if artifacts, _ := fact.Content["artifacts"].([]string); len(artifacts) != 2 {
		t.Errorf("artifacts = %v, want what changed", fact.Content["artifacts"])
	}
}

func TestClarificationFactAsksTheQuestionUnderItsOwnId(t *testing.T) {
	fact := ClarificationFact(clarification())

	if fact.Type != ClarificationFactType || fact.Key != "clar-1" {
		t.Fatalf("fact = %+v, want the question's own type and key", fact)
	}
	for field, want := range map[string]string{
		"id": "clar-1", "project": "forgelet-bridge", "role": "coder",
		"body": "which lane should the refund card start in?", "status": ClarificationWaiting,
	} {
		if got := fact.Content[field]; got != want {
			t.Errorf("%s = %v, want %q", field, got, want)
		}
	}
}
