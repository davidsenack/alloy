package pkg

import (
	"path/filepath"
	"testing"
)

func TestParseAllPackages(t *testing.T) {
	files, err := filepath.Glob("../../packages/*.toml")
	if err != nil {
		t.Fatalf("failed to glob packages: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no package files found")
	}

	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			pkg, err := ParseFile(f)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", f, err)
			}

			// Verify basic fields
			if pkg.Name == "" {
				t.Error("name is empty")
			}
			if pkg.Version == "" {
				t.Error("version is empty")
			}
			if len(pkg.InstallSteps) == 0 {
				t.Error("no install steps")
			}

			// Verify source is valid
			if pkg.Source.SourceType() == "" {
				t.Error("source type is empty")
			}

			t.Logf("OK: %s %s (%s source, %d steps)",
				pkg.Name, pkg.Version, pkg.Source.SourceType(), len(pkg.InstallSteps))
		})
	}
}
