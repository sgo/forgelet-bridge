package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// idlerCheckCommand is this repository's idler check: the tool a forge's own
// copy is a deployment of.
const idlerCheckCommand = "role_health.sh"

// idlerCheckRuns runs the check against one project, the way the forge runs it.
func idlerCheckRuns(_ context.Context, world any, captures []string) error {
	project, forge := captures[1], captures[2]
	return runIdlerCheck(world.(*World), forge, project)
}

// idlerCheckRaisesTheAlerts runs the check the way the forge's schedule does
// when it wants the operator told: one pass that raises the alert for a stall.
func idlerCheckRaisesTheAlerts(_ context.Context, world any, captures []string) error {
	project, forge := captures[1], captures[2]
	return runIdlerCheck(world.(*World), forge, project, "--notify")
}

// runIdlerCheck runs this repository's own idler check against one project of a
// forge root, with the flags the scenario asks for, and remembers what it said.
func runIdlerCheck(w *World, forgeRoot, project string, args ...string) error {
	projectDir, err := w.pathOf(forgeRoot, project)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx,
		filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", idlerCheckCommand),
		append([]string{projectDir}, args...)...)
	command.Dir = projectDir
	command.Env = append(os.Environ(), "ROLE_HEALTH_CLAUDE_PROJECTS="+w.claudeProjects())
	out, err := command.CombinedOutput()
	w.idlerOutput = string(out)
	w.idlerErr = err
	return nil
}

// theForgeHoldsTheAlertTheIdlerRaised checks the alert reached the forge's own
// dashboard queue, naming the role and the card it is about.
func theForgeHoldsTheAlertTheIdlerRaised(_ context.Context, world any, captures []string) error {
	_, body, err := world.(*World).idlersAlert("Stall watch: "+captures[1], captures[2])
	if err != nil {
		return err
	}
	if !strings.Contains(body, "Stall watch") {
		return fmt.Errorf("the alert in the dashboard's queue does not read as a stall watch: %s", body)
	}
	return nil
}

// theDashboardTypedTheIdlersAlert checks the alert arrived the way every other
// request does: the dashboard woke the master pane as it wrote the request
// down, so the alert has two chances to be seen rather than one.
func theDashboardTypedTheIdlersAlert(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	root, body, err := w.idlersAlert()
	if err != nil {
		return err
	}
	dashboard, err := w.dashboardOf(filepath.Base(root))
	if err != nil {
		return err
	}
	woke, err := dashboard.WokeWith(body)
	if err != nil {
		return err
	}
	if !woke {
		return fmt.Errorf("the dashboard did not type the alert it took into the master role's pane: %s", body)
	}
	return nil
}

// idlersAlert is the alert a stall left in the forge's dashboard queue: the
// pending request that reads as a stall watch, and the forge whose queue holds
// it. What the alert is about narrows it further when a scenario names that.
func (w *World) idlersAlert(about ...string) (root, body string, err error) {
	root, err = w.theForgeRoot()
	if err != nil {
		return "", "", err
	}
	store := w.dashboards[filepath.Base(root)]
	if store == nil {
		return "", "", fmt.Errorf("the fixture forge root %s has no dashboard queue", root)
	}
	pending, err := store.Pending()
	if err != nil {
		return "", "", err
	}
	for _, request := range pending {
		if !strings.Contains(request.Body, "Stall watch") {
			continue
		}
		matched := true
		for _, want := range about {
			if !strings.Contains(request.Body, want) {
				matched = false
			}
		}
		if matched {
			return root, request.Body, nil
		}
	}
	return "", "", fmt.Errorf("the forge root %s holds no stall alert about %s:\n%s",
		root, strings.Join(about, " "), idlerBodies(pending))
}

// idlerBodies is what the queue holds, for a failure that says what was there.
func idlerBodies(requests []dashboard.Request) string {
	if len(requests) == 0 {
		return "(nothing pending)"
	}
	var bodies []string
	for _, request := range requests {
		bodies = append(bodies, request.Body)
	}
	return strings.Join(bodies, "\n")
}

// idlerCheckReports checks what the check said about one role.
func idlerCheckReports(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	role, phrase := captures[1], captures[2]
	verdict, card, err := idlerVerdict(phrase)
	if err != nil {
		return err
	}
	line, err := idlerLine(w, role)
	if err != nil {
		return err
	}
	columns := strings.Fields(line)
	if len(columns) < 2 || columns[1] != verdict {
		return fmt.Errorf("the idler check reports the role %s as %q, want %s:\n%s", role, columns[1], verdict, w.idlerOutput)
	}
	if card != "" && !strings.Contains(line, card) {
		return fmt.Errorf("the idler check's line for %s does not name the card %s:\n%s", role, card, line)
	}
	return nil
}

