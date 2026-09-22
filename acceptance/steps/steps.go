package steps

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/acceptance/runtime"
)

// register wires every step of the acceptance features to a handler. Patterns
// capture the values that vary between scenarios; the wording that means
// different things gets its own handler.
func register(registry *runtime.Registry) error {
	steps := []struct {
		pattern string
		handler runtime.Handler
	}{
		{`^the fixture forge roots? (.+) (?:has|have) (?:its|their) dashboards? running$`, fixturesRunning},
		{`^the bridge is configured with the forge root (.+?) and the operator (\S+)$`, configuredForgeAndOperator},
		{`^the bridge is configured with the forge root (\S+) named (.+)$`, namedForge},
		{`^the bridge is configured with the operator (\S+)$`, configuredOperator},
		{`^the bridge is configured with the forge roots? (.+)$`, configuredForges},
		{`^the bridge is started$`, bridgeStarted},
		{`^the bridge is stopped and started again$`, bridgeRestarted},
		{`^the bridge has caught up with the forge$`, bridgeCaughtUp},
		{`^the bridge has published its Matrix device to the operator$`, bridgePublishedDevice},
		{`^the operator sees the same bridge Matrix device$`, operatorSeesSameDevice},
		{`^the bridge has created the forge space (.+?) and its chat room (.+)$`, bridgeCreatedSpace},

		{`^the operator sees the forge space (.+)$`, operatorSeesSpace},
		{`^the operator sees exactly one forge space named (.+)$`, operatorSeesOneSpace},
		{`^the operator sees no forge space named (.+)$`, noForgeSpace},
		{`^the operator sees (\d+) forge spaces$`, operatorSeesSpaces},
		{`^the operator is invited to the forge space (.+)$`, operatorInvitedToSpace},
		{`^the operator is invited to chat room (.+)$`, operatorInvitedToRoom},
		{`^the forge space (.+?) holds the chat room (.+)$`, spaceHoldsRoom},
		{`^the forge space (.+?) holds exactly one chat room (.+)$`, spaceHoldsOneRoom},
		{`^chat room (.+) is encrypted$`, chatRoomEncrypted},
		{`^the forge space (.+?) holds the approvals room (.+)$`, spaceHoldsApprovalsRoom},
		{`^the operator is invited to the approvals room (.+)$`, invitedToApprovalsRoom},
		{`^the approvals room (.+) is encrypted$`, approvalsRoomEncrypted},
		{`^the matrix client (\S+) has joined the approvals room$`, userJoinedApprovalsRoom},

		{`^the forge's dashboard already holds the chat request "(.+)"$`, dashboardHoldsRequest},
		{`^the forge (\S+)'s dashboard already holds the chat request "(.+)"$`, namedDashboardHoldsRequest},
		{`^the forge's dashboard already holds the pending approval for the card (\S+) (.+)$`, pendingApproval},
		{`^the approval message for the card (\S+) names the project (\S+), the gate "(.+)", and the changed files (\S+) and (\S+)$`, approvalMessageNames},
		{`^the operator taps ✅ on the approval message for the card (\S+)$`, approvalTapped},
		{`^(the operator|\S+) reacts (\S+) to the approval message for the card (\S+)$`, someoneReacts},
		{`^the operator replies "(.+)" in the approval message's thread for the card (\S+)$`, operatorRepliesToApproval},
		{`^the operator sends the message "(.+)" into the approvals room$`, operatorTalksInApprovalsRoom},
		{`^the forge recorded the approval for the card (\S+) as approved$`, approvalApproved},
		{`^the forge recorded the approval for the card (\S+) as sent back with "(.+)"$`, approvalSentBack},
		{`^the forge recorded exactly one resolution for the card (\S+)$`, resolutionCount},
		{`^the approval for the card (\S+) is still pending in the forge$`, approvalStillPending},
		{`^the forge was never asked to delete or tear down$`, forgeNeverDestroyed},
		{`^the operator approves the approval for the card (\S+) from the desktop$`, desktopApproved},
		{`^the operator decrypts the approval reply "(.+)" to the approval message for the card (\S+)$`, approvalReplyDecrypted},
		{`^the approval message's thread holds exactly one reply$`, approvalThreadOneReply},
		{`^the lieutenant answers the chat request "(.+)" with "(.+)"$`, lieutenantAnswers},
		{`^the operator decrypts the chat message "(.+)"$`, operatorDecryptsChatMessage},
		{`^the operator decrypts the chat message "(.+)" in the forge (.+)'s chat room sent under the name (.+)$`, forgeSentChatMessage},
		{`^the bridge sent the chat message "(.+)" encrypted$`, bridgeSentEncrypted},
		{`^the operator decrypts the thread reply "(.+)" to the chat message "(.+)"$`, operatorDecryptsThreadReply},
		{`^the operator decrypts the thread reply "(.+)" in the forge (.+)'s chat room sent under the name (.+) to the chat message "(.+)"$`, forgeSentThreadReply},
		{`^the operator sends the message "(.+)" into chat room (.+)$`, operatorSendsMessage},
		{`^(\S+) sends the message "(.+)" into chat room (.+)$`, userSendsMessage},
		{`^the matrix client (\S+) has joined chat room (.+)$`, userJoinedChatRoom},
		{`^the forge holds exactly one chat request reading "(.+)"$`, forgeHoldsOneRequest},
		{`^the forge holds a chat request reading "(.+)"$`, forgeHoldsRequest},
		{`^the forge holds (\d+) chat requests$`, forgeHoldsRequests},
		{`^the lieutenant is woken with the chat request reading "(.+)"$`, lieutenantWoken},
		{`^chat room (.+?) holds exactly one chat message reading "(.+)"$`, roomHoldsOneChatMessage},
		{`^chat room (.+?) holds exactly one thread reply reading "(.+)"$`, roomHoldsOneThreadReply},
	}

	for _, step := range steps {
		if err := registry.Step(step.pattern, step.handler); err != nil {
			return err
		}
	}
	return nil
}

func singleForge(w *World) (string, error) {
	if len(w.configured) != 1 {
		return "", fmt.Errorf("the bridge is configured with %d forge roots, want exactly one here", len(w.configured))
	}
	return w.configured[0], nil
}

func captured(captures []string, index int) (string, error) {
	if len(captures) <= index {
		return "", fmt.Errorf("step is missing capture %d", index)
	}
	return captures[index], nil
}

func stepContext() (context.Context, context.CancelFunc) {
	return contextWithTimeout()
}
