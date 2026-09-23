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
		{`^the bridge's status names the forge (\S+) as the only unhappy one$`, statusNamesTheOnlyUnhappyForge},
		{`^the bridge's status names no unhappy forge$`, statusNamesNoUnhappyForge},
		{`^the forge (\S+)'s dashboard is stopped$`, dashboardStopped},
		{`^the forge (\S+)'s dashboard is started again$`, dashboardStartedAgain},

		{`^the bridge configuration of the forge root (\S+) names the forge root (\S+)$`, configureForAdapter},
		{`^the bridge configuration of the forge root (\S+) names the forge root (\S+) as (.+)$`, bridgeConfigNamesForge},
		{`^the bridge configuration of the forge root (\S+) holds only the forge root (\S+)$`, bridgeConfigHoldsOnlyForge},
		{`^the adapter for the forge root (\S+) has started the bridge$`, adapterStartedBridge},
		{`^the adapter for the forge root (\S+) adds the forge root (\S+) as (.+)$`, adapterAddsForge},
		{`^the adapter for the forge root (\S+) is asked to add the forge root (\S+) as (.+)$`, adapterAddsForge},
		{`^the adapter's output names the forge (.+)$`, adapterOutputNamesForge},
		{`^the adapter's output names the copy of the bridge configuration it kept$`, adapterOutputNamesTheCopy},
		{`^the adapter kept a copy of the bridge configuration it replaced$`, adapterKeptACopy},
		{`^the adapter refused the new forge$`, adapterRefused},
		{`^the bridge's status names every configured forge as reached$`, statusNamesEveryConfiguredForgeAsReached},
		{`^the bridge's status names the forge (\S+) as one it reached$`, statusNamesTheForgeAsReached},
		{`^the bridge's status names the forge (\S+) as one it did not reach$`, statusNamesTheForgeAsUnreached},

		{`^the fixture forge root (\S+) holds the project (\S+)$`, forgeHoldsProject},
		{`^the fixture forge root (\S+) carries a lieutenant prompt that asks only about fleet-wide actions$`, forgePromptAsksLess},
		{`^the route gate has a proposal for the card (\S+) in the project (\S+)$`, gateHasProposal},
		{`^the operator answers the proposal for the card (\S+) with "(.+)"$`, operatorAnswersProposal},
		{`^the operator answers the proposal for the card (\S+) with the same words again$`, operatorAnswersAgain},
		{`^the forge's board has no card (\S+) for the project (\S+) yet$`, boardHasNoCard},
		{`^the forge's board now holds the card (\S+) for the project (\S+)$`, boardNowHoldsCard},
		{`^the forge's board holds exactly one card (\S+)$`, boardHoldsExactlyOneCard},
		{`^the gate still has the proposal for the card (\S+) waiting$`, gateStillHasProposal},
		{`^the gate refused the second card$`, gateRefusedTheSecondCard},
		{`^the fixture forge root (\S+) carries a constitution the forge wrote itself$`, forgeWroteItsOwnConstitution},
		{`^the fixture forge root (\S+) scaffolds new projects from the pack (\S+)$`, forgeScaffoldsFromAPack},
		{`^the project (\S+) of the fixture forge root (\S+) is mid-card$`, projectIsMidCard},
		{`^the adapter for the forge root (\S+) installs the rules$`, adapterInstallsRules},
		{`^the adapter for the forge root (\S+) installs the rules again$`, adapterInstallsRules},
		{`^the installer's output says it changed the (\S+) rule$`, installerChangedRule},
		{`^the installer's output says the (\S+) rule was already current$`, installerAlreadyCurrent},
		{`^the installer's output says it left the projects of the forge root (\S+) alone$`, installerLeftProjectsAlone},
		{`^the installer changed nothing in the forge root (\S+)$`, installerChangedNothing},
		{`^the constitution of the forge root (\S+) carries the (\S+) rule$`, constitutionCarriesRule},
		{`^the packs of the forge root (\S+) carry the (\S+) rule$`, packsCarryRule},
		{`^the forge root (\S+) keeps the wording it wrote itself$`, forgeKeepsItsWording},
		{`^the project (\S+) of the forge root (\S+) is the tree it was$`, projectIsTheTreeItWas},
		{`^the (\S+) rule in the constitution of the forge root (\S+) has gone stale$`, ruleHasGoneStale},
		{`^the acceptance suite remembers the worktree it runs from$`, rememberLane},
		{`^the worktree the acceptance suite runs from still holds its head, its branches and its changes$`, laneHeld},
		{`^the fixture forge holds the snapshot it took of the card (\S+)$`, fixtureHoldsSnapshot},
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
		{`^the forge space (.+?) holds the approvals room (.+)$`, spaceHoldsNamedRoom},
		{`^the operator is invited to the approvals room (.+)$`, invitedToNamedRoom},
		{`^the approvals room (.+) is encrypted$`, approvalsRoomEncrypted},
		{`^the matrix client (\S+) has joined the approvals room$`, userJoinedApprovalsRoom},
		{`^the forge space (.+?) holds the activity room (.+)$`, spaceHoldsNamedRoom},
		{`^the operator is invited to the activity room (.+)$`, invitedToNamedRoom},
		{`^the activity room (.+) is encrypted$`, approvalsRoomEncrypted},
		{`^the forge space (.+?) holds the clarifications room (.+)$`, spaceHoldsNamedRoom},
		{`^the operator is invited to the clarifications room (.+)$`, invitedToNamedRoom},
		{`^the clarifications room (.+) is encrypted$`, clarificationsRoomEncrypted},
		{`^the matrix client (\S+) has joined the clarifications room$`, userJoinedClarificationsRoom},

		{`^the forge's dashboard already holds the chat request "(.+)"$`, dashboardHoldsRequest},
		{`^the forge (\S+)'s dashboard already holds the chat request "(.+)"$`, namedDashboardHoldsRequest},
		{`^the forge's dashboard takes the chat request "(.+)" the way its clients give it$`, dashboardTakesRequestStep},
		{`^the forge's dashboard typed the chat request reading "(.+)" into the lieutenant's pane$`, dashboardWoke},
		{`^the forge's dashboard already holds the pending approval for the card (\S+) (.+)$`, pendingApproval},
		{`^the forge's dashboard already holds a pending approval for the card (\S+) it cannot send back$`, pendingApprovalThatCannotBeSentBack},
		{`^the forge (\S+)'s dashboard already holds a pending approval for the card (\S+) it cannot send back$`, forgeApprovalCannotBeSentBack},
		{`^the forge repairs the approval for the card (\S+)$`, forgeRepairsApproval},
		{`^the forge (\S+) repairs the approval for the card (\S+)$`, forgeRepairsItsApproval},
		{`^the approval message for the card (\S+) names the project (\S+), the gate "(.+)", and the changed files (\S+) and (\S+)$`, approvalMessageNames},
		{`^the operator taps ✅ on the approval message for the card (\S+)$`, approvalTapped},
		{`^(the operator|\S+) reacts (\S+) to the approval message for the card (\S+)$`, someoneReacts},
		{`^the matrix client (\S+) reacts (\S+) to the approval message for the card (\S+)$`, someoneReacts},
		{`^the bridge answers in the approvals room with the gestures it takes$`, gesturesAnswered},
		{`^the operator replies "(.+)" in the approval message's thread for the card (\S+)$`, operatorRepliesToApproval},
		{`^the operator replies "(.+)" in the approval message's thread for the card (\S+) in the forge (\S+)$`, operatorRepliesToForgeApproval},
		{`^the operator swipes a reply "(.+)" to the approval message for the card (\S+)$`, operatorSwipesReplyToApproval},
		{`^the operator sends the message "(.+)" into the approvals room$`, operatorTalksInApprovalsRoom},
		{`^the forge's dashboard recorded the approval for the card (\S+) as approved$`, approvalApproved},
		{`^the forge's dashboard recorded the approval for the card (\S+) as sent back with (?:exactly )?"(.+)"$`, approvalSentBack},
		{`^the forge's dashboard recorded exactly one resolution for the card (\S+)$`, resolutionCount},
		{`^the approval for the card (\S+) is still pending in the forge$`, approvalStillPending},
		{`^the forge's dashboard was never asked to delete or tear down$`, forgeNeverDestroyed},
		{`^the operator approves the approval for the card (\S+) from the desktop$`, desktopApproved},
		{`^the operator decrypts the approval reply "(.+)" to the approval message for the card (\S+)$`, approvalReplyDecrypted},
		{`^the operator decrypts the approval reply "(.+)" to the approval message for the card (\S+) in the forge (\S+)$`, operatorDecryptsForgeApprovalReply},
		{`^the approval message's thread holds exactly one reply$`, approvalThreadOneReply},
		{`^the approval message for the card (\S+) tells the operator a reply sends it back$`, approvalSaysWhatAReplyMeans},

		{`^the forge's dashboard already holds the pending clarification from the role (\S+) in the project (\S+) asking "(.+)"$`, pendingClarification},
		{`^the clarification message for the project (\S+) names the project (\S+), the blocked role (\S+), and the question "(.+)"$`, clarificationMessageNames},
		{`^the clarifications room holds exactly (\d+) clarification messages?$`, clarificationsRoomCount},
		{`^the operator replies "(.+)" in the clarification message's thread for the project (\S+)$`, operatorRepliesToClarification},
		{`^the operator swipes a reply "(.+)" to the clarification message for the project (\S+)$`, operatorSwipesReplyToClarification},
		{`^the matrix client (\S+) replies "(.+)" in the clarification message's thread for the project (\S+)$`, someoneRepliesToClarification},
		{`^the operator sends the message "(.+)" into the clarifications room$`, operatorTalksInClarificationsRoom},
		{`^the forge's dashboard recorded the clarification for the project (\S+) as answered with exactly "(.+)"$`, clarificationAnswered},
		{`^the blocked role (\S+) is woken with the answer "(.+)"$`, blockedRoleWoken},
		{`^the operator decrypts the clarification reply "(.+)" in the clarification message for the project (\S+)$`, clarificationReplyDecrypted},
		{`^the clarification message's thread holds exactly one reply$`, clarificationThreadOneReply},
		{`^the clarification message for the project (\S+) tells the operator a reply is the answer$`, clarificationSaysWhatAReplyMeans},
		{`^the clarification for the project (\S+) is still pending in the forge$`, clarificationStillWaiting},
		{`^the operator answers the clarification for the project (\S+) from the desktop with "(.+)"$`, operatorAnsweredFromTheDesktop},
		{`^the forge's board already holds the card (\S+) in the project (\S+) in the lane (\S+)$`, boardHoldsCard},
		{`^the forge (\S+)'s board already holds the card (\S+) in the project (\S+) in the lane (\S+)$`, forgeBoardHoldsCard},
		{`^the forge moves the card (\S+) to the lane (\S+)$`, forgeMovesCard},
		{`^the forge finishes the card (\S+)$`, forgeFinishesCard},
		{`^the operator decrypts a card update saying the card (\S+) appeared in the project (\S+) in the lane (\S+)$`, cardAppearedSeen},
		{`^the operator decrypts a card update saying the card (\S+) moved on in the project (\S+) to the lane (\S+)$`, cardMovedSeen},
		{`^the operator decrypts a card update saying the card (\S+) finished in the project (\S+)$`, cardFinishedSeen},
		{`^the operator decrypts a card update in the forge (\S+) saying the card (\S+) appeared in the project (\S+) in the lane (\S+)$`, forgeCardAppearedSeen},
		{`^the activity room holds exactly (\d+) card updates?$`, cardUpdateCount},
		{`^the approvals room holds no messages$`, approvalsRoomSilent},
		{`^the operator sends the message "(.+)" into the activity room$`, operatorTalksInActivityRoom},
		{`^a quiet stretch passes with nothing changing$`, quietStretch},
		{`^the lieutenant answers the chat request "(.+)" with "(.+)"$`, lieutenantAnswers},
		{`^the operator decrypts the chat message "(.+)"$`, operatorDecryptsChatMessage},
		{`^the operator decrypts the chat message "(.+)" in the forge (.+)'s chat room sent under the name (.+)$`, forgeSentChatMessage},
		{`^the bridge sent the chat message "(.+)" encrypted$`, bridgeSentEncrypted},
		{`^the operator decrypts the thread reply "(.+)" to the chat message "(.+)"$`, operatorDecryptsThreadReply},
		{`^the operator swipes a reply "(.+)" to the chat message "(.+)"$`, operatorSwipesReplyToChatMessage},
		{`^the operator decrypts the thread reply "(.+)" in the forge (.+)'s chat room sent under the name (.+) to the chat message "(.+)"$`, forgeSentThreadReply},
		{`^the operator sends the message "(.+)" into chat room (.+)$`, operatorSendsMessage},
		{`^the operator sends the message "(.+)" into the forge (\S+)'s chat room$`, operatorSendsIntoForgeChatRoom},
		{`^(\S+) sends the message "(.+)" into chat room (.+)$`, userSendsMessage},
		{`^the matrix client (\S+) has joined chat room (.+)$`, userJoinedChatRoom},
		{`^the forge holds exactly one chat request reading "(.+)"$`, forgeHoldsOneRequest},
		{`^the forge holds a chat request reading "(.+)"$`, forgeHoldsRequest},
		{`^the forge holds the chat request "(.+)" the dashboard took and typed into the lieutenant's pane$`, forgeHoldsRequestTheDashboardTook},
		{`^the forge (\S+) holds the chat request "(.+)" the dashboard took and typed into the lieutenant's pane$`, forgeHoldsTheRequestTheDashboardTook},
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
