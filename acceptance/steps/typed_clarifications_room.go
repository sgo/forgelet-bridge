package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

func operatorRepliesToTypedClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return oneUserRepliesToClarification(ctx, w, w.operatorID, captures[1], captures[2])
}

func bridgeStateHasNoTypedClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	loaded, err := state.Load(filepath.Join(w.stateDir, "bridge-state.json"))
	if err != nil {
		return err
	}
	for key, clarification := range loaded.Relay.Clarifications {
		if clarification.Source == relay.ClarificationSourceTyped &&
			clarification.ForgeRoot != "" && strings.Contains(key, "/"+captures[1]+"/") {
			return fmt.Errorf("bridge state still holds typed clarification %s for %s", key, captures[1])
		}
	}
	return nil
}

func fakeDashboardRecordedTypedAnswer(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	requests := fake.ChatRequests()
	for _, request := range requests {
		if request.ClarificationID == captures[3] && request.Text == captures[2] {
			return nil
		}
	}
	return fmt.Errorf("fake dashboard %s chat requests = %+v, want the typed answer %q for %s",
		captures[1], requests, captures[2], captures[3])
}

func clarificationsRoomCarriesBothTypedProjects(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.clarificationsRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, message := range operator.Messages(roomID) {
		for _, project := range []string{"project-alpha", "project-beta"} {
			if strings.HasPrefix(message.Body, clarificationMessageStart(project)) {
				seen[project] = true
			}
		}
	}
	if !seen["project-alpha"] || !seen["project-beta"] {
		return fmt.Errorf("typed clarification notices cover %v, want both project-alpha and project-beta", seen)
	}
	return nil
}

func typedClarificationMessageCarriesNoTranscript(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	_, _, body, err := w.clarificationMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	for _, forbidden := range []string{"UNRELATED TRANSCRIPT", "secret-marker", "/Users/operator/private"} {
		if strings.Contains(body, forbidden) {
			return fmt.Errorf("the typed clarification notice copied unrelated content %q:\n%s", forbidden, body)
		}
	}
	if !strings.Contains(body, "which safe stage?") {
		return fmt.Errorf("the typed clarification notice omitted its bounded excerpt:\n%s", body)
	}
	return nil
}

func clarificationsRoomShowsNoUnreadTyped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	bridgeState, err := state.Load(filepath.Join(w.stateDir, "bridge-state.json"))
	if err != nil {
		return err
	}
	for key, clarification := range bridgeState.Relay.Clarifications {
		if clarification.Source == relay.ClarificationSourceTyped && strings.Contains(key, "/"+captures[1]+"/") {
			return fmt.Errorf("typed clarification %s is still unread for %s", key, captures[1])
		}
	}
	return nil
}
