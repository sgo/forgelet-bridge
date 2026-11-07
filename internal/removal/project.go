package removal

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// ProjectCatalog answers the filesystem questions project removal must settle
// before it touches Matrix: whether the configured root is a Forgelet forge and
// whether the named single-agent project is managed there.
type ProjectCatalog interface {
	ForgeExists(root string) (bool, error)
	ManagedProject(root, name string) (bool, error)
}

// ProjectCleaner is the Matrix boundary for removing one project room. It has
// no caller-supplied room id: the inspection supplies the id from bridge state.
type ProjectCleaner interface {
	ClearProjectFacts(ctx context.Context, projectRoomID, activityRoomID, approvalsRoomID, project string) error
	RemoveOperator(ctx context.Context, roomID, operator string) error
	LeaveAndForget(ctx context.Context, roomID string) error
}

// ProjectInspection is a validated project removal request. Its room id comes
// only from bridge state and is never accepted from the command line.
type ProjectInspection struct {
	Result      Result
	Root        string
	ProjectName string
	RoomID      string
	Forge       state.Forge
	Project     state.Project
}

// InspectProject validates a root, project name, managed directory and saved
// project-room state without changing Matrix or bridge state.
func InspectProject(cfg config.Config, loaded *state.State, root, name string, catalog ProjectCatalog) ProjectInspection {
	if result, ok := validateProjectRequest(cfg, root, name); !ok {
		return result
	}
	if result, ok := validateProjectCatalog(catalog, root, name); !ok {
		return result
	}
	return inspectProjectState(loaded, root, name)
}

func validateProjectRequest(cfg config.Config, root, name string) (ProjectInspection, bool) {
	if strings.TrimSpace(root) == "" || hasParentTraversal(root) || !filepath.IsAbs(root) {
		return refusedProject("unsafe_root"), false
	}
	if !configuredRoot(cfg, root) {
		return refusedProject("unknown_root"), false
	}
	if unsafeProjectName(name) {
		return refusedProject("unsafe_project"), false
	}
	return ProjectInspection{}, true
}

func validateProjectCatalog(catalog ProjectCatalog, root, name string) (ProjectInspection, bool) {
	if catalog == nil {
		return refusedProject("state_unavailable"), false
	}
	if result, ok := validateCatalogForge(catalog, root); !ok {
		return result, false
	}
	return validateCatalogProject(catalog, root, name)
}

func validateCatalogForge(catalog ProjectCatalog, root string) (ProjectInspection, bool) {
	forgeExists, err := catalog.ForgeExists(root)
	if err != nil {
		return refusedProject("state_unavailable"), false
	}
	if !forgeExists {
		return refusedProject("unknown_root"), false
	}
	return ProjectInspection{}, true
}

func validateCatalogProject(catalog ProjectCatalog, root, name string) (ProjectInspection, bool) {
	managed, err := catalog.ManagedProject(root, name)
	if err != nil {
		return refusedProject("state_unavailable"), false
	}
	if !managed {
		return refusedProject("unknown_project"), false
	}
	return ProjectInspection{}, true
}

func inspectProjectState(loaded *state.State, root, name string) ProjectInspection {
	forge, result := projectForgeForRemoval(loaded, root)
	if result.Status != "" {
		return ProjectInspection{Result: result}
	}
	project, result := projectRecordForRemoval(forge, name)
	if result.Status != "" {
		return ProjectInspection{Result: result}
	}
	return readyProjectInspection(root, name, forge, project)
}

func projectForgeForRemoval(loaded *state.State, root string) (state.Forge, Result) {
	if loaded == nil {
		return state.Forge{}, Result{Status: Refused, Reason: "incomplete_project_state"}
	}
	forge, ok := loaded.Forges[root]
	if !ok {
		return state.Forge{}, Result{Status: NothingToRemove}
	}
	if !completeForge(forge) {
		return state.Forge{}, Result{Status: Refused, Reason: "incomplete_project_state"}
	}
	return forge, Result{}
}

