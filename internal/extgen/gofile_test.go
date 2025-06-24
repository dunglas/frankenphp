package extgen

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoFileGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import (
	"fmt"
	"strings"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: greet(name string): string
func greet(name *go_string) *go_value {
	return types.String("Hello " + CStringToGoString(name))
}

//export_php: calculate(a int, b int): int
func calculate(a long, b long) *go_value {
	result := a + b
	return types.Int(result)
}

func internalHelper(data string) string {
	return strings.ToUpper(data)
}

func anotherHelper() {
	fmt.Println("Internal helper")
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	generator := &Generator{
		BaseName:   "test",
		SourceFile: sourceFile,
		BuildDir:   tmpDir,
		Functions: []phpFunction{
			{
				Name:       "greet",
				ReturnType: "string",
				goFunction: `func greet(name *go_string) *go_value {
	return types.String("Hello " + CStringToGoString(name))
}`,
			},
			{
				Name:       "calculate",
				ReturnType: "int",
				goFunction: `func calculate(a long, b long) *go_value {
	result := a + b
	return types.Int(result)
}`,
			},
		},
	}

	goGen := GoFileGenerator{generator}
	require.NoError(t, goGen.generate())

	expectedFile := filepath.Join(tmpDir, "test.go")
	require.FileExists(t, expectedFile)

	content, err := ReadFile(expectedFile)
	require.NoError(t, err)

	testGoFileBasicStructure(t, content, "test")
	testGoFileImports(t, content)
	testGoFileExportedFunctions(t, content, generator.Functions)
	testGoFileInternalFunctions(t, content)
}

func TestGoFileGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name        string
		baseName    string
		sourceFile  string
		functions   []phpFunction
		contains    []string
		notContains []string
	}{
		{
			name:     "simple extension",
			baseName: "simple",
			sourceFile: createTempSourceFile(t, `package main

//export_php: test(): void
func test() {
	// simple function
}`),
			functions: []phpFunction{
				{
					Name:       "test",
					ReturnType: "void",
					goFunction: "func test() {\n\t// simple function\n}",
				},
			},
			contains: []string{
				"package simple",
				`#include "simple.h"`,
				"import \"C\"",
				"func init()",
				"frankenphp.RegisterExtension(",
				"//export test",
				"func test()",
			},
		},
		{
			name:     "extension with complex imports",
			baseName: "complex",
			sourceFile: createTempSourceFile(t, `package main

import (
	"fmt"
	"strings"
	"encoding/json"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: process(data string): string
func process(data *go_string) *go_value {
	return types.String(fmt.Sprintf("processed: %s", CStringToGoString(data)))
}`),
			functions: []phpFunction{
				{
					Name:       "process",
					ReturnType: "string",
					goFunction: `func process(data *go_string) *go_value {
	return String(fmt.Sprintf("processed: %s", CStringToGoString(data)))
}`,
				},
			},
			contains: []string{
				"package complex",
				`import "fmt"`,
				`import "strings"`,
				`import "encoding/json"`,
				"//export process",
				`import "C"`,
			},
		},
		{
			name:     "extension with internal functions",
			baseName: "internal",
			sourceFile: createTempSourceFile(t, `package main

//export_php: publicFunc(): void
func publicFunc() {}

func internalFunc1() string {
	return "internal"
}

func internalFunc2(data string) {
	// process data internally
}`),
			functions: []phpFunction{
				{
					Name:       "publicFunc",
					ReturnType: "void",
					goFunction: "func publicFunc() {}",
				},
			},
			contains: []string{
				"func internalFunc1() string",
				"func internalFunc2(data string)",
				"//export publicFunc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:   tt.baseName,
				SourceFile: tt.sourceFile,
				Functions:  tt.functions,
			}

			goGen := GoFileGenerator{generator}
			content, err := goGen.buildContent()
			require.NoError(t, err)

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated Go content should contain '%s'", expected)
			}
		})
	}
}

func TestGoFileGenerator_PackageNameSanitization(t *testing.T) {
	tests := []struct {
		baseName        string
		expectedPackage string
	}{
		{"simple", "simple"},
		{"my-extension", "my_extension"},
		{"ext.with.dots", "ext_with_dots"},
		{"123invalid", "_123invalid"},
		{"valid_name", "valid_name"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			sourceFile := createTempSourceFile(t, "package main\n//export_php: test(): void\nfunc test() {}")

			generator := &Generator{
				BaseName:   tt.baseName,
				SourceFile: sourceFile,
				Functions: []phpFunction{
					{Name: "test", ReturnType: "void", goFunction: "func test() {}"},
				},
			}

			goGen := GoFileGenerator{generator}
			content, err := goGen.buildContent()
			require.NoError(t, err)

			expectedPackage := "package " + tt.expectedPackage
			assert.Contains(t, content, expectedPackage, "Generated content should contain '%s'", expectedPackage)
		})
	}
}

