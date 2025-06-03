package extgen

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFunctionParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single function",
			input: `package main

//export_php:function testFunc(string $name): string
func testFunc(name *go_string) *go_value {
	return String("Hello " + CStringToGoString(name))
}`,
			expected: 1,
		},
		{
			name: "multiple functions",
			input: `package main

//export_php:function func1(int $a): int
func func1(a long) *go_value {
	return Int(a * 2)
}

//export_php:function func2(string $b): string  
func func2(b *go_string) *go_value {
	return String("processed: " + CStringToGoString(b))
}`,
			expected: 2,
		},
		{
			name: "no php functions",
			input: `package main

func regularFunc() {
	// Just a regular Go function
}`,
			expected: 0,
		},
		{
			name: "mixed functions",
			input: `package main

//export_php:function phpFunc(string $data): string
func phpFunc(data *go_string) *go_value {
	return String("PHP: " + CStringToGoString(data))
}

func internalFunc() {
	// Internal function without export_php comment
}

//export_php:function anotherPhpFunc(int $num): int
func anotherPhpFunc(num long) *go_value {
	return Int(num * 10)
}`,
			expected: 2,
		},
		{
			name: "wrong args syntax",
			input: `package main

//export_php function phpFunc(data string): string
func phpFunc(data *go_string) *go_value {
	return String("PHP: " + CStringToGoString(data))
}`,
			expected: 0,
		},
		{
			name: "decoupled function names",
			input: `package main

//export_php:function my_php_function(string $name): string
func myGoFunction(name *go_string) *go_value {
	return String("Hello " + CStringToGoString(name))
}

//export_php:function another_php_func(int $num): int
func someOtherGoName(num long) *go_value {
	return Int(num * 5)
}`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test*.go")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.input)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			parser := NewFuncParserDefRegex()
			functions, err := parser.parse(tmpfile.Name())
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			assert.Len(t, functions, tt.expected, "parse() got wrong number of functions")

			if tt.name == "single function" && len(functions) > 0 {
				fn := functions[0]
				assert.Equal(t, "testFunc", fn.Name, "Expected function name 'testFunc'")
				assert.Equal(t, "string", fn.ReturnType, "Expected return type 'string'")
				assert.Len(t, fn.Params, 1, "Expected 1 parameter")
				if len(fn.Params) > 0 {
					assert.Equal(t, "name", fn.Params[0].Name, "Expected parameter name 'name'")
				}
			}

			if tt.name == "decoupled function names" && len(functions) >= 2 {
				fn1 := functions[0]
				assert.Equal(t, "my_php_function", fn1.Name, "Expected PHP function name 'my_php_function'")
				fn2 := functions[1]
				assert.Equal(t, "another_php_func", fn2.Name, "Expected PHP function name 'another_php_func'")
			}
		})
	}
}

func TestSignatureParsing(t *testing.T) {
	tests := []struct {
		name        string
		signature   string
		expectError bool
		funcName    string
		paramCount  int
		returnType  string
		nullable    bool
	}{
		{
			name:       "simple function",
			signature:  "test(name string): string",
			funcName:   "test",
			paramCount: 1,
			returnType: "string",
			nullable:   false,
		},
		{
			name:       "nullable return",
			signature:  "test(id int): ?string",
			funcName:   "test",
			paramCount: 1,
			returnType: "string",
			nullable:   true,
		},
		{
			name:       "multiple params",
			signature:  "calculate(a int, b float, name string): float",
			funcName:   "calculate",
			paramCount: 3,
			returnType: "float",
			nullable:   false,
		},
		{
			name:       "no parameters",
			signature:  "getValue(): int",
			funcName:   "getValue",
			paramCount: 0,
			returnType: "int",
			nullable:   false,
		},
		{
			name:       "nullable parameters",
			signature:  "process(?string data, ?int count): bool",
			funcName:   "process",
			paramCount: 2,
			returnType: "bool",
			nullable:   false,
		},
		{
			name:        "invalid signature",
			signature:   "invalid syntax here",
			expectError: true,
		},
		{
			name:        "missing return type",
			signature:   "test(name string)",
			expectError: true,
		},
	}

	parser := NewFuncParserDefRegex()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := parser.parseSignature(tt.signature)

			if tt.expectError {
				assert.Error(t, err, "parseSignature() expected error but got none")
				return
			}

			assert.NoError(t, err, "parseSignature() unexpected error")
			assert.Equal(t, tt.funcName, fn.Name, "parseSignature() name mismatch")
			assert.Len(t, fn.Params, tt.paramCount, "parseSignature() param count mismatch")
			assert.Equal(t, tt.returnType, fn.ReturnType, "parseSignature() return type mismatch")
			assert.Equal(t, tt.nullable, fn.IsReturnNullable, "parseSignature() nullable mismatch")

			if tt.name == "nullable parameters" {
				if len(fn.Params) >= 2 {
					assert.True(t, fn.Params[0].IsNullable, "First parameter should be nullable")
					assert.True(t, fn.Params[1].IsNullable, "Second parameter should be nullable")
				}
			}
		})
	}
}

func TestParameterParsing(t *testing.T) {
	tests := []struct {
		name             string
		paramStr         string
		expectedName     string
		expectedType     string
		expectedNullable bool
		expectedDefault  string
		hasDefault       bool
		expectError      bool
	}{
		{
			name:         "simple string param",
			paramStr:     "string name",
			expectedName: "name",
			expectedType: "string",
		},
		{
			name:             "nullable int param",
			paramStr:         "?int count",
			expectedName:     "count",
			expectedType:     "int",
			expectedNullable: true,
		},
		{
			name:            "param with default",
			paramStr:        "string message = 'hello'",
			expectedName:    "message",
			expectedType:    "string",
			expectedDefault: "hello",
			hasDefault:      true,
		},
		{
			name:            "int with default",
			paramStr:        "int limit = 10",
			expectedName:    "limit",
			expectedType:    "int",
			expectedDefault: "10",
			hasDefault:      true,
		},
		{
			name:             "nullable with default",
			paramStr:         "?string data = null",
			expectedName:     "data",
			expectedType:     "string",
			expectedNullable: true,
			expectedDefault:  "null",
			hasDefault:       true,
		},
		{
			name:        "invalid format",
			paramStr:    "invalid",
			expectError: true,
		},
	}

	parser := NewFuncParserDefRegex()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, err := parser.parseParameter(tt.paramStr)

			if tt.expectError {
				assert.Error(t, err, "parseParameter() expected error but got none")
				return
			}

			assert.NoError(t, err, "parseParameter() unexpected error")
			assert.Equal(t, tt.expectedName, param.Name, "parseParameter() name mismatch")
			assert.Equal(t, tt.expectedType, param.Type, "parseParameter() type mismatch")
			assert.Equal(t, tt.expectedNullable, param.IsNullable, "parseParameter() nullable mismatch")
			assert.Equal(t, tt.hasDefault, param.HasDefault, "parseParameter() hasDefault mismatch")

			if tt.hasDefault {
				assert.Equal(t, tt.expectedDefault, param.DefaultValue, "parseParameter() defaultValue mismatch")
			}
		})
	}
}
