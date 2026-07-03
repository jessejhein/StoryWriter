// BDD Scenario: 8.4.1 - Branch-owned types at the repository boundary
// Requirements: M8-R01, M8-R12
// Test purpose: branch policy/orchestration files do not import internal/gitstore.

package branch_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Test: branch policy and orchestration files (excluding the concrete adapter)
// do not import internal/gitstore, keeping the boundary clean.
func TestBranchPolicyFilesDoNotImportGitstore(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			if strings.HasSuffix(filename, "repository.go") {
				continue
			}
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == "storywork/internal/gitstore" {
					t.Fatalf("%s imports internal/gitstore; only repository.go may do so", filename)
				}
			}
		}
	}
}