func TestGoFileGenerator_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		expectErr  bool
	}{
		{
			name:       "nonexistent file",
			sourceFile: "/nonexistent/file.go",
			expectErr:  true,
		},
		{
			name:       "invalid Go syntax",
			sourceFile: createTempSourceFile(t, "invalid go syntax here"),
			expectErr:  true,
		},
		{
			name:       "valid file",
			sourceFile: createTempSourceFile(t, "package main\nfunc test() {}"),
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:   "test",
				SourceFile: tt.sourceFile,
			}

			goGen := GoFileGenerator{generator}
			_, err := goGen.buildContent()

			if tt.expectErr {
				assert.Error(t, err, "Expected error but got none")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestGoFileGenerator_ImportFiltering(t *testing.T) {
	sourceContent := `package main

import (
	"C"
	"fmt"
	"strings"
	"github.com/dunglas/frankenphp/internal/extensions/types"
	"github.com/other/package"
	originalPkg "github.com/test/original"
)

//export_php: test(): void
func test() {}`

	sourceFile := createTempSourceFile(t, sourceContent)

	generator := &Generator{
		BaseName:   "importtest",
		SourceFile: sourceFile,
		Functions: []phpFunction{
			{Name: "test", ReturnType: "void", goFunction: "func test() {}"},
		},
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)

	expectedImports := []string{
		`import "fmt"`,
		`import "strings"`,
		`import "github.com/other/package"`,
	}

	for _, imp := range expectedImports {
		assert.Contains(t, content, imp, "Generated content should contain import: %s", imp)
	}

	forbiddenImports := []string{
		`import "C"`,
	}

	cImportCount := strings.Count(content, `import "C"`)
	assert.Equal(t, 1, cImportCount, "Expected exactly 1 occurrence of 'import \"C\"'")

	for _, imp := range forbiddenImports[1:] {
		assert.NotContains(t, content, imp, "Generated content should NOT contain import: %s", imp)
	}
}

func TestGoFileGenerator_ComplexScenario(t *testing.T) {
	sourceContent := `package example

import (
	"fmt"
	"strings"
	"encoding/json"
	"github.com/dunglas/frankenphp/internal/extensions/types"
)

//export_php: processData(input string, options array): array
func processData(input *go_string, options *go_nullable) *go_value {
	data := CStringToGoString(input)
	processed := internalProcess(data)
	return types.Array([]interface{}{processed})
}

//export_php: validateInput(data string): bool
func validateInput(data *go_string) *go_value {
	input := CStringToGoString(data)
	isValid := len(input) > 0 && validateFormat(input)
	return types.Bool(isValid)
}

func internalProcess(data string) string {
	return strings.ToUpper(data)
}

func validateFormat(input string) bool {
	return !strings.Contains(input, "invalid")
}

func jsonHelper(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func debugPrint(msg string) {
	fmt.Printf("DEBUG: %s\n", msg)
}`

	sourceFile := createTempSourceFile(t, sourceContent)

	functions := []phpFunction{
		{
			Name:       "processData",
			ReturnType: "array",
			goFunction: `func processData(input *go_string, options *go_nullable) *go_value {
	data := CStringToGoString(input)
	processed := internalProcess(data)
	return Array([]interface{}{processed})
}`,
		},
		{
			Name:       "validateInput",
			ReturnType: "bool",
			goFunction: `func validateInput(data *go_string) *go_value {
	input := CStringToGoString(data)
	isValid := len(input) > 0 && validateFormat(input)
	return Bool(isValid)
}`,
		},
	}

	generator := &Generator{
		BaseName:   "complex-example",
		SourceFile: sourceFile,
		Functions:  functions,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)
	assert.Contains(t, content, "package complex_example", "Package name should be sanitized")

	internalFuncs := []string{
		"func internalProcess(data string) string",
		"func validateFormat(input string) bool",
		"func jsonHelper(data interface{}) ([]byte, error)",
		"func debugPrint(msg string)",
	}

	for _, fn := range internalFuncs {
		assert.Contains(t, content, fn, "Generated content should contain internal function: %s", fn)
	}

	for _, fn := range functions {
		exportDirective := "//export " + fn.Name
		assert.Contains(t, content, exportDirective, "Generated content should contain export directive: %s", exportDirective)
	}

	assert.False(t, strings.Contains(content, "types.Array") || strings.Contains(content, "types.Bool"), "Types should be replaced (types.* should not appear)")
	assert.True(t, strings.Contains(content, "return Array(") && strings.Contains(content, "return Bool("), "Replaced types should appear without types prefix")
}

func TestGoFileGenerator_MethodWrapperWithNullableParams(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := `package main

import "fmt"

//export_php:class TestClass
type TestStruct struct {
	name string
}

//export_php:method TestClass::processData(string $name, ?int $count, ?bool $enabled): string
func (ts *TestStruct) ProcessData(name string, count *int64, enabled *bool) string {
	result := fmt.Sprintf("name=%s", name)
	if count != nil {
		result += fmt.Sprintf(", count=%d", *count)
	}
	if enabled != nil {
		result += fmt.Sprintf(", enabled=%t", *enabled)
	}
	return result
}`

	sourceFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(sourceFile, []byte(sourceContent), 0644))

	methods := []phpClassMethod{
		{
			Name:       "ProcessData",
			PhpName:    "processData",
			ClassName:  "TestClass",
			Signature:  "processData(string $name, ?int $count, ?bool $enabled): string",
			ReturnType: "string",
			Params: []phpParameter{
				{Name: "name", PhpType: "string", IsNullable: false},
				{Name: "count", PhpType: "int", IsNullable: true},
				{Name: "enabled", PhpType: "bool", IsNullable: true},
			},
			goFunction: `func (ts *TestStruct) ProcessData(name string, count *int64, enabled *bool) string {
	result := fmt.Sprintf("name=%s", name)
	if count != nil {
		result += fmt.Sprintf(", count=%d", *count)
	}
	if enabled != nil {
		result += fmt.Sprintf(", enabled=%t", *enabled)
	}
	return result
}`,
		},
	}

	classes := []phpClass{
		{
			Name:     "TestClass",
			GoStruct: "TestStruct",
			Methods:  methods,
		},
	}

	generator := &Generator{
		BaseName:   "nullable_test",
		SourceFile: sourceFile,
		Classes:    classes,
		BuildDir:   tmpDir,
	}

	goGen := GoFileGenerator{generator}
	content, err := goGen.buildContent()
	require.NoError(t, err)

	expectedWrapperSignature := "func ProcessData_wrapper(handle C.uintptr_t, name *C.zend_string, count *int64, enabled *bool)"
	assert.Contains(t, content, expectedWrapperSignature, "Generated content should contain wrapper with nullable pointer types: %s", expectedWrapperSignature)

	expectedCall := "structObj.ProcessData(name, count, enabled)"
	assert.Contains(t, content, expectedCall, "Generated content should contain correct method call: %s", expectedCall)

	exportDirective := "//export ProcessData_wrapper"
	assert.Contains(t, content, exportDirective, "Generated content should contain export directive: %s", exportDirective)
}

func createTempSourceFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "source.go")

	require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

	return tmpFile
}

