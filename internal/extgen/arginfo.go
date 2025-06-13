package extgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type arginfoGenerator struct {
	generator *Generator
}

func (ag *arginfoGenerator) generate() error {
	genStubPath := os.Getenv("GEN_STUB_SCRIPT")
	if genStubPath == "" {
		genStubPath = "/usr/local/src/php/build/gen_stub.php"
	}

	if _, err := os.Stat(genStubPath); err != nil {
		return fmt.Errorf(`the PHP "gen_stub.php" file couldn't be found under %q, you can set the "GEN_STUB_SCRIPT" environement variable to set a custom location`, genStubPath)
	}

	stubFile := ag.generator.BaseName + ".stub.php"
	cmd := exec.Command("php", genStubPath, filepath.Join(ag.generator.BuildDir, stubFile))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running gen_stub script: %w", err)
	}

	return ag.fixArginfoFile(stubFile)
}

func (ag *arginfoGenerator) fixArginfoFile(stubFile string) error {
	arginfoFile := strings.TrimSuffix(stubFile, ".stub.php") + "_arginfo.h"
	arginfoPath := filepath.Join(ag.generator.BuildDir, arginfoFile)

	content, err := ReadFile(arginfoPath)
	if err != nil {
		return fmt.Errorf("reading arginfo file: %w", err)
	}

	// TODO: Fix the zend_register_internal_class_with_flags issue
	fixedContent := strings.ReplaceAll(content,
		"zend_register_internal_class_with_flags(&ce, NULL, 0)",
		"zend_register_internal_class(&ce)")

	return WriteFile(arginfoPath, fixedContent)
}
