package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocumentationGenerator_Generate(t *testing.T) {
	tests := []struct {
		name        string
		generator   *Generator
		expectError bool
	}{
		{
			name: "simple extension with functions",
			generator: &Generator{
				BaseName: "testextension",
				BuildDir: "",
				functions: []phpFunction{
					{
						name:       "greet",
						returnType: "string",
						params: []phpParameter{
							{name: "name", phpType: "string"},
						},
						signature: "greet(string $name): string",
					},
				},
				classes: []phpClass{},
			},
			expectError: false,
		},
		{
			name: "extension with classes",
			generator: &Generator{
				BaseName:  "classextension",
				BuildDir:  "",
				functions: []phpFunction{},
				classes: []phpClass{
					{
						name: "TestClass",
						properties: []phpClassProperty{
							{name: "name", phpType: "string"},
							{name: "count", phpType: "int", isNullable: true},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "extension with both functions and classes",
			generator: &Generator{
				BaseName: "fullextension",
				BuildDir: "",
				functions: []phpFunction{
					{
						name:             "calculate",
						returnType:       "int",
						isReturnNullable: true,
						params: []phpParameter{
							{name: "base", phpType: "int"},
							{name: "multiplier", phpType: "int", hasDefault: true, defaultValue: "2", isNullable: true},
						},
						signature: "calculate(int $base, ?int $multiplier = 2): ?int",
					},
				},
				classes: []phpClass{
					{
						name: "Calculator",
						properties: []phpClassProperty{
							{name: "precision", phpType: "int"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty extension",
			generator: &Generator{
				BaseName:  "emptyextension",
				BuildDir:  "",
				functions: []phpFunction{},
				classes:   []phpClass{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			tt.generator.BuildDir = tempDir

			docGen := &DocumentationGenerator{
				generator: tt.generator,
			}

			err := docGen.generate()

			if tt.expectError {
				assert.Error(t, err, "generate() expected error but got none")
				return
			}

			assert.NoError(t, err, "generate() unexpected error")

			readmePath := filepath.Join(tempDir, "README.md")
			_, err = os.Stat(readmePath)
			if !assert.False(t, os.IsNotExist(err), "README.md file was not created") {
				return
			}

			content, err := os.ReadFile(readmePath)
			if !assert.NoError(t, err, "Failed to read generated README.md") {
				return
			}

			contentStr := string(content)

			assert.Contains(t, contentStr, "# "+tt.generator.BaseName+" Extension", "README should contain extension title")

			assert.Contains(t, contentStr, "Auto-generated PHP extension from Go code.", "README should contain description")

			if len(tt.generator.functions) > 0 {
				assert.Contains(t, contentStr, "## functions", "README should contain functions section when functions exist")

				for _, fn := range tt.generator.functions {
					assert.Contains(t, contentStr, "### "+fn.name, "README should contain function %s", fn.name)
					assert.Contains(t, contentStr, fn.signature, "README should contain function signature for %s", fn.name)
				}
			}

			if len(tt.generator.classes) > 0 {
				assert.Contains(t, contentStr, "## classes", "README should contain classes section when classes exist")

				for _, class := range tt.generator.classes {
					assert.Contains(t, contentStr, "### "+class.name, "README should contain class %s", class.name)
				}
			}
		})
	}
}

func TestDocumentationGenerator_GenerateMarkdown(t *testing.T) {
	tests := []struct {
		name        string
		generator   *Generator
		contains    []string
		notContains []string
	}{
		{
			name: "function with parameters",
			generator: &Generator{
				BaseName: "testextension",
				functions: []phpFunction{
					{
						name:       "processData",
						returnType: "array",
						params: []phpParameter{
							{name: "data", phpType: "string"},
							{name: "options", phpType: "array", isNullable: true},
							{name: "count", phpType: "int", hasDefault: true, defaultValue: "10"},
						},
						signature: "processData(string $data, ?array $options, int $count = 10): array",
					},
				},
				classes: []phpClass{},
			},
			contains: []string{
				"# testextension Extension",
				"## functions",
				"### processData",
				"**Parameters:**",
				"- `data` (string)",
				"- `options` (array) (nullable)",
				"- `count` (int) (default: 10)",
				"**Returns:** array",
			},
		},
		{
			name: "nullable return type",
			generator: &Generator{
				BaseName: "nullableext",
				functions: []phpFunction{
					{
						name:             "maybeGetValue",
						returnType:       "string",
						isReturnNullable: true,
						params:           []phpParameter{},
						signature:        "maybeGetValue(): ?string",
					},
				},
				classes: []phpClass{},
			},
			contains: []string{
				"**Returns:** string (nullable)",
			},
		},
		{
			name: "class with properties",
			generator: &Generator{
				BaseName:  "classext",
				functions: []phpFunction{},
				classes: []phpClass{
					{
						name: "DataProcessor",
						properties: []phpClassProperty{
							{name: "name", phpType: "string"},
							{name: "config", phpType: "array", isNullable: true},
							{name: "enabled", phpType: "bool"},
						},
					},
				},
			},
			contains: []string{
				"## classes",
				"### DataProcessor",
				"**properties:**",
				"- `name`: string",
				"- `config`: array (nullable)",
				"- `enabled`: bool",
			},
		},
		{
			name: "extension with no functions or classes",
			generator: &Generator{
				BaseName:  "emptyext",
				functions: []phpFunction{},
				classes:   []phpClass{},
			},
			contains: []string{
				"# emptyext Extension",
				"Auto-generated PHP extension from Go code.",
			},
			notContains: []string{
				"## functions",
				"## classes",
			},
		},
		{
			name: "function with no parameters",
			generator: &Generator{
				BaseName: "noparamext",
				functions: []phpFunction{
					{
						name:       "getCurrentTime",
						returnType: "int",
						params:     []phpParameter{},
						signature:  "getCurrentTime(): int",
					},
				},
				classes: []phpClass{},
			},
			contains: []string{
				"### getCurrentTime",
				"**Returns:** int",
			},
			notContains: []string{
				"**Parameters:**",
			},
		},
		{
			name: "class with no properties",
			generator: &Generator{
				BaseName:  "nopropsext",
				functions: []phpFunction{},
				classes: []phpClass{
					{
						name:       "EmptyClass",
						properties: []phpClassProperty{},
					},
				},
			},
			contains: []string{
				"### EmptyClass",
			},
			notContains: []string{
				"**properties:**",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docGen := &DocumentationGenerator{
				generator: tt.generator,
			}

			result, err := docGen.generateMarkdown()
			if !assert.NoError(t, err, "generateMarkdown() unexpected error") {
				return
			}

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "generateMarkdown() should contain '%s'", expected)
			}

			for _, notExpected := range tt.notContains {
				assert.NotContains(t, result, notExpected, "generateMarkdown() should NOT contain '%s'", notExpected)
			}
		})
	}
}

func TestDocumentationGenerator_Generate_InvalidDirectory(t *testing.T) {
	generator := &Generator{
		BaseName:  "test",
		BuildDir:  "/nonexistent/directory",
		functions: []phpFunction{},
		classes:   []phpClass{},
	}

	docGen := &DocumentationGenerator{
		generator: generator,
	}

	err := docGen.generate()
	assert.Error(t, err, "generate() expected error for invalid directory but got none")
}

func TestDocumentationGenerator_TemplateError(t *testing.T) {
	generator := &Generator{
		BaseName: "test",
		functions: []phpFunction{
			{
				name:       "test",
				returnType: "string",
				signature:  "test(): string",
			},
		},
		classes: []phpClass{},
	}

	docGen := &DocumentationGenerator{
		generator: generator,
	}

	result, err := docGen.generateMarkdown()
	assert.NoError(t, err, "generateMarkdown() unexpected error")
	assert.NotEmpty(t, result, "generateMarkdown() returned empty result")
}

func BenchmarkDocumentationGenerator_GenerateMarkdown(b *testing.B) {
	generator := &Generator{
		BaseName: "benchext",
		functions: []phpFunction{
			{
				name:       "function1",
				returnType: "string",
				params: []phpParameter{
					{name: "param1", phpType: "string"},
					{name: "param2", phpType: "int", hasDefault: true, defaultValue: "0"},
				},
				signature: "function1(string $param1, int $param2 = 0): string",
			},
			{
				name:             "function2",
				returnType:       "array",
				isReturnNullable: true,
				params: []phpParameter{
					{name: "data", phpType: "array", isNullable: true},
				},
				signature: "function2(?array $data): ?array",
			},
		},
		classes: []phpClass{
			{
				name: "TestClass",
				properties: []phpClassProperty{
					{name: "prop1", phpType: "string"},
					{name: "prop2", phpType: "int", isNullable: true},
				},
			},
		},
	}

	docGen := &DocumentationGenerator{
		generator: generator,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := docGen.generateMarkdown()
		if err != nil {
			b.Fatalf("generateMarkdown() error: %v", err)
		}
	}
}