func projectRecordForRemoval(forge state.Forge, name string) (state.Project, Result) {
	project, ok := forge.Projects[name]
	if !ok {
		return state.Project{}, Result{Status: NothingToRemove}
	}
	if project.RoomID == "" || projectRoomCollides(forge, name, project.RoomID) {
		return state.Project{}, Result{Status: Refused, Reason: "incomplete_project_state"}
	}
	return project, Result{}
}

func readyProjectInspection(root, name string, forge state.Forge, project state.Project) ProjectInspection {
	return ProjectInspection{
		Result:      Result{Status: Ready},
		Root:        root,
		ProjectName: name,
		RoomID:      project.RoomID,
		Forge:       forge,
		Project:     project,
	}
}

func configuredRoot(cfg config.Config, root string) bool {
	for _, forge := range cfg.Forges {
		if forge.Root == root {
			return true
		}
	}
	return false
}

func unsafeProjectName(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || name == "." || name == ".." {
		return true
	}
	return projectNameShapeUnsafe(name)
}

func projectNameShapeUnsafe(name string) bool {
	return filepath.IsAbs(name) || strings.HasPrefix(name, "!") ||
		strings.ContainsAny(name, "/\\\x00") || filepath.Base(name) != name
}

func projectRoomCollides(forge state.Forge, name, roomID string) bool {
	for _, forgeRoomID := range []string{
		forge.SpaceID,
		forge.RoomID,
		forge.ApprovalsRoomID,
		forge.ActivityRoomID,
		forge.ClarificationsRoomID,
	} {
		if roomID == forgeRoomID {
			return true
		}
	}
	for otherName, project := range forge.Projects {
		if otherName != name && roomID == project.RoomID {
			return true
		}
	}
	return false
}

// RemoveProject carries out a validated project inspection. A retry reloads
// state, skips completed Matrix stages, and never selects a room by name.
func (s Service) RemoveProject(ctx context.Context, inspection ProjectInspection) Result {
	if inspection.Result.Status != Ready {
		return inspection.Result
	}
	fresh, result, ready := s.reloadProject(inspection.Root, inspection.ProjectName)
	if !ready {
		return result
	}
	if result := s.cleanProjectFacts(ctx, fresh); result.Status != "" {
		return result
	}
	if result := s.removeProjectOperator(ctx, fresh); result.Status != "" {
		return result
	}
	if result := s.leaveProject(ctx, fresh); result.Status != "" {
		return result
	}
	return s.finalizeProject(fresh.Root, fresh.ProjectName, fresh.RoomID)
}

func (s Service) cleanProjectFacts(ctx context.Context, inspection ProjectInspection) Result {
	progress, result := s.projectProgress(inspection)
	if result.Status != "" {
		return result
	}
	if progress.FactsCleared {
		return Result{}
	}
	if err := s.ProjectCleaner.ClearProjectFacts(
		ctx, inspection.RoomID, inspection.Forge.ActivityRoomID,
		inspection.Forge.ApprovalsRoomID, inspection.ProjectName,
	); err != nil {
		return retryable("relay_cleanup_failed")
	}
	progress = progress.MarkFactsCleared()
	return s.saveProjectProgress(inspection, progress)
}

func (s Service) removeProjectOperator(ctx context.Context, inspection ProjectInspection) Result {
	progress, result := s.projectProgress(inspection)
	if result.Status != "" {
		return result
	}
	if progress.OperatorRemoved {
		return Result{}
	}
	if err := s.ProjectCleaner.RemoveOperator(ctx, inspection.RoomID, s.Operator); err != nil {
		return retryable("operator_removal_failed")
	}
	progress = progress.MarkOperatorRemoved()
	return s.saveProjectProgress(inspection, progress)
}

func (s Service) leaveProject(ctx context.Context, inspection ProjectInspection) Result {
	progress, result := s.projectProgress(inspection)
	if result.Status != "" {
		return result
	}
	if progress.BridgeLeft {
		return Result{}
	}
	if err := s.ProjectCleaner.LeaveAndForget(ctx, inspection.RoomID); err != nil {
		return retryable("bridge_membership_failed")
	}
	progress = progress.MarkBridgeLeft()
	return s.saveProjectProgress(inspection, progress)
}

