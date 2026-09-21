// Package fixtures starts the outside world the acceptance tests need: a
// fixture forge root with its dashboard, a Synapse homeserver the tests run
// themselves, the bridge process, and a Matrix client standing in for the
// operator.
package fixtures

import (
	"os"
	"path/filepath"
	"runtime"
)

// ProjectRootEnv names the environment variable that points at the project
// checkout, so that a test binary started from anywhere finds the fixtures.
const ProjectRootEnv = "FORGELET_PROJECT_ROOT"

// WorkDirEnv names the environment variable that points at the scratch
// directory a test run may use.
const WorkDirEnv = "FORGELET_ACCEPTANCE_WORK_DIR"

// ProjectRoot is the checkout the tests belong to.
func ProjectRoot() string {
	if root := os.Getenv(ProjectRootEnv); root != "" {
		return root
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		// .../acceptance/fixtures/paths.go -> checkout root
		return filepath.Dir(filepath.Dir(filepath.Dir(file)))
	}
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}

// WorkDir is the scratch directory of the current test run.
func WorkDir() string {
	if dir := os.Getenv(WorkDirEnv); dir != "" {
		return dir
	}
	return filepath.Join(ProjectRoot(), "build", "acceptance", "run")
}

// SynapseEnvDir is the project-local Python environment the pinned Synapse
// lives in, so that no system package is needed.
func SynapseEnvDir() string {
	return filepath.Join(ProjectRoot(), "build", "acceptance", "synapse")
}

// SynapseRequirements pins the Synapse version the acceptance tests run.
func SynapseRequirements() string {
	return filepath.Join(ProjectRoot(), "acceptance", "synapse", "requirements.txt")
}

// FeaturesDir holds the Gherkin features the bridge answers to.
func FeaturesDir() string {
	return filepath.Join(ProjectRoot(), "features")
}
