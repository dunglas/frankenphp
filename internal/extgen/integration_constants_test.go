package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstantsIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

//export_php:const
const STATUS_OK = iota

//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const: function test(): void
func Test() {
    // Implementation
}

func main() {}
`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	generator := &Generator{
		BaseName:   "testext",
		SourceFile: testFile,
		BuildDir:   filepath.Join(tmpDir, "build"),
	}

	err = generator.parseSource()
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	assert.Len(t, generator.Constants, 2, "Expected 2 constants")

	expectedConstants := map[string]struct {
		Value  string
		IsIota bool
	}{
		"STATUS_OK":       {"0", true},
		"MAX_CONNECTIONS": {"100", false},
	}

	for _, constant := range generator.Constants {
		expected, exists := expectedConstants[constant.Name]
		assert.True(t, exists, "Unexpected constant: %s", constant.Name)
		if !exists {
			continue
		}

		assert.Equal(t, expected.Value, constant.Value, "Constant %s: value mismatch", constant.Name)
		assert.Equal(t, expected.IsIota, constant.IsIota, "Constant %s: isIota mismatch", constant.Name)
	}

	err = generator.setupBuildDirectory()
	if err != nil {
		t.Fatalf("Failed to setup build directory: %v", err)
	}

	err = generator.generateStubFile()
	if err != nil {
		t.Fatalf("Failed to generate stub file: %v", err)
	}

	stubPath := filepath.Join(generator.BuildDir, generator.BaseName+".stub.php")
	stubContent, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("Failed to read stub file: %v", err)
	}

	stubStr := string(stubContent)

	assert.Contains(t, stubStr, "* @cvalue", "Stub does not contain @cvalue annotation for iota constant")
	assert.Contains(t, stubStr, "const STATUS_OK = UNKNOWN;", "Stub does not contain STATUS_OK constant with UNKNOWN value")
	assert.Contains(t, stubStr, "const MAX_CONNECTIONS = 100;", "Stub does not contain MAX_CONNECTIONS constant with explicit value")

	err = generator.generateCFile()
	if err != nil {
		t.Fatalf("Failed to generate C file: %v", err)
	}

	cPath := filepath.Join(generator.BuildDir, generator.BaseName+".c")
	cContent, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("Failed to read C file: %v", err)
	}

	cStr := string(cContent)

	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("STATUS_OK", STATUS_OK, CONST_CS | CONST_PERSISTENT);`, "C file does not contain STATUS_OK registration")
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("MAX_CONNECTIONS", 100, CONST_CS | CONST_PERSISTENT);`, "C file does not contain MAX_CONNECTIONS registration")
}

func TestConstantsIntegrationOctal(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

//export_php:const
const FILE_PERM = 0o755

//export_php:const
const OTHER_PERM = 0o644

//export_php:const
const REGULAR_INT = 42

func main() {}
`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	generator := &Generator{
		BaseName:   "octalstest",
		SourceFile: testFile,
		BuildDir:   filepath.Join(tmpDir, "build"),
	}

	err = generator.parseSource()
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	assert.Len(t, generator.Constants, 3, "Expected 3 constants")

	// Verify CValue conversion
	for _, constant := range generator.Constants {
		switch constant.Name {
		case "FILE_PERM":
			assert.Equal(t, "0o755", constant.Value, "FILE_PERM value mismatch")
			assert.Equal(t, "493", constant.CValue(), "FILE_PERM CValue mismatch")
		case "OTHER_PERM":
			assert.Equal(t, "0o644", constant.Value, "OTHER_PERM value mismatch")
			assert.Equal(t, "420", constant.CValue(), "OTHER_PERM CValue mismatch")
		case "REGULAR_INT":
			assert.Equal(t, "42", constant.Value, "REGULAR_INT value mismatch")
			assert.Equal(t, "42", constant.CValue(), "REGULAR_INT CValue mismatch")
		}
	}

	err = generator.setupBuildDirectory()
	if err != nil {
		t.Fatalf("Failed to setup build directory: %v", err)
	}

	// Test C file generation
	err = generator.generateCFile()
	if err != nil {
		t.Fatalf("Failed to generate C file: %v", err)
	}

	cPath := filepath.Join(generator.BuildDir, generator.BaseName+".c")
	cContent, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("Failed to read C file: %v", err)
	}

	cStr := string(cContent)

	// Verify C file uses decimal values for octal constants
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("FILE_PERM", 493, CONST_CS | CONST_PERSISTENT);`, "C file does not contain FILE_PERM registration with decimal value 493")
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("OTHER_PERM", 420, CONST_CS | CONST_PERSISTENT);`, "C file does not contain OTHER_PERM registration with decimal value 420")
	assert.Contains(t, cStr, `REGISTER_LONG_CONSTANT("REGULAR_INT", 42, CONST_CS | CONST_PERSISTENT);`, "C file does not contain REGULAR_INT registration with value 42")

	// Test header file generation
	err = generator.generateHeaderFile()
	if err != nil {
		t.Fatalf("Failed to generate header file: %v", err)
	}

	hPath := filepath.Join(generator.BuildDir, generator.BaseName+".h")
	hContent, err := os.ReadFile(hPath)
	if err != nil {
		t.Fatalf("Failed to read header file: %v", err)
	}

	hStr := string(hContent)

	// Verify header file uses decimal values for octal constants in #define
	assert.Contains(t, hStr, "#define FILE_PERM 493", "Header file does not contain FILE_PERM #define with decimal value 493")
	assert.Contains(t, hStr, "#define OTHER_PERM 420", "Header file does not contain OTHER_PERM #define with decimal value 420")
	assert.Contains(t, hStr, "#define REGULAR_INT 42", "Header file does not contain REGULAR_INT #define with value 42")
}
