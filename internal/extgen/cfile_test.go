package extgen

import (
	"github.com/stretchr/testify/require"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCFileGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	generator := &Generator{
		BaseName: "test_extension",
		BuildDir: tmpDir,
		Functions: []phpFunction{
			{
				Name:       "simpleFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "input", PhpType: "string"},
				},
			},
			{
				Name:       "complexFunction",
				ReturnType: "array",
				Params: []phpParameter{
					{Name: "data", PhpType: "string"},
					{Name: "count", PhpType: "int", IsNullable: true},
					{Name: "options", PhpType: "array", HasDefault: true, DefaultValue: "[]"},
				},
			},
		},
		Classes: []phpClass{
			{
				Name:     "TestClass",
				GoStruct: "TestStruct",
				Properties: []phpClassProperty{
					{Name: "id", PhpType: "int"},
					{Name: "name", PhpType: "string"},
				},
			},
		},
	}

	cGen := cFileGenerator{generator}
	require.NoError(t, cGen.generate())

	expectedFile := filepath.Join(tmpDir, "test_extension.c")
	require.FileExists(t, expectedFile, "Expected C file was not created: %s", expectedFile)

	content, err := ReadFile(expectedFile)
	require.NoError(t, err)

	testCFileBasicStructure(t, content, "test_extension")
	testCFileFunctions(t, content, generator.Functions)
	testCFileClasses(t, content, generator.Classes)
}

func TestCFileGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name        string
		baseName    string
		functions   []phpFunction
		classes     []phpClass
		contains    []string
		notContains []string
	}{
		{
			name:     "empty extension",
			baseName: "empty",
			contains: []string{
				"#include <php.h>",
				"#include <Zend/zend_API.h>",
				`#include "empty.h"`,
				"PHP_MINIT_FUNCTION(empty)",
				"empty_module_entry",
				"return SUCCESS;",
			},
		},
		{
			name:     "extension with functions only",
			baseName: "func_only",
			functions: []phpFunction{
				{Name: "testFunc", ReturnType: "string"},
			},
			contains: []string{
				"PHP_FUNCTION(testFunc)",
				`#include "func_only.h"`,
				"func_only_module_entry",
				"PHP_MINIT_FUNCTION(func_only)",
			},
		},
		{
			name:     "extension with classes only",
			baseName: "class_only",
			classes: []phpClass{
				{Name: "MyClass", GoStruct: "MyStruct"},
			},
			contains: []string{
				"register_all_classes()",
				"register_class_MyClass();",
				"PHP_METHOD(MyClass, __construct)",
				`#include "class_only.h"`,
			},
		},
		{
			name:     "extension with functions and classes",
			baseName: "full",
			functions: []phpFunction{
				{Name: "doSomething", ReturnType: "void"},
			},
			classes: []phpClass{
				{Name: "FullClass", GoStruct: "FullStruct"},
			},
			contains: []string{
				"PHP_FUNCTION(doSomething)",
				"PHP_METHOD(FullClass, __construct)",
				"register_all_classes()",
				"register_class_FullClass();",
				`#include "full.h"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:  tt.baseName,
				Functions: tt.functions,
				Classes:   tt.classes,
			}

			cGen := cFileGenerator{generator}
			content, err := cGen.buildContent()
			require.NoError(t, err)

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated C content should contain '%s'", expected)
			}
		})
	}
}

func TestCFileGenerator_GetTemplateContent(t *testing.T) {
	tests := []struct {
		name        string
		baseName    string
		classes     []phpClass
		contains    []string
		notContains []string
	}{
		{
			name:     "extension without classes",
			baseName: "myext",
			contains: []string{
				`#include "myext.h"`,
				`#include "myext_arginfo.h"`,
				"PHP_MINIT_FUNCTION(myext)",
				"myext_module_entry",
				"return SUCCESS;",
			},
		},
		{
			name:     "extension with classes",
			baseName: "complex_name",
			classes: []phpClass{
				{Name: "TestClass", GoStruct: "TestStruct"},
				{Name: "AnotherClass", GoStruct: "AnotherStruct"},
			},
			contains: []string{
				`#include "complex_name.h"`,
				`#include "complex_name_arginfo.h"`,
				"PHP_MINIT_FUNCTION(complex_name)",
				"complex_name_module_entry",
				"register_all_classes()",
				"register_class_TestClass();",
				"register_class_AnotherClass();",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName: tt.baseName,
				Classes:  tt.classes,
			}
			cGen := cFileGenerator{generator}
			content, err := cGen.getTemplateContent()
			require.NoError(t, err)

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Template content should contain '%s'", expected)
			}

			for _, notExpected := range tt.notContains {
				assert.NotContains(t, content, notExpected, "Template content should NOT contain '%s'", notExpected)
			}
		})
	}
}