// idlerCheckNamesTool checks the check judged the role against the tool its own
// row records.
func idlerCheckNamesTool(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	tool, role := captures[1], captures[2]
	line, err := idlerLine(w, role)
	if err != nil {
		return err
	}
	if !strings.Contains(line, "tool="+tool) {
		return fmt.Errorf("the idler check does not name the tool %s for the role %s:\n%s", tool, role, line)
	}
	return nil
}

// idlerCheckStatus checks the check's exit status: it fails when any role is
// stalled and passes when none is.
func idlerCheckStatus(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.idlerOutput == "" {
		return fmt.Errorf("the idler check has not run")
	}
	switch captures[1] {
	case "fails":
		if w.idlerErr == nil {
			return fmt.Errorf("the idler check reported no stall:\n%s", w.idlerOutput)
		}
	case "passes":
		if w.idlerErr != nil {
			return fmt.Errorf("the idler check failed with no stall to report:\n%s", w.idlerOutput)
		}
	default:
		return fmt.Errorf("the step does not know the wording %q", captures[1])
	}
	return nil
}

// idlerCheckReportsForgeNotRunning checks the check says a forge with no role
// sessions is not running, rather than filing every role as a dead agent.
func idlerCheckReportsForgeNotRunning(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	line, err := idlerLine(w, "forge")
	if err != nil {
		return err
	}
	if columns := strings.Fields(line); len(columns) < 2 || columns[1] != "not-running" {
		return fmt.Errorf("the idler check does not report the forge as not running:\n%s", w.idlerOutput)
	}
	return nil
}

// idlerCheckReportsTheNoteMissing checks the check says the card's note went
// missing, which is its own verdict: the board says a role holds work and
// nothing anywhere exists to hand it over.
func idlerCheckReportsTheNoteMissing(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	card := captures[1]
	for _, line := range strings.Split(strings.TrimRight(w.idlerOutput, "\n"), "\n") {
		columns := strings.Fields(line)
		if len(columns) >= 3 && columns[1] == "note-missing" && strings.Contains(columns[2], card) {
			return nil
		}
	}
	return fmt.Errorf("the idler check does not report the note for the card %s as missing:\n%s", card, w.idlerOutput)
}

// idlerCheckDoesNotReportAnAgentThatHasGone checks the check did not judge a
// role whose pane runs a tool the agent is inside as an agent that has gone: a
// tool's foreground is the agent at work, not an agent that stopped.
func idlerCheckDoesNotReportAnAgentThatHasGone(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	line, err := idlerLine(w, captures[1])
	if err != nil {
		return err
	}
	if columns := strings.Fields(line); len(columns) >= 2 && columns[1] == "agent-gone" {
		return fmt.Errorf("the idler check reports the role %s as an agent that has gone:\n%s", captures[1], w.idlerOutput)
	}
	return nil
}

// theForgeHoldsTheAlertTheIdlerRaisedAboutAnAgentThatHasGone checks the alert a
// dead agent raised reached the forge's dashboard queue, naming the role and the
// reading. There is no card in it: an agent that has gone is a stall whether or
// not the role holds one.
func theForgeHoldsTheAlertTheIdlerRaisedAboutAnAgentThatHasGone(_ context.Context, world any, captures []string) error {
	_, body, err := world.(*World).idlersAlert("Stall watch: "+captures[1], "agent-gone")
	if err != nil {
		return err
	}
	if !strings.Contains(body, "Stall watch") {
		return fmt.Errorf("the alert in the dashboard's queue does not read as a stall watch: %s", body)
	}
	return nil
}

// idlerLine is the check's report line for one role.
func idlerLine(w *World, role string) (string, error) {
	for _, line := range strings.Split(strings.TrimRight(w.idlerOutput, "\n"), "\n") {
		if columns := strings.Fields(line); len(columns) > 0 && columns[0] == role {
			return line, nil
		}
	}
	return "", fmt.Errorf("the idler check reports nothing for the role %s:\n%s", role, w.idlerOutput)
}

