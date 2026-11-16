package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

const (
	// AutomationProjection is the bounded dashboard projection the bridge
	// consumes.
	AutomationProjection = "automation-v1"

	maxAutomationResponseBytes  = 512 * 1024
	maxAutomationApprovals      = 200
	maxAutomationClarifications = 200
	maxAutomationProjects       = 64
	maxAutomationCards          = 200
	maxAutomationLanes          = 64
	maxAutomationActivity       = 64
	maxAutomationIdentities     = 512
	maxAutomationQuestion       = 500
)

var automationResolutionValues = map[string]bool{
	relay.ResolutionApproved:              true,
	relay.ResolutionSentBack:              true,
	relay.ResolutionAutomaticSpecApproval: true,
	relay.ResolutionAutoApproved:          true,
}

// AutomationState is the bounded projection shared by every automation
// consumer in one bridge tick.
type AutomationState struct {
	Projection        string                                         `json:"projection"`
	Identity          *AutomationIdentity                            `json:"identity"`
	Approvals         *AutomationSection[AutomationApproval]         `json:"approvals"`
	ResolvedApprovals *AutomationSection[AutomationResolvedApproval] `json:"resolved_approvals"`
	Clarifications    *AutomationClarifications                      `json:"clarifications"`
	Projects          *AutomationSection[AutomationProject]          `json:"projects"`
}

// AutomationIdentity names the Family or project dashboard that answered.
type AutomationIdentity struct {
	Kind         string                     `json:"kind"`
	Name         string                     `json:"name"`
	AllProjects  *AutomationSection[string] `json:"all_projects"`
	OpenProjects *AutomationSection[string] `json:"open_projects"`
}

// AutomationSection is one bounded collection in the projection.
type AutomationSection[T any] struct {
	Count     int  `json:"count"`
	Truncated bool `json:"truncated"`
	Items     []T  `json:"items"`
}

// AutomationApproval is one pending approval from the projection.
type AutomationApproval struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Card    string `json:"task"`
	TaskID  string `json:"task_id"`
	Gate    string `json:"gate"`
}

// AutomationResolvedApproval is one approval resolved away from the room.
type AutomationResolvedApproval struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Card       string `json:"task"`
	Resolution string `json:"resolution"`
}

// AutomationClarifications separates the legacy store from typed project-chat
// clarifications while retaining one validated projection.
type AutomationClarifications struct {
	Legacy *AutomationSection[AutomationClarification] `json:"legacy"`
	Typed  *AutomationSection[AutomationClarification] `json:"typed"`
}

// AutomationClarification is one clarification's safe lifecycle projection.
type AutomationClarification struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	Project         string  `json:"project"`
	Role            string  `json:"role"`
	ClarificationID string  `json:"clarification_id"`
	Attention       string  `json:"attention"`
	QuestionExcerpt *string `json:"question_excerpt"`
}

// AutomationProject carries the project identity, lanes, cards and bounded
// activity facts the bridge may consume.
type AutomationProject struct {
	Name     string                                 `json:"name"`
	Lanes    *AutomationSection[string]             `json:"lanes"`
	Cards    *AutomationSection[AutomationCard]     `json:"cards"`
	Activity *AutomationSection[AutomationActivity] `json:"activity"`
}

// AutomationCard is one card and the lane the projection places it in.
type AutomationCard struct {
	Name string `json:"name"`
	Lane string `json:"lane"`
}

// AutomationActivity is a bounded activity row. Its individual safe fields are
// not needed by the bridge and are intentionally not retained.
type AutomationActivity struct {
	Role  string `json:"role"`
	State string `json:"state"`
}

// UnsupportedProjectionError reports a dashboard that does not serve the
// required automation projection.
type UnsupportedProjectionError struct {
	Status int
}

func (e UnsupportedProjectionError) Error() string {
	return fmt.Sprintf("automation projection is unsupported (HTTP %d)", e.Status)
}

// AutomationState reads and validates the bounded projection once.
func (a *API) AutomationState(ctx context.Context) (AutomationState, error) {
	state, err := a.automationResponse(ctx)
	if err != nil {
		return AutomationState{}, err
	}
	if err := validateAutomationState(state); err != nil {
		return AutomationState{}, err
	}
	return state, nil
}

