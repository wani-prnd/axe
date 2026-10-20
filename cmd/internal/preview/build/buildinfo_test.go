package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeBuildInfo(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "build-info.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildInfoSettings(t *testing.T) {
	path := writeBuildInfo(t, `{
	  "schemaVersion": 1, "module": "FeatureUI", "configuration": "Debug", "provider": "bazel",
	  "swiftcArguments": ["-module-name", "FeatureUI", "-target", "arm64-apple-ios17.0-simulator", "-swift-version", "6",
	                      "-Xcc", "-fmodule-map-file=/out/Clib.modulemap"],
	  "sourceFiles": ["/src/FeatureUI/View.swift"],
	  "moduleSearchPaths": ["/out/FeatureUI", "/out/app"],
	  "frameworkSearchPaths": ["/out/Frameworks"],
	  "indexStorePath": "/out/index",
	  "transitiveDependencies": [],
	  "appBundlePath": "/out/app/MyApp.app",
	  "appBundleId": "com.example.app"
	}`)
	info, err := LoadBuildInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := info.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ModuleName != "MyApp" || s.BundleID != "axe.com.example.app" || s.OriginalBundleID != "com.example.app" {
		t.Errorf("app identity: %+v", s)
	}
	if s.BuiltProductsDir != "/out/app" || s.DeploymentTarget != "17.0" || s.SwiftVersion != "6" {
		t.Errorf("paths/versions: %+v", s)
	}
	if len(s.ExtraIncludePaths) != 1 || s.ExtraIncludePaths[0] != "/out/FeatureUI" {
		t.Errorf("include paths (BuiltProductsDir must be de-duplicated): %v", s.ExtraIncludePaths)
	}
	if len(s.ExtraFrameworkPaths) != 1 || len(s.ExtraModuleMapFiles) != 1 || s.ExtraModuleMapFiles[0] != "/out/Clib.modulemap" {
		t.Errorf("framework/modulemap paths: %v %v", s.ExtraFrameworkPaths, s.ExtraModuleMapFiles)
	}
	if got := s.ModuleForSource("/src/FeatureUI/View.swift"); got != "FeatureUI" {
		t.Errorf("ModuleForSource = %q", got)
	}
	if got := s.ModuleForSource("/elsewhere.swift"); got != "" {
		t.Errorf("ModuleForSource for unknown file = %q", got)
	}

	dirs := ProjectDirs{Root: t.TempDir(), Build: filepath.Join(t.TempDir(), "build")}
	result, err := PrepareFromBuildInfo(path, dirs)
	if err != nil {
		t.Fatal(err)
	}
	if result.Built {
		t.Error("build info mode must not report a build")
	}
	if target, _ := os.Readlink(dirs.IndexStorePath()); target != "/out/index" {
		t.Errorf("index store link = %q", target)
	}
}

func TestLoadBuildInfoRejectsInvalidInput(t *testing.T) {
	for name, body := range map[string]string{
		"version":  `{"schemaVersion": 2}`,
		"no app":   `{"schemaVersion": 1, "module": "M"}`,
		"not json": `{`,
	} {
		if _, err := LoadBuildInfo(writeBuildInfo(t, body)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	info := &BuildInfo{SchemaVersion: 1, AppBundlePath: "/a/A.app", AppBundleID: "a"}
	if _, err := info.Settings(); err == nil {
		t.Error("missing -target must be an error")
	}
}

func staticBuildInfoConfig(t *testing.T) ProjectConfig {
	t.Helper()
	path := writeBuildInfo(t, `{
	  "schemaVersion": 1, "module": "FeatureUI",
	  "swiftcArguments": ["-module-name", "FeatureUI", "-target", "arm64-apple-ios17.0-simulator"],
	  "appBundlePath": "/out/app/MyApp.app", "appBundleId": "com.example.app"
	}`)
	return ProjectConfig{BuildInfoPath: path}
}

// Prepare and Preparer must serve a static build info without touching xcodebuild.
func TestPrepareAndPreparerWithBuildInfoNeverCallXcodebuild(t *testing.T) {
	pc := staticBuildInfoConfig(t)
	dirs := ProjectDirs{Root: t.TempDir(), Build: filepath.Join(t.TempDir(), "build")}

	r := &fakeRunner{}
	if _, err := Prepare(context.Background(), pc, dirs, false, r); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := NewPreparer(pc, dirs, false, r).Prepare(context.Background()); err != nil {
		t.Fatalf("Preparer.Prepare: %v", err)
	}
	if r.fetchArgs != nil || r.buildArgs != nil {
		t.Errorf("xcodebuild must not run: fetch=%v build=%v", r.fetchArgs, r.buildArgs)
	}
}

// Run is the only path that rebuilds with xcodebuild outside Prepare (watch retry,
// serve optimistic rebuild); it must refuse a static build info configuration.
func TestRunRejectsBuildInfo(t *testing.T) {
	r := &fakeRunner{}
	err := Run(context.Background(), staticBuildInfoConfig(t), ProjectDirs{Build: t.TempDir()}, r)
	if !errors.Is(err, ErrRebuildUnsupported) {
		t.Fatalf("expected ErrRebuildUnsupported, got %v", err)
	}
	if r.buildArgs != nil {
		t.Errorf("xcodebuild must not run: %v", r.buildArgs)
	}
}

// A build info without an index store must not leave a link from an earlier run behind.
func TestPrepareFromBuildInfoRemovesStaleIndexStoreLink(t *testing.T) {
	root := t.TempDir()
	dirs := ProjectDirs{Root: root, Build: filepath.Join(root, "build")}
	withStore := writeBuildInfo(t, `{
	  "schemaVersion": 1, "module": "FeatureUI",
	  "swiftcArguments": ["-module-name", "FeatureUI", "-target", "arm64-apple-ios17.0-simulator"],
	  "appBundlePath": "/out/app/MyApp.app", "appBundleId": "com.example.app",
	  "indexStorePath": "/out/index"
	}`)
	if _, err := PrepareFromBuildInfo(withStore, dirs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dirs.IndexStorePath()); err != nil {
		t.Fatalf("expected index store link after first Prepare: %v", err)
	}
	if _, err := PrepareFromBuildInfo(staticBuildInfoConfig(t).BuildInfoPath, dirs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dirs.IndexStorePath()); !os.IsNotExist(err) {
		t.Errorf("stale index store link must be removed, Lstat err = %v", err)
	}
}