func (s Service) projectProgress(inspection ProjectInspection) (state.RoomCleanup, Result) {
	loaded, err := s.Load()
	if err != nil {
		return state.RoomCleanup{}, retryable("state_unavailable")
	}
	project, ok := currentProject(loaded, inspection)
	if !ok {
		return state.RoomCleanup{}, retryable("state_unavailable")
	}
	if project.Cleanup == nil {
		return state.RoomCleanup{}, Result{}
	}
	return *project.Cleanup, Result{}
}

func currentProject(loaded *state.State, inspection ProjectInspection) (state.Project, bool) {
	forge, ok := loaded.Forges[inspection.Root]
	if !ok {
		return state.Project{}, false
	}
	project, ok := forge.Projects[inspection.ProjectName]
	return project, ok && project.RoomID == inspection.RoomID
}

func (s Service) reloadProject(root, name string) (ProjectInspection, Result, bool) {
	if s.ProjectCleaner == nil {
		return ProjectInspection{}, retryable("matrix_unavailable"), false
	}
	loaded, err := s.Load()
	if err != nil {
		return ProjectInspection{}, retryable("state_unavailable"), false
	}
	fresh := InspectProject(s.Config, loaded, root, name, s.Projects)
	return fresh, fresh.Result, fresh.Result.Status == Ready
}

func (s Service) saveProjectProgress(inspection ProjectInspection, progress state.RoomCleanup) Result {
	loaded, err := s.Load()
	if err != nil {
		return retryable("state_unavailable")
	}
	if !recordProjectProgress(loaded, inspection, progress) {
		return retryable("state_unavailable")
	}
	if err := loaded.Save(s.StatePath); err != nil {
		return retryable("state_unavailable")
	}
	return Result{}
}

func recordProjectProgress(loaded *state.State, inspection ProjectInspection, progress state.RoomCleanup) bool {
	forge, ok := loaded.Forges[inspection.Root]
	if !ok {
		return false
	}
	project, ok := forge.Projects[inspection.ProjectName]
	if !ok || project.RoomID != inspection.RoomID {
		return false
	}
	project.Cleanup = &progress
	forge.Projects[inspection.ProjectName] = project
	loaded.Forges[inspection.Root] = forge
	return true
}

func (s Service) finalizeProject(root, name, roomID string) Result {
	loaded, err := s.Load()
	if err != nil {
		return retryable("state_unavailable")
	}
	if result, done := removeProjectRecord(loaded, root, name, roomID); done {
		return result
	}
	if err := loaded.Save(s.StatePath); err != nil {
		return retryable("state_unavailable")
	}
	return Result{Status: Removed}
}

func removeProjectRecord(loaded *state.State, root, name, roomID string) (Result, bool) {
	forge, ok := loaded.Forges[root]
	if !ok {
		return Result{Status: NothingToRemove}, true
	}
	project, ok := forge.Projects[name]
	if !ok || project.RoomID != roomID {
		return Result{Status: NothingToRemove}, true
	}
	purgeProjectRelay(&loaded.Relay, root, name, roomID, forge.ActivityRoomID)
	delete(forge.Projects, name)
	loaded.Forges[root] = forge
	return Result{}, false
}

func purgeProjectRelay(bookkeeping *relay.State, root, name, roomID, activityRoomID string) {
	bookkeeping.EnsureMaps()
	purgeProjectWaiting(bookkeeping, name, roomID, activityRoomID)
	purgeProjectRooms(bookkeeping, name, roomID)
	purgeProjectActivity(bookkeeping, root, name)
	bookkeeping.ForgetProjectScoped(root, name)
}

func purgeProjectWaiting(bookkeeping *relay.State, name, roomID, activityRoomID string) {
	for key := range bookkeeping.Waiting {
		waitingRoom, eventType, stateKey, ok := splitWaiting(key)
		if ok && projectWaitingFact(waitingRoom, eventType, stateKey, name, roomID, activityRoomID) {
			delete(bookkeeping.Waiting, key)
		}
	}
}

func projectWaitingFact(room, eventType, stateKey, name, roomID, activityRoomID string) bool {
	if room == roomID {
		return true
	}
	return room == activityRoomID && eventType == relay.BoardFactType && stateKey == name
}

