//go:build !wasm

package dom

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomDoesNotImportAssetmin(t *testing.T) {
	err := filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, "\"")
			if strings.Contains(impPath, "assetmin") {
				t.Errorf("file %s imports forbidden package %s", path, impPath)
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk files: %v", err)
	}
}