func TestCFileIntegrationWithGenerators(t *testing.T) {
	tmpDir := t.TempDir()

	functions := []phpFunction{
		{
			Name:             "processData",
			ReturnType:       "array",
			IsReturnNullable: true,
			Params: []phpParameter{
				{Name: "input", PhpType: "string"},
				{Name: "options", PhpType: "array", HasDefault: true, DefaultValue: "[]"},
				{Name: "callback", PhpType: "object", IsNullable: true},
			},
		},
		{
			Name:       "validateInput",
			ReturnType: "bool",
			Params: []phpParameter{
				{Name: "data", PhpType: "string", IsNullable: true},
				{Name: "strict", PhpType: "bool", HasDefault: true, DefaultValue: "false"},
			},
		},
	}

	classes := []phpClass{
		{
			Name:     "DataProcessor",
			GoStruct: "DataProcessorStruct",
			Properties: []phpClassProperty{
				{Name: "mode", PhpType: "string"},
				{Name: "timeout", PhpType: "int", IsNullable: true},
				{Name: "options", PhpType: "array"},
			},
		},
		{
			Name:     "Result",
			GoStruct: "ResultStruct",
			Properties: []phpClassProperty{
				{Name: "success", PhpType: "bool"},
				{Name: "data", PhpType: "mixed", IsNullable: true},
				{Name: "errors", PhpType: "array"},
			},
		},
	}

	generator := &Generator{
		BaseName:  "integration_test",
		BuildDir:  tmpDir,
		Functions: functions,
		Classes:   classes,
	}

	cGen := cFileGenerator{generator}
	require.NoError(t, cGen.generate())

	content, err := ReadFile(filepath.Join(tmpDir, "integration_test.c"))
	require.NoError(t, err)

	for _, fn := range functions {
		expectedFunc := "PHP_FUNCTION(" + fn.Name + ")"
		assert.Contains(t, content, expectedFunc, "Generated C file should contain function: %s", expectedFunc)
	}

	for _, class := range classes {
		expectedMethod := "PHP_METHOD(" + class.Name + ", __construct)"
		assert.Contains(t, content, expectedMethod, "Generated C file should contain class method: %s", expectedMethod)
	}

	assert.Contains(t, content, "register_all_classes()", "Generated C file should contain class registration call")
	assert.Contains(t, content, "integration_test_module_entry", "Generated C file should contain integration_test_module_entry")
}

func TestCFileErrorHandling(t *testing.T) {
	// Test with invalid build directory
	generator := &Generator{
		BaseName: "test",
		BuildDir: "/invalid/readonly/path",
		Functions: []phpFunction{
			{Name: "test", ReturnType: "void"},
		},
	}

	cGen := cFileGenerator{generator}
	err := cGen.generate()
	assert.Error(t, err, "Expected error when writing to invalid directory")
}

func TestCFileSpecialCharacters(t *testing.T) {
	tests := []struct {
		baseName string
		expected string
	}{
		{"simple", "simple"},
		{"my_extension", "my_extension"},
		{"ext-with-dashes", "ext-with-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.baseName, func(t *testing.T) {
			generator := &Generator{
				BaseName: tt.baseName,
				Functions: []phpFunction{
					{Name: "test", ReturnType: "void"},
				},
			}

			cGen := cFileGenerator{generator}
			content, err := cGen.buildContent()
			require.NoError(t, err)

			expectedInclude := "#include \"" + tt.expected + ".h\""
			assert.Contains(t, content, expectedInclude, "Content should contain include: %s", expectedInclude)
		})
	}
}