func purgeProjectRooms(bookkeeping *relay.State, name, roomID string) {
	purgeProjectApprovals(bookkeeping, name, roomID)
	purgeProjectClarifications(bookkeeping, name, roomID)
}

func purgeProjectApprovals(bookkeeping *relay.State, name, roomID string) {
	for key, approval := range bookkeeping.Approvals {
		if approval.RoomID == roomID || strings.HasPrefix(key, name+"/") {
			delete(bookkeeping.Approvals, key)
		}
	}
}

func purgeProjectClarifications(bookkeeping *relay.State, name, roomID string) {
	for key, clarification := range bookkeeping.Clarifications {
		if clarification.RoomID == roomID || strings.HasPrefix(key, name+"/") {
			delete(bookkeeping.Clarifications, key)
		}
	}
}

func purgeProjectActivity(bookkeeping *relay.State, root, name string) {
	for key, activity := range bookkeeping.Activity {
		if activity.ForgeRoot == root && activity.Project == name {
			delete(bookkeeping.Activity, key)
		}
	}
}

func refusedProject(reason string) ProjectInspection {
	return ProjectInspection{Result: Result{Status: Refused, Reason: reason}}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-07T23:53:54+01:00","module_hash":"ee8b778122095af687a9b46a9e6e6c6a74686f51b9a937eeaebae92b96fa656d","functions":[{"id":"func/InspectProject","name":"InspectProject","line":42,"end_line":50,"hash":"69b7adc0220976536a4a8be7f06a4b1e0b607a7e23e5a9c7953f202f1120509d"},{"id":"func/validateProjectRequest","name":"validateProjectRequest","line":52,"end_line":63,"hash":"fcfceb2c6e0eff13480610be9d801b9a07faf7318db3f9dd55374edf748ba337"},{"id":"func/validateProjectCatalog","name":"validateProjectCatalog","line":65,"end_line":73,"hash":"9db8fa70653de68f89953dabad38d48aa85b7c649812b4cbec3c186a9364d4ea"},{"id":"func/validateCatalogForge","name":"validateCatalogForge","line":75,"end_line":84,"hash":"9a49195206cbb0169a09a56a22b344f72fa613f9061145263f8553debf60ca07"},{"id":"func/validateCatalogProject","name":"validateCatalogProject","line":86,"end_line":95,"hash":"1d6edbfdc678f833a1b9f4045bd6e5faf08773f10e1cec9c306bc540743cd367"},{"id":"func/inspectProjectState","name":"inspectProjectState","line":97,"end_line":107,"hash":"72cc4643dac0d3dddd6e69e07ee4f17d39aeb0fb94dd4af77acb3439c71efe1e"},{"id":"func/projectForgeForRemoval","name":"projectForgeForRemoval","line":109,"end_line":121,"hash":"274e108803320b7ef23421c6fee5394feb79eaa369b38fdf38b18fe8f71d2ecf"},{"id":"func/projectRecordForRemoval","name":"projectRecordForRemoval","line":123,"end_line":132,"hash":"a47266f7ebbb5be1f61af5d7f155bc88b5ac6c5f0cbc8a4b30018f10fef27db4"},{"id":"func/readyProjectInspection","name":"readyProjectInspection","line":134,"end_line":143,"hash":"53250cabbebdaced64a897f05682df49d11f0567125f2a808e7d36f2c229203d"},{"id":"func/configuredRoot","name":"configuredRoot","line":145,"end_line":152,"hash":"4622b2269c9fc0da419538142b95b08ebd46672a13f6d43c04d5838183592873"},{"id":"func/unsafeProjectName","name":"unsafeProjectName","line":154,"end_line":159,"hash":"8a91920a28835ddaecfec03f9b9618abaa6f7ad87b5ef6fb482ecef690cfec9d"},{"id":"func/projectNameShapeUnsafe","name":"projectNameShapeUnsafe","line":161,"end_line":164,"hash":"6a15522c5c19d9a569873cfd98ffd5a91313ba086240ff30355cccd9fc993805"},{"id":"func/projectRoomCollides","name":"projectRoomCollides","line":166,"end_line":184,"hash":"21a4df63476404cd65bef5024294627b2ce472a725f4a65725d6b28bdfc028bd"},{"id":"func/Service.RemoveProject","name":"Service.RemoveProject","line":188,"end_line":206,"hash":"43f560e66804e600129743c2413b2618eeb3efad570da20af473d9b032d9a014"},{"id":"func/Service.cleanProjectFacts","name":"Service.cleanProjectFacts","line":208,"end_line":224,"hash":"3c68cc14d6bb2d5bf4a63358a14659b4ff58f6f17c688040c3343b6d53f96cb2"},{"id":"func/Service.removeProjectOperator","name":"Service.removeProjectOperator","line":226,"end_line":239,"hash":"9a701ac1de6a735743baa32e36326febc0093cc87f668ea7f321f984a56c800c"},{"id":"func/Service.leaveProject","name":"Service.leaveProject","line":241,"end_line":254,"hash":"612178273d2fa77ad8f85ccccdbb70c5c2b945e96d8f2b2a3a98c61dac950676"},{"id":"func/Service.projectProgress","name":"Service.projectProgress","line":256,"end_line":269,"hash":"1ad4281fddf7abda3d122160c46c6ae31ff5d339ff82143831ad48d922dae490"},{"id":"func/currentProject","name":"currentProject","line":271,"end_line":278,"hash":"6d80c2be0f540e7d3fc26f233d6978f472e0c9b504ac898f8c6416ef1c94b937"},{"id":"func/Service.reloadProject","name":"Service.reloadProject","line":280,"end_line":290,"hash":"20befbf49098a9d678395898b16da9f58dc887939071ac49904618f03ef06fd4"},{"id":"func/Service.saveProjectProgress","name":"Service.saveProjectProgress","line":292,"end_line":304,"hash":"f30350645b314634013b80135d97a0844681c580d08387493fe331f66223874f"},{"id":"func/recordProjectProgress","name":"recordProjectProgress","line":306,"end_line":319,"hash":"aa4699992792dd7e1e4097e1367911a4a6047dab8c0904814f38db91761f8fce"},{"id":"func/Service.finalizeProject","name":"Service.finalizeProject","line":321,"end_line":333,"hash":"a208844f9dd529ff7f266e9800e19a930d5042e1f25d147afc80d0ddb90e2f97"},{"id":"func/removeProjectRecord","name":"removeProjectRecord","line":335,"end_line":348,"hash":"d6feef326404aa83cb32b4e22bb0f2bed86604b2b9692518bff7292776e4f521"},{"id":"func/purgeProjectRelay","name":"purgeProjectRelay","line":350,"end_line":356,"hash":"513f717f16ed213b571deeccd4d575ac0510306bc62d6a6ca435edf4c9b70246"},{"id":"func/purgeProjectWaiting","name":"purgeProjectWaiting","line":358,"end_line":365,"hash":"ce3b0585cb81a0375fc4ead5016baefd4d2507cddd475740b1bc51fba0258b0f"},{"id":"func/projectWaitingFact","name":"projectWaitingFact","line":367,"end_line":372,"hash":"08611a041624d0e1f7f086545e3dc60f2c3b8107287d7d0298c4ae60a3c6e7ab"},{"id":"func/purgeProjectRooms","name":"purgeProjectRooms","line":374,"end_line":377,"hash":"f80165758b64d902630c60da39d17f18863cee6fb921ca8f4a2ccc5640af85a8"},{"id":"func/purgeProjectApprovals","name":"purgeProjectApprovals","line":379,"end_line":385,"hash":"5ef090e00cc8913d364ad8ebb2f4ca21d427c689ee9485eed7aa48edb71f257e"},{"id":"func/purgeProjectClarifications","name":"purgeProjectClarifications","line":387,"end_line":393,"hash":"2c06a948528700b27e0f2b9a1361eaa367068b44f0b735827180c5245e43bdcd"},{"id":"func/purgeProjectActivity","name":"purgeProjectActivity","line":395,"end_line":401,"hash":"117cbde90f20715c91ffc1a248a7cbe2dee85a2c445513ad353b3fd468025a61"},{"id":"func/refusedProject","name":"refusedProject","line":403,"end_line":405,"hash":"e43d7bcb3798ac519073657fdb34b382a858ba9249469edbc85724158c99a180"}]}
// mutate4go-manifest-end
