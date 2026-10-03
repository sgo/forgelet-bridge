package steps

import (
	"context"
	"fmt"
	"slices"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// The approvals and clarifications rooms report an outcome the same way: the
// bridge marks the message that carried the question and leaves the thread
// quiet. The checks the two rooms share live here, so neither drifts from the
// other.

// bridgeReactions are the reaction keys the bridge put on a message.
func bridgeReactions(operator *fixtures.User, roomID, messageID, sender string) []string {
	var keys []string
	for _, reaction := range operator.Reactions(roomID, messageID) {
		if reaction.Sender == sender {
			keys = append(keys, reaction.Key)
		}
	}
	return keys
}

// roomCarriesBridgeMark waits for the bridge's reaction mark on a message,
// naming the message for the timeout.
func roomCarriesBridgeMark(ctx context.Context, w *World, roomID, messageID, mark, described string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("the bridge never marked %s with %s", described, mark), func() (bool, error) {
		return slices.Contains(bridgeReactions(operator, roomID, messageID, w.bridgeUserID), mark), nil
	})
}

// roomCarriesNoBridgeReaction checks the bridge left a message unmarked, once
// the room has settled.
func roomCarriesNoBridgeReaction(ctx context.Context, w *World, roomID, messageID, described string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if marks := bridgeReactions(operator, roomID, messageID, w.bridgeUserID); len(marks) != 0 {
		return fmt.Errorf("%s carries the bridge's reactions %v, want none", described, marks)
	}
	return nil
}
