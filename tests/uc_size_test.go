package dom_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTrackerNoReflect(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo not available")
	}

	src := `package main
import "webtyp.com/dom"
func main() {
	s := dom.NewString("a")
	dom.DeriveString(func() string {
		return s.Get() + s.Get()
	})
}
`
	tmpDir := t.TempDir()
	mainFile := tmpDir + "/main.go"
	os.WriteFile(mainFile, []byte(src), 0644)

	cmd := exec.Command("tinygo", "build", "-target", "wasm", "-opt=z", "-panic=trap", "-size=full", "-o", tmpDir+"/out.wasm", mainFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tinygo build failed: %v\n%s", err, string(out))
	}

	if strings.Contains(string(out), "internal/reflectlite") {
		t.Errorf("binary contains internal/reflectlite, which means reflect was pulled in")
	}
}
