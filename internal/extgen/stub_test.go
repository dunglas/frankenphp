package extgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStubGenerator_Generate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stub_generator_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	generator := &Generator{
		BaseName: "test_extension",
		BuildDir: tmpDir,
		functions: []phpFunction{
			{
				name:      "greet",
				signature: "greet(string $name): string",
				params: []phpParameter{
					{name: "name", phpType: "string"},
				},
				returnType: "string",
			},
			{
				name:      "calculate",
				signature: "calculate(int $a, int $b): int",
				params: []phpParameter{
					{name: "a", phpType: "int"},
					{name: "b", phpType: "int"},
				},
				returnType: "int",
			},
		},
		classes: []phpClass{
			{
				name:     "User",
				goStruct: "UserStruct",
			},
		},
		constants: []phpConstant{
			{
				name:    "GLOBAL_CONST",
				value:   "42",
				phpType: "int",
			},
			{
				name:      "USER_STATUS_ACTIVE",
				value:     "1",
				phpType:   "int",
				className: "User",
			},
		},
	}

	stubGen := StubGenerator{generator}
	err = stubGen.generate()
	assert.NoError(t, err, "generate() failed")

	expectedFile := filepath.Join(tmpDir, "test_extension.stub.php")
	_, err = os.Stat(expectedFile)
	assert.False(t, os.IsNotExist(err), "Expected stub file was not created: %s", expectedFile)

	content, err := ReadFile(expectedFile)
	assert.NoError(t, err, "Failed to read generated stub file")

	testStubBasicStructure(t, content)
	testStubFunctions(t, content, generator.functions)
	testStubClasses(t, content, generator.classes)
	testStubConstants(t, content, generator.constants)
}

func TestStubGenerator_BuildContent(t *testing.T) {
	tests := []struct {
		name      string
		functions []phpFunction
		classes   []phpClass
		constants []phpConstant
		contains  []string
	}{
		{
			name:      "empty extension",
			functions: []phpFunction{},
			classes:   []phpClass{},
			constants: []phpConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
			},
		},
		{
			name: "functions only",
			functions: []phpFunction{
				{
					name:      "testFunc",
					signature: "testFunc(string $param): bool",
				},
			},
			classes:   []phpClass{},
			constants: []phpConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"function testFunc(string $param): bool {}",
			},
		},
		{
			name:      "classes only",
			functions: []phpFunction{},
			classes: []phpClass{
				{
					name: "TestClass",
				},
			},
			constants: []phpConstant{},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"class TestClass {",
				"public function __construct() {}",
				"}",
			},
		},
		{
			name:      "constants only",
			functions: []phpFunction{},
			classes:   []phpClass{},
			constants: []phpConstant{
				{
					name:    "GLOBAL_CONST",
					value:   "\"test\"",
					phpType: "string",
				},
			},
			contains: []string{
				"<?php",
				"/** @generate-class-entries */",
				"const GLOBAL_CONST = \"test\";",
			},
		},
		{
			name: "functions and classes",
			functions: []phpFunction{
				{
					name:      "process",
					signature: "process(array $data): array",
				},
			},
			classes: []phpClass{
				{
					name: "Result",
				},
			},
			constants: []phpConstant{},
			contains: []string{
				"function process(array $data): array {}",
				"class Result {",
				"public function __construct() {}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				functions: tt.functions,
				classes:   tt.classes,
				constants: tt.constants,
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated stub content should contain '%s'", expected)
			}
		})
	}
}

func TestStubGenerator_FunctionSignatures(t *testing.T) {
	tests := []struct {
		name     string
		function phpFunction
		expected string
	}{
		{
			name: "simple function",
			function: phpFunction{
				name:      "test",
				signature: "test(): void",
			},
			expected: "function test(): void {}",
		},
		{
			name: "function with parameters",
			function: phpFunction{
				name:      "greet",
				signature: "greet(string $name): string",
			},
			expected: "function greet(string $name): string {}",
		},
		{
			name: "function with nullable return",
			function: phpFunction{
				name:      "findUser",
				signature: "findUser(int $id): ?object",
			},
			expected: "function findUser(int $id): ?object {}",
		},
		{
			name: "complex function signature",
			function: phpFunction{
				name:      "process",
				signature: "process(array $data, ?string $prefix = null, bool $strict = false): ?array",
			},
			expected: "function process(array $data, ?string $prefix = null, bool $strict = false): ?array {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				functions: []phpFunction{tt.function},
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")
			assert.Contains(t, content, tt.expected, "Generated content should contain function signature: %s", tt.expected)
		})
	}
}

