package pkg

import (
	"testing"
)

func TestParse(t *testing.T) {
	data := []byte(`
name = "ripgrep"
version = "14.1.0"
description = "Fast search tool"
homepage = "https://github.com/BurntSushi/ripgrep"
license = "MIT"

[source]
url = "https://example.com/rg-{{version}}.tar.gz"
sha256 = "abc123"
strip = 1

[install_paths]
prefix = "/usr/local"

[[install_steps]]
type = "copy"
src = "rg"
dest = "{{bindir}}/rg"
mode = "0755"

[[install_steps]]
type = "mkdir"
path = "{{docdir}}"
`)

	pkg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if pkg.Name != "ripgrep" {
		t.Errorf("expected name 'ripgrep', got %q", pkg.Name)
	}
	if pkg.Version != "14.1.0" {
		t.Errorf("expected version '14.1.0', got %q", pkg.Version)
	}
	if pkg.Source.SourceType() != "url" {
		t.Errorf("expected source type 'url', got %q", pkg.Source.SourceType())
	}
	if len(pkg.InstallSteps) != 2 {
		t.Errorf("expected 2 install steps, got %d", len(pkg.InstallSteps))
	}
}

func TestParseValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{
			name:    "missing name",
			data:    `version = "1.0"`,
			wantErr: "package name is required",
		},
		{
			name:    "missing version",
			data:    `name = "test"`,
			wantErr: "package version is required",
		},
		{
			name: "missing source",
			data: `
name = "test"
version = "1.0"
[[install_steps]]
type = "mkdir"
path = "/tmp"
`,
			wantErr: "package source is required",
		},
		{
			name: "multiple sources",
			data: `
name = "test"
version = "1.0"
[source]
url = "https://example.com/test.tar.gz"
git = "https://github.com/test/test"
sha256 = "abc123"
[[install_steps]]
type = "mkdir"
path = "/tmp"
`,
			wantErr: "only one source type allowed",
		},
		{
			name: "missing sha256 for url",
			data: `
name = "test"
version = "1.0"
[source]
url = "https://example.com/test.tar.gz"
[[install_steps]]
type = "mkdir"
path = "/tmp"
`,
			wantErr: "sha256 checksum required",
		},
		{
			name: "missing install steps",
			data: `
name = "test"
version = "1.0"
[source]
url = "https://example.com/test.tar.gz"
sha256 = "abc123"
`,
			wantErr: "at least one install step is required",
		},
		{
			name: "invalid step type",
			data: `
name = "test"
version = "1.0"
[source]
url = "https://example.com/test.tar.gz"
sha256 = "abc123"
[[install_steps]]
type = "invalid"
`,
			wantErr: "unknown step type: invalid",
		},
		{
			name: "copy step missing src",
			data: `
name = "test"
version = "1.0"
[source]
url = "https://example.com/test.tar.gz"
sha256 = "abc123"
[[install_steps]]
type = "copy"
dest = "/usr/bin/test"
`,
			wantErr: "copy step requires src",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.data))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestExpandedPaths(t *testing.T) {
	pkg := &Package{
		Name:    "test",
		Version: "1.0.0",
		InstallPaths: InstallPaths{
			Prefix: "/opt",
		},
	}
	pkg.applyDefaults()

	paths := pkg.ExpandedPaths()

	if paths.Prefix != "/opt" {
		t.Errorf("expected prefix '/opt', got %q", paths.Prefix)
	}
	if paths.BinDir != "/opt/bin" {
		t.Errorf("expected bindir '/opt/bin', got %q", paths.BinDir)
	}
	if paths.DocDir != "/opt/share/doc/test" {
		t.Errorf("expected docdir '/opt/share/doc/test', got %q", paths.DocDir)
	}
}

func TestExpandedSource(t *testing.T) {
	pkg := &Package{
		Name:    "test",
		Version: "2.0.0",
		Source: Source{
			URL:    "https://example.com/test-{{version}}.tar.gz",
			SHA256: "abc",
		},
	}

	src := pkg.ExpandedSource()

	expected := "https://example.com/test-2.0.0.tar.gz"
	if src.URL != expected {
		t.Errorf("expected URL %q, got %q", expected, src.URL)
	}
}

func TestExpandedSteps(t *testing.T) {
	pkg := &Package{
		Name:    "test",
		Version: "1.0.0",
		Source: Source{
			URL:    "https://example.com/test.tar.gz",
			SHA256: "abc",
		},
		InstallSteps: []InstallStep{
			{Type: "copy", Src: "bin/test", Dest: "{{bindir}}/test"},
			{Type: "mkdir", Path: "{{docdir}}"},
		},
	}
	pkg.applyDefaults()

	steps := pkg.ExpandedSteps("/tmp/src")

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Dest != "/usr/local/bin/test" {
		t.Errorf("expected dest '/usr/local/bin/test', got %q", steps[0].Dest)
	}
	if steps[1].Path != "/usr/local/share/doc/test" {
		t.Errorf("expected path '/usr/local/share/doc/test', got %q", steps[1].Path)
	}
}

func TestGitSource(t *testing.T) {
	data := []byte(`
name = "test"
version = "1.0.0"

[source]
git = "https://github.com/test/test.git"
ref = "v1.0.0"

[[install_steps]]
type = "run"
command = "make install PREFIX={{prefix}}"
`)

	pkg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if pkg.Source.SourceType() != "git" {
		t.Errorf("expected source type 'git', got %q", pkg.Source.SourceType())
	}
	if pkg.Source.Ref != "v1.0.0" {
		t.Errorf("expected ref 'v1.0.0', got %q", pkg.Source.Ref)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
