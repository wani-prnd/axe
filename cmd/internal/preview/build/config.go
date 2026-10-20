// Package build provides the xcodebuild-level build operations for axe preview.
// It encapsulates project configuration, build settings extraction, project
// building, and compiler path extraction — all device-independent concerns.
package build

import (
	"fmt"
	"path/filepath"
)

// ProjectConfig abstracts --project / --workspace + --scheme.
// Paths are stored as absolute paths.
type ProjectConfig struct {
	Project       string
	Workspace     string
	Scheme        string
	Configuration string // e.g. "Debug", "Release"; empty means xcodebuild default

	// BuildInfoPath is a BuildInfo JSON file (see buildinfo.go). When set, axe uses it
	// instead of running xcodebuild, so any build system can drive previews.
	BuildInfoPath string
	// Root is the project root used when neither Project nor Workspace is set (build info mode).
	Root string
}

// UsesBuildInfo reports whether the build is driven by a BuildInfo file.
func (pc ProjectConfig) UsesBuildInfo() bool {
	return pc.BuildInfoPath != ""
}

// NewProjectConfig creates a ProjectConfig with absolute paths resolved.
func NewProjectConfig(project, workspace, scheme, configuration string) (ProjectConfig, error) {
	pc := ProjectConfig{Scheme: scheme, Configuration: configuration}
	if workspace != "" {
		abs, err := filepath.Abs(workspace)
		if err != nil {
			return pc, fmt.Errorf("resolving workspace path: %w", err)
		}
		pc.Workspace = abs
	}
	if project != "" {
		abs, err := filepath.Abs(project)
		if err != nil {
			return pc, fmt.Errorf("resolving project path: %w", err)
		}
		pc.Project = abs
	}
	return pc, nil
}

// XcodebuildArgs returns the project/workspace arguments for xcodebuild.
func (pc ProjectConfig) XcodebuildArgs() []string {
	var args []string
	if pc.Workspace != "" {
		args = []string{"-workspace", pc.Workspace, "-scheme", pc.Scheme}
	} else {
		args = []string{"-project", pc.Project, "-scheme", pc.Scheme}
	}
	if pc.Configuration != "" {
		args = append(args, "-configuration", pc.Configuration)
	}
	return args
}

// PrimaryPath returns the path that identifies the project for cache naming: the
// workspace or project path (whichever is set), or Root in build info mode without one.
func (pc ProjectConfig) PrimaryPath() string {
	if pc.Workspace != "" {
		return pc.Workspace
	}
	if pc.Project != "" {
		return pc.Project
	}
	return pc.Root
}

// ProjectRoot returns the directory that holds the project sources: the directory
// of the workspace or project, or Root in build info mode without one.
func (pc ProjectConfig) ProjectRoot() string {
	if pc.Workspace != "" {
		return filepath.Dir(pc.Workspace)
	}
	if pc.Project != "" {
		return filepath.Dir(pc.Project)
	}
	return pc.Root
}
