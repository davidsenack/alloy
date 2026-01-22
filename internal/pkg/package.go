// Package pkg provides types and parsing for Alloy package definitions.
package pkg

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// Package represents a complete package definition.
type Package struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Description string   `toml:"description,omitempty"`
	Homepage    string   `toml:"homepage,omitempty"`
	License     string   `toml:"license,omitempty"`
	Provides    []string `toml:"provides,omitempty"`

	Source       Source        `toml:"source"`
	InstallPaths InstallPaths  `toml:"install_paths"`
	InstallSteps []InstallStep `toml:"install_steps"`
}

// Source defines where to obtain the package.
type Source struct {
	URL    string `toml:"url,omitempty"`
	Git    string `toml:"git,omitempty"`
	Binary string `toml:"binary,omitempty"`
	SHA256 string `toml:"sha256,omitempty"`
	Ref    string `toml:"ref,omitempty"`
	Strip  int    `toml:"strip,omitempty"`
}

// SourceType returns the type of source (url, git, or binary).
func (s Source) SourceType() string {
	if s.URL != "" {
		return "url"
	}
	if s.Git != "" {
		return "git"
	}
	if s.Binary != "" {
		return "binary"
	}
	return ""
}

// Location returns the source location (URL, git repo, or binary URL).
func (s Source) Location() string {
	if s.URL != "" {
		return s.URL
	}
	if s.Git != "" {
		return s.Git
	}
	if s.Binary != "" {
		return s.Binary
	}
	return ""
}

// InstallPaths defines where package files are installed.
type InstallPaths struct {
	Prefix  string `toml:"prefix,omitempty"`
	BinDir  string `toml:"bindir,omitempty"`
	LibDir  string `toml:"libdir,omitempty"`
	DataDir string `toml:"datadir,omitempty"`
	ManDir  string `toml:"mandir,omitempty"`
	DocDir  string `toml:"docdir,omitempty"`
}

// InstallStep represents a single installation action.
type InstallStep struct {
	Type      string   `toml:"type"`
	Command   string   `toml:"command,omitempty"`
	WorkDir   string   `toml:"workdir,omitempty"`
	Src       string   `toml:"src,omitempty"`
	Dest      string   `toml:"dest,omitempty"`
	Path      string   `toml:"path,omitempty"`
	Mode      string   `toml:"mode,omitempty"`
	Platforms []string `toml:"platforms,omitempty"`
}

// StepType constants for installation steps.
const (
	StepRun     = "run"
	StepCopy    = "copy"
	StepMkdir   = "mkdir"
	StepSymlink = "symlink"
)

// ParseFile reads and parses a package definition from a TOML file.
func ParseFile(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading package file: %w", err)
	}
	return Parse(data)
}

// Parse parses a package definition from TOML data.
func Parse(data []byte) (*Package, error) {
	var pkg Package
	if err := toml.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing package definition: %w", err)
	}

	if err := pkg.Validate(); err != nil {
		return nil, err
	}

	pkg.applyDefaults()
	return &pkg, nil
}

// Validate checks that the package definition is valid.
func (p *Package) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("package name is required")
	}
	if p.Version == "" {
		return fmt.Errorf("package version is required")
	}

	// Validate source
	sourceCount := 0
	if p.Source.URL != "" {
		sourceCount++
	}
	if p.Source.Git != "" {
		sourceCount++
	}
	if p.Source.Binary != "" {
		sourceCount++
	}
	if sourceCount == 0 {
		return fmt.Errorf("package source is required (url, git, or binary)")
	}
	if sourceCount > 1 {
		return fmt.Errorf("only one source type allowed (url, git, or binary)")
	}

	// Require checksum for url and binary sources
	if (p.Source.URL != "" || p.Source.Binary != "") && p.Source.SHA256 == "" {
		return fmt.Errorf("sha256 checksum required for url/binary sources")
	}

	// Validate install steps
	if len(p.InstallSteps) == 0 {
		return fmt.Errorf("at least one install step is required")
	}
	for i, step := range p.InstallSteps {
		if err := validateStep(step); err != nil {
			return fmt.Errorf("install_steps[%d]: %w", i, err)
		}
	}

	return nil
}

