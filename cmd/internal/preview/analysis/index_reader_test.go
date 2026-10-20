package analysis

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestXcodeToolchainLibPath(t *testing.T) {
	libPath := xcodeToolchainLibPath()
	if libPath == "" {
		t.Skip("xcode-select not available; skipping")
	}

	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Errorf("xcodeToolchainLibPath() = %q, but directory does not exist", libPath)
	}
}

// The reader resolves relative paths in the index store against its working directory, so it
// must run in the source root that LoadIndexStore resolves them against.
func TestIndexReaderCommandRunsInSourceRoot(t *testing.T) {
	root := t.TempDir()
	cmd := indexReaderCommand(context.Background(), "/bin/reader", "/idx", root)
	if cmd.Dir != root {
		t.Errorf("Dir = %q, want %q", cmd.Dir, root)
	}
	if want := []string{"/bin/reader", "/idx", "--source-root", root}; !slices.Equal(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}

	cmd = indexReaderCommand(context.Background(), "/bin/reader", "/idx", "")
	if cmd.Dir != "" {
		t.Errorf("Dir without sourceRoot = %q, want empty", cmd.Dir)
	}
	if want := []string{"/bin/reader", "/idx"}; !slices.Equal(cmd.Args, want) {
		t.Errorf("Args without sourceRoot = %v, want %v", cmd.Args, want)
	}
}

// A relative source root must not be resolved twice (once by Dir, once by the reader).
func TestIndexReaderCommandMakesPathsAbsolute(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, _ := os.Getwd()
	cmd := indexReaderCommand(context.Background(), "/bin/reader", "idx", "src")
	if want := filepath.Join(cwd, "src"); cmd.Dir != want || cmd.Args[3] != want {
		t.Errorf("Dir = %q, source-root = %q, want %q", cmd.Dir, cmd.Args[3], want)
	}
	if cmd.Args[1] != filepath.Join(cwd, "idx") {
		t.Errorf("index store arg = %q", cmd.Args[1])
	}
}
