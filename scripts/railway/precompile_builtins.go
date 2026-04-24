package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Precompile only the builtin templates that are actively exposed in the
// wallet quick-deploy UX or required by those flows (dex_factory -> dex_pair).
// Custom source compilation still uses /contract/compile-plugin at runtime,
// and direct .so upload deploy remains unchanged.
var builtinTemplates = []string{
	"bridge_token",
	"dao_treasury",
	"dex_factory",
	"dex_pair",
	"dex_swap",
	"lending_pool",
	"lqd20",
	"nft_collection",
}

func main() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fail(err)
	}

	buildsDir := filepath.Join(projectRoot, "_plugin_builds")
	outputDir := filepath.Join(projectRoot, "bin", "builtins")
	if err := os.MkdirAll(buildsDir, 0o755); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fail(err)
	}

	for _, name := range builtinTemplates {
		if err := compileBuiltin(projectRoot, buildsDir, outputDir, name); err != nil {
			fail(err)
		}
	}
}

func compileBuiltin(projectRoot, buildsDir, outputDir, name string) error {
	srcFile := filepath.Join(projectRoot, "contract", name+".go")
	srcBytes, err := os.ReadFile(srcFile)
	if err != nil {
		return fmt.Errorf("read builtin %s: %w", name, err)
	}

	source := string(srcBytes)
	source = strings.Replace(source, "//go:build ignore\n", "", 1)
	source = strings.Replace(source, "// +build ignore\n", "", 1)

	tmpDir, err := os.MkdirTemp(buildsDir, "prebuilt_"+name+"_")
	if err != nil {
		return fmt.Errorf("mkdir temp for %s: %w", name, err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "contract.go"), []byte(source), 0o644); err != nil {
		return fmt.Errorf("write source for %s: %w", name, err)
	}

	relPkg := "./" + strings.TrimPrefix(filepath.ToSlash(tmpDir), filepath.ToSlash(projectRoot)+"/")
	outPath := filepath.Join(outputDir, name+".so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outPath, relPkg)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compile builtin %s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