func (a *API) automationResponse(ctx context.Context) (AutomationState, error) {
	headers, status, data, err := a.exchange(ctx, "GET", "/api/automation-state", nil, maxAutomationResponseBytes)
	if unsupportedAutomationStatus(status) {
		return AutomationState{}, UnsupportedProjectionError{Status: status}
	}
	if err != nil {
		return AutomationState{}, fmt.Errorf("read automation state: %w", err)
	}
	if got := headers.Get("X-SwarmForge-Projection"); got != AutomationProjection {
		return AutomationState{}, fmt.Errorf("invalid automation projection header %q", got)
	}
	var state AutomationState
	if err := json.Unmarshal(data, &state); err != nil {
		return AutomationState{}, fmt.Errorf("decode automation projection: %w", err)
	}
	return state, nil
}

func unsupportedAutomationStatus(status int) bool {
	return status == 404 || status == 501
}

func validateAutomationState(state AutomationState) error {
	if state.Projection != AutomationProjection {
		return fmt.Errorf("invalid automation projection discriminator %q", state.Projection)
	}
	if err := validateAutomationSections(state); err != nil {
		return err
	}
	if err := validateApprovalRows(state.Approvals.Items, state.ResolvedApprovals.Items); err != nil {
		return err
	}
	return validateProjectRows(state.Projects.Items)
}

func validateAutomationSections(state AutomationState) error {
	if err := validateAutomationIdentity(state.Identity); err != nil {
		return err
	}
	if err := validateAutomationCollections(state); err != nil {
		return err
	}
	return validateClarificationSections(state.Clarifications)
}

func validateAutomationCollections(state AutomationState) error {
	if err := validateAutomationSection("approvals", state.Approvals, maxAutomationApprovals); err != nil {
		return err
	}
	if err := validateAutomationSection("resolved approvals", state.ResolvedApprovals, maxAutomationApprovals); err != nil {
		return err
	}
	if err := validateAutomationSection("projects", state.Projects, maxAutomationProjects); err != nil {
		return err
	}
	return nil
}

func validateAutomationIdentity(identity *AutomationIdentity) error {
	if !validAutomationIdentity(identity) {
		return fmt.Errorf("invalid automation projection identity")
	}
	if err := validateAutomationSection("all projects", identity.AllProjects, maxAutomationIdentities); err != nil {
		return err
	}
	return validateAutomationSection("open projects", identity.OpenProjects, maxAutomationProjects)
}

func validAutomationIdentity(identity *AutomationIdentity) bool {
	return identity != nil &&
		(identity.Kind == "family" || identity.Kind == "project") &&
		strings.TrimSpace(identity.Name) != ""
}

func validateAutomationSection[T any](name string, section *AutomationSection[T], limit int) error {
	if section == nil || section.Count < len(section.Items) || len(section.Items) > limit ||
		section.Truncated != (section.Count > len(section.Items)) {
		return fmt.Errorf("invalid automation projection section %s", name)
	}
	return nil
}

func validateClarificationSections(clarifications *AutomationClarifications) error {
	if clarifications == nil {
		return fmt.Errorf("missing automation projection clarifications")
	}
	if err := validateAutomationSection("legacy clarifications", clarifications.Legacy, maxAutomationClarifications); err != nil {
		return err
	}
	if err := validateAutomationSection("typed clarifications", clarifications.Typed, maxAutomationClarifications); err != nil {
		return err
	}
	return validateClarificationRows(clarifications)
}

func validateClarificationRows(clarifications *AutomationClarifications) error {
	if err := validateLegacyClarificationRows(clarifications.Legacy.Items); err != nil {
		return err
	}
	return validateTypedClarificationRows(clarifications.Typed.Items)
}

func validateLegacyClarificationRows(items []AutomationClarification) error {
	for _, item := range items {
		if item.Status == "pending" && !validLegacyClarification(item) {
			return fmt.Errorf("invalid pending legacy clarification in automation projection")
		}
	}
	return nil
}

