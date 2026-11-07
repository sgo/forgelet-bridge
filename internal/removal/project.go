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
	if strings.TrimSpace(root) == "" || hasParentTraversal(root) || !filepath.IsAbs(root) {
		return refusedProject("unsafe_root")
	}
	if !configuredRoot(cfg, root) {
		return refusedProject("unknown_root")
	}
	if unsafeProjectName(name) {
		return refusedProject("unsafe_project")
	}
	if catalog == nil {
		return refusedProject("state_unavailable")
	}
	forgeExists, err := catalog.ForgeExists(root)
	if err != nil {
		return refusedProject("state_unavailable")
	}
	if !forgeExists {
		return refusedProject("unknown_root")
	}
	managed, err := catalog.ManagedProject(root, name)
	if err != nil {
		return refusedProject("state_unavailable")
	}
	if !managed {
		return refusedProject("unknown_project")
	}
	if loaded == nil {
		return refusedProject("incomplete_project_state")
	}

	forge, ok := loaded.Forges[root]
	if !ok {
		return ProjectInspection{Result: Result{Status: NothingToRemove}}
	}
	if !completeForge(forge) {
		return refusedProject("incomplete_project_state")
	}
	project, ok := forge.Projects[name]
	if !ok {
		return ProjectInspection{Result: Result{Status: NothingToRemove}}
	}
	if project.RoomID == "" || projectRoomCollides(forge, name, project.RoomID) {
		return refusedProject("incomplete_project_state")
	}

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
	if name == "" || name != strings.TrimSpace(name) ||
		name == "." || name == ".." || filepath.IsAbs(name) ||
		strings.HasPrefix(name, "!") ||
		strings.ContainsAny(name, "/\\\x00") {
		return true
	}
	return filepath.Base(name) != name
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
	progress := state.RoomCleanup{}
	if fresh.Project.Cleanup != nil {
		progress = *fresh.Project.Cleanup
	}

	if !progress.FactsCleared {
		if err := s.ProjectCleaner.ClearProjectFacts(
			ctx, fresh.RoomID, fresh.Forge.ActivityRoomID,
			fresh.Forge.ApprovalsRoomID, fresh.ProjectName,
		); err != nil {
			return retryable("relay_cleanup_failed")
		}
		progress.FactsCleared = true
		if result := s.saveProjectProgress(fresh, progress); result.Status != "" {
			return result
		}
	}
	if !progress.OperatorRemoved {
		if err := s.ProjectCleaner.RemoveOperator(ctx, fresh.RoomID, s.Operator); err != nil {
			return retryable("operator_removal_failed")
		}
		progress.OperatorRemoved = true
		if result := s.saveProjectProgress(fresh, progress); result.Status != "" {
			return result
		}
	}
	if !progress.BridgeLeft {
		if err := s.ProjectCleaner.LeaveAndForget(ctx, fresh.RoomID); err != nil {
			return retryable("bridge_membership_failed")
		}
		progress.BridgeLeft = true
		if result := s.saveProjectProgress(fresh, progress); result.Status != "" {
			return result
		}
	}
	return s.finalizeProject(fresh.Root, fresh.ProjectName, fresh.RoomID)
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
	forge, ok := loaded.Forges[inspection.Root]
	if !ok {
		return retryable("state_unavailable")
	}
	project, ok := forge.Projects[inspection.ProjectName]
	if !ok || project.RoomID != inspection.RoomID {
		return retryable("state_unavailable")
	}
	project.Cleanup = &progress
	forge.Projects[inspection.ProjectName] = project
	loaded.Forges[inspection.Root] = forge
	if err := loaded.Save(s.StatePath); err != nil {
		return retryable("state_unavailable")
	}
	return Result{}
}

func (s Service) finalizeProject(root, name, roomID string) Result {
	loaded, err := s.Load()
	if err != nil {
		return retryable("state_unavailable")
	}
	forge, ok := loaded.Forges[root]
	if !ok {
		return Result{Status: NothingToRemove}
	}
	project, ok := forge.Projects[name]
	if !ok || project.RoomID != roomID {
		return Result{Status: NothingToRemove}
	}
	purgeProjectRelay(&loaded.Relay, root, name, roomID, forge.ActivityRoomID)
	delete(forge.Projects, name)
	loaded.Forges[root] = forge
	if err := loaded.Save(s.StatePath); err != nil {
		return retryable("state_unavailable")
	}
	return Result{Status: Removed}
}

func purgeProjectRelay(bookkeeping *relay.State, root, name, roomID, activityRoomID string) {
	bookkeeping.EnsureMaps()
	for key := range bookkeeping.Waiting {
		waitingRoom, eventType, stateKey, ok := splitWaiting(key)
		if ok && (waitingRoom == roomID ||
			waitingRoom == activityRoomID && eventType == relay.BoardFactType && stateKey == name) {
			delete(bookkeeping.Waiting, key)
		}
	}
	for key, approval := range bookkeeping.Approvals {
		if approval.RoomID == roomID || strings.HasPrefix(key, name+"/") {
			delete(bookkeeping.Approvals, key)
		}
	}
	for key, clarification := range bookkeeping.Clarifications {
		if clarification.RoomID == roomID || strings.HasPrefix(key, name+"/") {
			delete(bookkeeping.Clarifications, key)
		}
	}
	for key, activity := range bookkeeping.Activity {
		if activity.ForgeRoot == root && activity.Project == name {
			delete(bookkeeping.Activity, key)
		}
	}
	bookkeeping.ForgetProjectScoped(root, name)
}

func refusedProject(reason string) ProjectInspection {
	return ProjectInspection{Result: Result{Status: Refused, Reason: reason}}
}
