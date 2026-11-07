package relay

import "testing"

func TestForgetProjectScopedKeepsSiblingAndForgeWork(t *testing.T) {
	var state State
	state.EnsureMaps()
	state.Threads["target-request"] = "$target"
	state.Replied["target-request"] = "$target-reply"
	state.Relayed["$target-message"] = "target-request"
	state.Pending["$target-pending"] = "target"
	state.PendingThreads["$target-pending"] = "$target-thread"
	state.ScopeProject(ScopeThreads, "target-request", "/forges/one", "target")
	state.ScopeProject(ScopeReplied, "target-request", "/forges/one", "target")
	state.ScopeProject(ScopeRelayed, "$target-message", "/forges/one", "target")
	state.ScopeProject(ScopePending, "$target-pending", "/forges/one", "target")
	state.ScopeProject(ScopePendingThreads, "$target-pending", "/forges/one", "target")

	state.Threads["sibling-request"] = "$sibling"
	state.Relayed["$sibling-message"] = "sibling-request"
	state.ScopeProject(ScopeThreads, "sibling-request", "/forges/one", "sibling")
	state.ScopeProject(ScopeRelayed, "$sibling-message", "/forges/one", "sibling")

	state.Threads["forge-request"] = "$forge"
	state.Scope(ScopeThreads, "forge-request", "/forges/one")

	state.ForgetProjectScoped("/forges/one", "target")

	for section, key := range map[string]string{
		ScopeThreads:        "target-request",
		ScopeReplied:        "target-request",
		ScopeRelayed:        "$target-message",
		ScopePending:        "$target-pending",
		ScopePendingThreads: "$target-pending",
	} {
		if state.ProjectScopeOwner(section, key) != "" {
			t.Errorf("target scope %s/%s remains", section, key)
		}
	}
	if _, ok := state.Threads["target-request"]; ok {
		t.Error("target thread remains")
	}
	if _, ok := state.Replied["target-request"]; ok {
		t.Error("target reply remains")
	}
	if _, ok := state.Relayed["$target-message"]; ok {
		t.Error("target relay pair remains")
	}
	if _, ok := state.Pending["$target-pending"]; ok {
		t.Error("target pending message remains")
	}
	if _, ok := state.PendingThreads["$target-pending"]; ok {
		t.Error("target pending thread remains")
	}

	if state.ProjectScopeOwner(ScopeThreads, "sibling-request") != "sibling" ||
		state.Threads["sibling-request"] != "$sibling" ||
		state.Relayed["$sibling-message"] != "sibling-request" {
		t.Errorf("sibling relay state was changed: %+v", state)
	}
	if state.Threads["forge-request"] != "$forge" ||
		state.ScopeOwner(ScopeThreads, "forge-request") != "/forges/one" {
		t.Errorf("forge-level relay state was changed: %+v", state)
	}
}

func TestForgetScopedAlsoDropsNarrowProjectOwnership(t *testing.T) {
	var state State
	state.EnsureMaps()
	state.Threads["request"] = "$message"
	state.ScopeProject(ScopeThreads, "request", "/forges/one", "project")
	state.Threads["kept"] = "$kept"
	state.Scope(ScopeThreads, "kept", "/forges/other")
	state.ForgetScoped("/forges/one")

	if _, ok := state.Threads["request"]; ok {
		t.Error("forge removal retained a project-scoped thread")
	}
	if len(state.ProjectScopes) != 0 ||
		state.ScopeOwner(ScopeThreads, "request") != "" ||
		state.ScopeOwner(ScopeThreads, "kept") != "/forges/other" {
		t.Fatalf("owners = scopes:%v project_scopes:%v", state.Scopes, state.ProjectScopes)
	}
	if state.Threads["kept"] != "$kept" {
		t.Error("forge removal cleared another forge's thread")
	}
}

func TestScopeQueriesAndUnscopingRemoveBothOwners(t *testing.T) {
	var state State
	state.EnsureMaps()
	state.ScopeProject(ScopeThreads, "request", "/forges/one", "project")
	if !state.InScope(ScopeThreads, "request", "/forges/one") ||
		state.InScope(ScopeThreads, "request", "/forges/two") {
		t.Fatal("forge scope query did not use the exact owner")
	}
	state.Unscope(ScopeThreads, "request")
	if state.ScopeOwner(ScopeThreads, "request") != "" ||
		state.ProjectScopeOwner(ScopeThreads, "request") != "" {
		t.Fatal("Unscope left a forge or project owner")
	}
}

func TestDeleteScopedIgnoresAnInvalidKey(t *testing.T) {
	var state State
	state.EnsureMaps()
	state.deleteScoped("invalid")
	if len(state.Scopes) != 0 || len(state.ProjectScopes) != 0 {
		t.Fatalf("invalid scoped key changed state: %+v", state)
	}
}

func TestRequestActionsIgnoresAnEmptyRequest(t *testing.T) {
	if actions := requestActions(State{}, Request{Body: "  "}, map[string]string{}); len(actions) != 0 {
		t.Fatalf("empty request actions = %+v", actions)
	}
}