func TestStubGenerator_ClassGeneration(t *testing.T) {
	tests := []struct {
		name     string
		class    phpClass
		contains []string
	}{
		{
			name: "simple class",
			class: phpClass{
				name: "SimpleClass",
			},
			contains: []string{
				"class SimpleClass {",
				"public function __construct() {}",
				"}",
			},
		},
		{
			name: "class with no properties",
			class: phpClass{
				name: "EmptyClass",
			},
			contains: []string{
				"class EmptyClass {",
				"public function __construct() {}",
				"}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				classes: []phpClass{tt.class},
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected, "Generated content should contain '%s'", expected)
			}
		})
	}
}

func TestStubGenerator_MultipleItems(t *testing.T) {
	functions := []phpFunction{
		{
			name:      "func1",
			signature: "func1(): void",
		},
		{
			name:      "func2",
			signature: "func2(string $param): bool",
		},
		{
			name:      "func3",
			signature: "func3(int $a, int $b): int",
		},
	}

	classes := []phpClass{
		{
			name: "Class1",
		},
		{
			name: "Class2",
		},
	}

	generator := &Generator{
		functions: functions,
		classes:   classes,
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	for _, fn := range functions {
		expectedFunc := "function " + fn.name
		assert.Contains(t, content, expectedFunc, "Should contain function: %s", expectedFunc)
	}

	for _, class := range classes {
		expectedClass := "class " + class.name
		assert.Contains(t, content, expectedClass, "Should contain class: %s", expectedClass)
	}

	funcPos := strings.Index(content, "function func1")
	classPos := strings.Index(content, "class Class1")

	assert.NotEqual(t, -1, funcPos, "functions should be present")
	assert.NotEqual(t, -1, classPos, "classes should be present")
	assert.Less(t, funcPos, classPos, "functions should appear before classes in the stub file")
}

func TestStubGenerator_ErrorHandling(t *testing.T) {
	generator := &Generator{
		BaseName: "test",
		BuildDir: "/invalid/readonly/path",
		functions: []phpFunction{
			{name: "test", signature: "test(): void"},
		},
	}

	stubGen := StubGenerator{generator}
	err := stubGen.generate()
	assert.Error(t, err, "Expected error when writing to invalid directory")
}

func TestStubGenerator_EmptyContent(t *testing.T) {
	generator := &Generator{
		functions: []phpFunction{},
		classes:   []phpClass{},
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	expectedMinimal := []string{
		"<?php",
		"/** @generate-class-entries */",
	}

	for _, expected := range expectedMinimal {
		assert.Contains(t, content, expected, "Even empty content should contain: %s", expected)
	}

	assert.NotContains(t, content, "function ", "Empty stub should not contain function declarations")
	assert.NotContains(t, content, "class ", "Empty stub should not contain class declarations")
}

func TestStubGenerator_PHPSyntaxValidation(t *testing.T) {
	functions := []phpFunction{
		{
			name:      "complexFunc",
			signature: "complexFunc(?string $name = null, array $options = [], bool $strict = false): ?object",
		},
	}

	classes := []phpClass{
		{
			name: "ComplexClass",
		},
	}

	generator := &Generator{
		functions: functions,
		classes:   classes,
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	syntaxChecks := []struct {
		element string
		reason  string
	}{
		{"<?php", "should start with PHP opening tag"},
		{"{", "should contain opening braces"},
		{"}", "should contain closing braces"},
		{"public", "should use proper visibility"},
		{"function", "should contain function keyword"},
		{"class", "should contain class keyword"},
	}

	for _, check := range syntaxChecks {
		assert.Contains(t, content, check.element, "Generated PHP %s", check.reason)
	}

	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	assert.Equal(t, openBraces, closeBraces, "Unbalanced braces in PHP: %d open, %d close", openBraces, closeBraces)

	assert.Contains(t, content, "function complexFunc(?string $name = null, array $options = [], bool $strict = false): ?object {}", "Complex function signature should be preserved exactly")
}

func TestStubGenerator_ClassConstants(t *testing.T) {
	tests := []struct {
		name      string
		classes   []phpClass
		constants []phpConstant
		contains  []string
	}{
		{
			name: "class with constants",
			classes: []phpClass{
				{name: "MyClass"},
			},
			constants: []phpConstant{
				{
					name:      "STATUS_ACTIVE",
					value:     "1",
					phpType:   "int",
					className: "MyClass",
				},
				{
					name:      "STATUS_INACTIVE",
					value:     "0",
					phpType:   "int",
					className: "MyClass",
				},
			},
			contains: []string{
				"class MyClass {",
				"public const STATUS_ACTIVE = 1;",
				"public const STATUS_INACTIVE = 0;",
				"public function __construct() {}",
			},
		},
		{
			name: "class with iota constants",
			classes: []phpClass{
				{name: "StatusClass"},
			},
			constants: []phpConstant{
				{
					name:      "FIRST",
					value:     "0",
					phpType:   "int",
					isIota:    true,
					className: "StatusClass",
				},
				{
					name:      "SECOND",
					value:     "1",
					phpType:   "int",
					isIota:    true,
					className: "StatusClass",
				},
			},
			contains: []string{
				"class StatusClass {",
				"public const FIRST = UNKNOWN;",
				"public const SECOND = UNKNOWN;",
				"@cvalue FIRST",
				"@cvalue SECOND",
			},
		},
		{
			name: "global and class constants",
			classes: []phpClass{
				{name: "TestClass"},
			},
			constants: []phpConstant{
				{
					name:    "GLOBAL_CONST",
					value:   "\"global\"",
					phpType: "string",
				},
				{
					name:      "CLASS_CONST",
					value:     "42",
					phpType:   "int",
					className: "TestClass",
				},
			},
			contains: []string{
				"const GLOBAL_CONST = \"global\";",
				"class TestClass {",
				"public const CLASS_CONST = 42;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &Generator{
				classes:   tt.classes,
				constants: tt.constants,
			}

			stubGen := StubGenerator{generator}
			content, err := stubGen.buildContent()
			assert.NoError(t, err, "buildContent() failed")

			for _, expected := range tt.contains {
				assert.Contains(t, content, expected)
			}
		})
	}
}

func TestStubGenerator_FileStructure(t *testing.T) {
	generator := &Generator{
		functions: []phpFunction{
			{name: "testFunc", signature: "testFunc(): void"},
		},
		classes: []phpClass{
			{
				name: "TestClass",
			},
		},
	}

	stubGen := StubGenerator{generator}
	content, err := stubGen.buildContent()
	assert.NoError(t, err, "buildContent() failed")

	lines := strings.Split(content, "\n")

	assert.GreaterOrEqual(t, len(lines), 3, "Stub file should have multiple lines")
	assert.Equal(t, "<?php", strings.TrimSpace(lines[0]), "First line should be <?php opening tag")

	foundGenerateDirective := false
	for _, line := range lines {
		if strings.Contains(line, "@generate-class-entries") {
			foundGenerateDirective = true
			break
		}
	}

	assert.True(t, foundGenerateDirective, "Should contain @generate-class-entries directive")
	assert.Contains(t, strings.Join(lines, "\n"), "\n\n", "Should have proper spacing between sections")
}

func testStubBasicStructure(t *testing.T, content string) {
	requiredElements := []string{
		"<?php",
		"/** @generate-class-entries */",
	}

	for _, element := range requiredElements {
		assert.Contains(t, content, element, "Stub file should contain: %s", element)
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		assert.Equal(t, "<?php", strings.TrimSpace(lines[0]), "Stub file should start with <?php")
	}
}

func testStubFunctions(t *testing.T, content string, functions []phpFunction) {
	for _, fn := range functions {
		expectedFunc := "function " + fn.signature + " {}"
		assert.Contains(t, content, expectedFunc, "Stub should contain function: %s", expectedFunc)
	}
}

func testStubClasses(t *testing.T, content string, classes []phpClass) {
	for _, class := range classes {
		expectedClass := "class " + class.name + " {"
		assert.Contains(t, content, expectedClass, "Stub should contain class: %s", expectedClass)

		expectedConstructor := "public function __construct() {}"
		assert.Contains(t, content, expectedConstructor, "Class %s should have constructor", class.name)

		assert.Contains(t, content, "}", "Class %s should be properly closed", class.name)
	}
}

func testStubConstants(t *testing.T, content string, constants []phpConstant) {
	for _, constant := range constants {
		if constant.className == "" {
			if constant.isIota {
				expectedConst := "const " + constant.name + " = UNKNOWN;"
				assert.Contains(t, content, expectedConst, "Stub should contain iota constant: %s", expectedConst)
			} else {
				expectedConst := "const " + constant.name + " = " + constant.value + ";"
				assert.Contains(t, content, expectedConst, "Stub should contain constant: %s", expectedConst)
			}
		} else {
			if constant.isIota {
				expectedConst := "public const " + constant.name + " = UNKNOWN;"
				assert.Contains(t, content, expectedConst, "Stub should contain class iota constant: %s", expectedConst)
			} else {
				expectedConst := "public const " + constant.name + " = " + constant.value + ";"
				assert.Contains(t, content, expectedConst, "Stub should contain class constant: %s", expectedConst)
			}
		}
	}
}