func validateTypedClarificationRows(items []AutomationClarification) error {
	for _, item := range items {
		if item.Status == "open" && !validTypedClarification(item) {
			return fmt.Errorf("invalid open typed clarification in automation projection")
		}
	}
	return nil
}

func validLegacyClarification(item AutomationClarification) bool {
	return item.ID != "" && item.Role != "" && validQuestionExcerpt(item.QuestionExcerpt)
}

func validTypedClarification(item AutomationClarification) bool {
	return item.ClarificationID != "" && validQuestionExcerpt(item.QuestionExcerpt)
}

func validQuestionExcerpt(excerpt *string) bool {
	if excerpt == nil {
		return false
	}
	text := strings.TrimSpace(*excerpt)
	return text != "" && len([]rune(text)) <= maxAutomationQuestion
}

func validateApprovalRows(pending []AutomationApproval, resolved []AutomationResolvedApproval) error {
	if err := validatePendingApprovalRows(pending); err != nil {
		return err
	}
	return validateResolvedApprovalRows(resolved)
}

func validatePendingApprovalRows(pending []AutomationApproval) error {
	for _, approval := range pending {
		if approval.ID == "" || approval.Project == "" || approval.Card == "" {
			return fmt.Errorf("invalid pending approval in automation projection")
		}
	}
	return nil
}

func validateResolvedApprovalRows(resolved []AutomationResolvedApproval) error {
	for _, approval := range resolved {
		if approval.ID == "" || approval.Project == "" || !automationResolutionValues[approval.Resolution] {
			return fmt.Errorf("invalid resolved approval in automation projection")
		}
	}
	return nil
}

func validateProjectRows(projects []AutomationProject) error {
	for _, project := range projects {
		if err := validateProjectRow(project); err != nil {
			return err
		}
	}
	return nil
}

func validateProjectRow(project AutomationProject) error {
	if strings.TrimSpace(project.Name) == "" {
		return fmt.Errorf("invalid project in automation projection")
	}
	for _, check := range []func() error{
		func() error {
			return validateAutomationSection("project lanes for "+project.Name, project.Lanes, maxAutomationLanes)
		},
		func() error {
			return validateAutomationSection("project cards for "+project.Name, project.Cards, maxAutomationCards)
		},
		func() error {
			return validateAutomationSection("project activity for "+project.Name, project.Activity, maxAutomationActivity)
		},
	} {
		if err := check(); err != nil {
			return err
		}
	}
	return validateProjectCards(project)
}