func testGoFileBasicStructure(t *testing.T, content, baseName string) {
	requiredElements := []string{
		"package " + SanitizePackageName(baseName),
		"/*",
		"#include <stdlib.h>",
		`#include "` + baseName + `.h"`,
		"*/",
		`import "C"`,
		"func init() {",
		"frankenphp.RegisterExtension(",
		"}",
	}

	for _, element := range requiredElements {
		assert.Contains(t, content, element, "Go file should contain: %s", element)
	}
}

func testGoFileImports(t *testing.T, content string) {
	cImportCount := strings.Count(content, `import "C"`)
	assert.Equal(t, 1, cImportCount, "Expected exactly 1 C import")
}

func testGoFileExportedFunctions(t *testing.T, content string, functions []phpFunction) {
	for _, fn := range functions {
		exportDirective := "//export " + fn.Name
		assert.Contains(t, content, exportDirective, "Go file should contain export directive: %s", exportDirective)

		funcStart := "func " + fn.Name + "("
		assert.Contains(t, content, funcStart, "Go file should contain function definition: %s", funcStart)
	}
}

func testGoFileInternalFunctions(t *testing.T, content string) {
	internalIndicators := []string{
		"func internalHelper",
		"func anotherHelper",
	}

	foundInternal := false
	for _, indicator := range internalIndicators {
		if strings.Contains(content, indicator) {
			foundInternal = true

			break
		}
	}

	if !foundInternal {
		t.Log("No internal functions found (this may be expected)")
	}
}