func validateStep(step InstallStep) error {
	switch step.Type {
	case StepRun:
		if step.Command == "" {
			return fmt.Errorf("run step requires command")
		}
	case StepCopy:
		if step.Src == "" {
			return fmt.Errorf("copy step requires src")
		}
		if step.Dest == "" {
			return fmt.Errorf("copy step requires dest")
		}
	case StepMkdir:
		if step.Path == "" {
			return fmt.Errorf("mkdir step requires path")
		}
	case StepSymlink:
		if step.Src == "" {
			return fmt.Errorf("symlink step requires src")
		}
		if step.Dest == "" {
			return fmt.Errorf("symlink step requires dest")
		}
	case "":
		return fmt.Errorf("step type is required")
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
	return nil
}

func (p *Package) applyDefaults() {
	if p.InstallPaths.Prefix == "" {
		p.InstallPaths.Prefix = "/usr/local"
	}
	if p.InstallPaths.BinDir == "" {
		p.InstallPaths.BinDir = "{{prefix}}/bin"
	}
	if p.InstallPaths.LibDir == "" {
		p.InstallPaths.LibDir = "{{prefix}}/lib"
	}
	if p.InstallPaths.DataDir == "" {
		p.InstallPaths.DataDir = "{{prefix}}/share"
	}
	if p.InstallPaths.ManDir == "" {
		p.InstallPaths.ManDir = "{{datadir}}/man"
	}
	if p.InstallPaths.DocDir == "" {
		p.InstallPaths.DocDir = "{{datadir}}/doc/{{name}}"
	}
	if p.Source.Strip == 0 && p.Source.URL != "" {
		p.Source.Strip = 1
	}
}

// ExpandedPaths returns InstallPaths with all template variables expanded.
func (p *Package) ExpandedPaths() InstallPaths {
	vars := p.baseVars()

	paths := InstallPaths{
		Prefix: p.expand(p.InstallPaths.Prefix, vars),
	}
	vars["prefix"] = paths.Prefix

	paths.BinDir = p.expand(p.InstallPaths.BinDir, vars)
	paths.LibDir = p.expand(p.InstallPaths.LibDir, vars)
	paths.DataDir = p.expand(p.InstallPaths.DataDir, vars)
	vars["bindir"] = paths.BinDir
	vars["libdir"] = paths.LibDir
	vars["datadir"] = paths.DataDir

	paths.ManDir = p.expand(p.InstallPaths.ManDir, vars)
	paths.DocDir = p.expand(p.InstallPaths.DocDir, vars)

	return paths
}

// ExpandedSource returns the Source with template variables expanded.
func (p *Package) ExpandedSource() Source {
	vars := p.baseVars()
	return Source{
		URL:    p.expand(p.Source.URL, vars),
		Git:    p.expand(p.Source.Git, vars),
		Binary: p.expand(p.Source.Binary, vars),
		SHA256: p.Source.SHA256,
		Ref:    p.expand(p.Source.Ref, vars),
		Strip:  p.Source.Strip,
	}
}

// ExpandedSteps returns install steps with template variables expanded.
// srcdir is the path to the extracted/cloned source directory.
func (p *Package) ExpandedSteps(srcdir string) []InstallStep {
	paths := p.ExpandedPaths()
	vars := p.baseVars()
	vars["prefix"] = paths.Prefix
	vars["bindir"] = paths.BinDir
	vars["libdir"] = paths.LibDir
	vars["datadir"] = paths.DataDir
	vars["mandir"] = paths.ManDir
	vars["docdir"] = paths.DocDir
	vars["srcdir"] = srcdir

	var steps []InstallStep
	for _, step := range p.InstallSteps {
		if !step.matchesPlatform() {
			continue
		}
		steps = append(steps, InstallStep{
			Type:      step.Type,
			Command:   p.expand(step.Command, vars),
			WorkDir:   p.expand(step.WorkDir, vars),
			Src:       p.expand(step.Src, vars),
			Dest:      p.expand(step.Dest, vars),
			Path:      p.expand(step.Path, vars),
			Mode:      step.Mode,
			Platforms: step.Platforms,
		})
	}
	return steps
}

func (p *Package) baseVars() map[string]string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	os := runtime.GOOS
	if os == "darwin" {
		os = "apple-darwin"
	}

	return map[string]string{
		"name":    p.Name,
		"version": p.Version,
		"arch":    arch,
		"os":      os,
	}
}

func (p *Package) expand(s string, vars map[string]string) string {
	result := s
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

func (s InstallStep) matchesPlatform() bool {
	if len(s.Platforms) == 0 {
		return true
	}

	current := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, p := range s.Platforms {
		if p == current {
			return true
		}
	}
	return false
}