func validateProjectCards(project AutomationProject) error {
	for _, card := range project.Cards.Items {
		if strings.TrimSpace(card.Name) == "" || strings.TrimSpace(card.Lane) == "" {
			return fmt.Errorf("invalid card in automation projection project %s", project.Name)
		}
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-16T15:16:17+01:00","module_hash":"f42bd2c8f1bf169d3980e43fa3a16bdbb32badc1f3d3d9339004afeb261d50c2","functions":[{"id":"func/UnsupportedProjectionError.Error","name":"UnsupportedProjectionError.Error","line":124,"end_line":126,"hash":"6a0ebfbc9411999c4830cdcba0b3820de6522a45007ba331ff2255b00a989b0b"},{"id":"func/API.AutomationState","name":"API.AutomationState","line":129,"end_line":138,"hash":"40f86cd80ad083feeeb2de4df354d6d9dbbf5f9a7cde89147f411c600a7a0f16"},{"id":"func/API.automationResponse","name":"API.automationResponse","line":140,"end_line":156,"hash":"716a334b178dc6f18d4a79e68e0e490ca20f9e2e4ad8ef3ccf8e91461b252aa0"},{"id":"func/unsupportedAutomationStatus","name":"unsupportedAutomationStatus","line":158,"end_line":160,"hash":"920cfbdd5914ef0bf93a105ed651236a5db5c716ae80f53d749b0c6015d106fa"},{"id":"func/validateAutomationState","name":"validateAutomationState","line":162,"end_line":173,"hash":"93b08c6086eb4e65d97f27f137cf1227a37225a6ac25ee31b07c245ff979d667"},{"id":"func/validateAutomationSections","name":"validateAutomationSections","line":175,"end_line":183,"hash":"4ea4a8c7e506bdf5b741ae6281e3dac0fb929078937f830da860b94cb436884f"},{"id":"func/validateAutomationCollections","name":"validateAutomationCollections","line":185,"end_line":196,"hash":"20ebb820c808d4aff09e16287208b03a800a31baa1c5a301b54db5687d756422"},{"id":"func/validateAutomationIdentity","name":"validateAutomationIdentity","line":198,"end_line":206,"hash":"e595abaa00107281cc74a4be5c4e01323731f2bf8faf9c93203712339b058db7"},{"id":"func/validAutomationIdentity","name":"validAutomationIdentity","line":208,"end_line":212,"hash":"b9261e73cd8dbe2aa2f16d841e33e4d4568055a6b2e67a4b343d57f9809442d9"},{"id":"func/validateAutomationSection","name":"validateAutomationSection","line":214,"end_line":220,"hash":"d193330bc0b6cb768ea440b4efdafc08e48dcc314ab4f8b7b9af673992f9cc2d"},{"id":"func/validateClarificationSections","name":"validateClarificationSections","line":222,"end_line":233,"hash":"7c8fa2242668639bb0148f317998c46b954b7809204a1fb3e7e70e17499f4835"},{"id":"func/validateClarificationRows","name":"validateClarificationRows","line":235,"end_line":240,"hash":"597dc32d2c82d9bd2a5c1a3a20f3a10d3f3866237d1b3f1c1049558ebad278ae"},{"id":"func/validateLegacyClarificationRows","name":"validateLegacyClarificationRows","line":242,"end_line":249,"hash":"e99344bd748c8e4b26887a614005b17e2c515eed0fba0cfdb917e76d785fea7d"},{"id":"func/validateTypedClarificationRows","name":"validateTypedClarificationRows","line":251,"end_line":258,"hash":"d28211ce72aed345072f95a8fe8e476508d71ff920cfaad2419993d3fef0bd54"},{"id":"func/validLegacyClarification","name":"validLegacyClarification","line":260,"end_line":262,"hash":"c267593eecfd5f6ebfcf8b4763737cdb96385e5ba6d4991fc020a0c2b8bb8a40"},{"id":"func/validTypedClarification","name":"validTypedClarification","line":264,"end_line":266,"hash":"cdd5e673baad19f82bab48599b1d773a87588b23362a4616e7f2f67f48016afa"},{"id":"func/validQuestionExcerpt","name":"validQuestionExcerpt","line":268,"end_line":274,"hash":"99b2955fba6414fb1ca031c46f7719137ecda01e5bff928705759e3ae0926ddc"},{"id":"func/validateApprovalRows","name":"validateApprovalRows","line":276,"end_line":281,"hash":"09987ef5e8cdd7d09c434a8233673d373319ca14f994b2143a01c120fd88a7a5"},{"id":"func/validatePendingApprovalRows","name":"validatePendingApprovalRows","line":283,"end_line":290,"hash":"9a86063e73582a5da547738721e7f289221eaa58d3a81c046931d767bbadaa70"},{"id":"func/validateResolvedApprovalRows","name":"validateResolvedApprovalRows","line":292,"end_line":299,"hash":"29e7f4cd1bbebe20072fbe1897b78f0b557c2bff788a345220f988004c8de92a"},{"id":"func/validateProjectRows","name":"validateProjectRows","line":301,"end_line":308,"hash":"102c607c44cd48ccbafe3a5c981c8362afddf7464f0a3a69c74b9500809baa0d"},{"id":"func/validateProjectRow","name":"validateProjectRow","line":310,"end_line":330,"hash":"59bbd67081a16df08fe031c38e55818a80e99c93ef17b87da75caf2a479e9133"},{"id":"func/validateProjectCards","name":"validateProjectCards","line":332,"end_line":339,"hash":"8773f3c63697ac8faf37102cccb60dc9c09631d4a167b3e1b8bc220ffee79360"}]}
// mutate4go-manifest-end