func testCFileBasicStructure(t *testing.T, content, baseName string) {
	requiredElements := []string{
		"#include <php.h>",
		"#include <Zend/zend_API.h>",
		`#include "_cgo_export.h"`,
		`#include "` + baseName + `.h"`,
		`#include "` + baseName + `_arginfo.h"`,
		"PHP_MINIT_FUNCTION(" + baseName + ")",
		baseName + "_module_entry",
	}

	for _, element := range requiredElements {
		assert.Contains(t, content, element, "C file should contain: %s", element)
	}
}

func testCFileFunctions(t *testing.T, content string, functions []phpFunction) {
	for _, fn := range functions {
		phpFunc := "PHP_FUNCTION(" + fn.Name + ")"
		assert.Contains(t, content, phpFunc, "C file should contain function declaration: %s", phpFunc)
	}
}

func testCFileClasses(t *testing.T, content string, classes []phpClass) {
	if len(classes) == 0 {
		// Si pas de classes, ne devrait pas contenir register_all_classes
		assert.NotContains(t, content, "register_all_classes()", "C file should NOT contain register_all_classes call when no classes")
		return
	}

	assert.Contains(t, content, "void register_all_classes() {", "C file should contain register_all_classes function")
	assert.Contains(t, content, "register_all_classes();", "C file should contain register_all_classes call in MINIT")

	for _, class := range classes {
		expectedCall := "register_class_" + class.Name + "();"
		assert.Contains(t, content, expectedCall, "C file should contain class registration call: %s", expectedCall)

		constructor := "PHP_METHOD(" + class.Name + ", __construct)"
		assert.Contains(t, content, constructor, "C file should contain constructor: %s", constructor)
	}
}

func TestCFileContentValidation(t *testing.T) {
	generator := &Generator{
		BaseName: "syntax_test",
		Functions: []phpFunction{
			{
				Name:       "testFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "param", PhpType: "string"},
				},
			},
		},
		Classes: []phpClass{
			{Name: "TestClass", GoStruct: "TestStruct"},
		},
	}

	cGen := cFileGenerator{generator}
	content, err := cGen.buildContent()
	require.NoError(t, err)

	syntaxElements := []string{
		"{", "}", "(", ")", ";",
		"static", "void", "int",
		"#include",
	}

	for _, element := range syntaxElements {
		assert.Contains(t, content, element, "Generated C content should contain basic C syntax: %s", element)
	}

	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")

	assert.Equal(t, openBraces, closeBraces, "Unbalanced braces in generated C code: %d open, %d close", openBraces, closeBraces)
	assert.False(t, strings.Contains(content, ";;"), "Generated C code contains double semicolons")
	assert.False(t, strings.Contains(content, "{{") || strings.Contains(content, "}}"), "Generated C code contains unresolved template syntax")
}

func TestCFileConstants(t *testing.T) {
	tests := []struct {
		name      string
		baseName  string
		constants []phpConstant
		classes   []phpClass
		contains  []string
	}{
		{
			name:     "global constants only",
			baseName: "const_test",
			constants: []phpConstant{
				{
					Name:    "GLOBAL_INT",
					Value:   "42",
					PhpType: "int",
				},
				{
					Name:    "GLOBAL_STRING",
					Value:   `"test"`,
					PhpType: "string",
				},
			},
			contains: []string{
				"REGISTER_LONG_CONSTANT(\"GLOBAL_INT\", 42, CONST_CS | CONST_PERSISTENT);",
				"REGISTER_STRING_CONSTANT(\"GLOBAL_STRING\", \"test\", CONST_CS | CONST_PERSISTENT);",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				BaseName:  tt.baseName,
				Constants: tt.constants,
				Classes:   tt.classes,
			}

			cGen := cFileGenerator{generator}
			content, err := cGen.buildContent()
			require.NoError(t, err)

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated C content should contain '%s'", expected)
			}
		})
	}
}

func TestCFileTemplateErrorHandling(t *testing.T) {
	generator := &Generator{
		BaseName: "error_test",
	}

	cGen := cFileGenerator{generator}

	_, err := cGen.getTemplateContent()
	assert.NoError(t, err, "getTemplateContent() should not fail with valid template")
}