// idlerVerdict turns the wording a scenario uses into the verdict the check
// prints, and the card that wording names when it names one.
func idlerVerdict(phrase string) (string, string, error) {
	switch {
	case strings.HasPrefix(phrase, "idle holding the card "):
		return "idle-holding-card", strings.TrimPrefix(phrase, "idle holding the card "), nil
	case phrase == "idle with nothing to pick up":
		return "idle-nothing-to-pick-up", "", nil
	case phrase == "idle with nothing assigned":
		return "idle-nothing-assigned", "", nil
	case phrase == "working":
		return "working", "", nil
	case phrase == "running a tool it does not know":
		return "tool-not-known", "", nil
	case phrase == "waiting on a decision":
		return "waiting-on-a-decision", "", nil
	case phrase == "waiting on the operator":
		return "waiting-on-the-operator", "", nil
	case phrase == "an agent that has gone":
		return "agent-gone", "", nil
	case phrase == "a session that has gone":
		return "session-gone", "", nil
	}
	return "", "", fmt.Errorf("the step does not know the wording %q", phrase)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-06T08:59:23+02:00","module_hash":"0d8644473f397e50ff1150a067dca96f6bc97bc59c81e675c3c2069146546b20","functions":[{"id":"func/idlerCheckRuns","name":"idlerCheckRuns","line":20,"end_line":23,"hash":"4c7db9b3dfe69bf272c5fd30f22a618f27f212ff207adb0d63b88af43a4c403a"},{"id":"func/idlerCheckRaisesTheAlerts","name":"idlerCheckRaisesTheAlerts","line":27,"end_line":30,"hash":"84af9af4577df80e5b3e05a4acf2634b9993cc238a82b1be015eb4b726cec513"},{"id":"func/runIdlerCheck","name":"runIdlerCheck","line":34,"end_line":50,"hash":"d82607f370f44888d9650dfa3abb698bca7b9c4a05b28bc9c222cebf76f0e887"},{"id":"func/theForgeHoldsTheAlertTheIdlerRaised","name":"theForgeHoldsTheAlertTheIdlerRaised","line":54,"end_line":63,"hash":"300748d3705644a6f6e780cfba94dc46a970d061487d48e8b7b6eba8e2ee100e"},{"id":"func/theDashboardTypedTheIdlersAlert","name":"theDashboardTypedTheIdlersAlert","line":68,"end_line":86,"hash":"6d55b6b7082de47f5431e6c5f2cfcb0399256853ce1ea4d154f11c954ed4d5e9"},{"id":"func/World.idlersAlert","name":"World.idlersAlert","line":91,"end_line":120,"hash":"0007972a1e0ee2774d20b61762965a7006a357127f65be66e946b18afcea558c"},{"id":"func/idlerBodies","name":"idlerBodies","line":123,"end_line":132,"hash":"a2a5070f1813f2eb934416eadfe58f22aeb8b8d6eb536787c10ab1adb925c816"},{"id":"func/idlerCheckReports","name":"idlerCheckReports","line":135,"end_line":154,"hash":"b6476c0f3e7b745d3b7f57031dadcd34927d7046c5545eb9112164c41a23dc0b"},{"id":"func/idlerCheckNamesTool","name":"idlerCheckNamesTool","line":158,"end_line":169,"hash":"92e9809dcd9cbad42f40c12f538eeba36a628f7506f165051b381bcedee1325c"},{"id":"func/idlerCheckStatus","name":"idlerCheckStatus","line":173,"end_line":191,"hash":"b9601a90fc798b5ac72ba8b839ae0a5f942055c44d30e2ecfd725ecd05937ff5"},{"id":"func/idlerCheckReportsForgeNotRunning","name":"idlerCheckReportsForgeNotRunning","line":195,"end_line":205,"hash":"66279159cb433a412a8c8ac6c1a3c3a847c0cdf9c82f8c0b7830855d1986606b"},{"id":"func/idlerCheckReportsTheNoteMissing","name":"idlerCheckReportsTheNoteMissing","line":210,"end_line":220,"hash":"7405c0f08066e6c35fb45e4588fa4965d9e0e9649e1ff516344604e400bf4801"},{"id":"func/idlerCheckDoesNotReportAnAgentThatHasGone","name":"idlerCheckDoesNotReportAnAgentThatHasGone","line":225,"end_line":235,"hash":"43c7d4f9918145424b00382c7b143dfe3d6dd09561781bb02a6389151af5ae86"},{"id":"func/theForgeHoldsTheAlertTheIdlerRaisedAboutAnAgentThatHasGone","name":"theForgeHoldsTheAlertTheIdlerRaisedAboutAnAgentThatHasGone","line":241,"end_line":250,"hash":"158043d1edce250c04c1b4bd54e2bbb443887041fb3129e51bd6b96c0267e400"},{"id":"func/idlerLine","name":"idlerLine","line":253,"end_line":260,"hash":"b0ca68834cd45d49948466a1f1c727148da73142aba84bfdd42f60676966278a"},{"id":"func/idlerVerdict","name":"idlerVerdict","line":264,"end_line":286,"hash":"c41e8d32308ed94f2727f1f59bba993643b6447316efb00001e530870bc9c12b"}]}
// mutate4go-manifest-end
