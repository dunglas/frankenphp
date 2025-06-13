package extgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPHPFunctionGenerator_Generate(t *testing.T) {
	tests := []struct {
		name     string
		function phpFunction
		contains []string // Strings that should be present in the output
	}{
		{
			name: "simple string function",
			function: phpFunction{
				Name:       "greet",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "name", PhpType: "string"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(greet)",
				"zend_string *name = NULL;",
				"Z_PARAM_STR(name)",
				"zend_string *result = greet(name);",
				"RETURN_STR(result)",
			},
		},
		{
			name: "function with default parameter",
			function: phpFunction{
				Name:       "calculate",
				ReturnType: "int",
				Params: []phpParameter{
					{Name: "base", PhpType: "int"},
					{Name: "multiplier", PhpType: "int", HasDefault: true, DefaultValue: "2"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(calculate)",
				"zend_long base = 0;",
				"zend_long multiplier = 2;",
				"ZEND_PARSE_PARAMETERS_START(1, 2)",
				"Z_PARAM_OPTIONAL",
				"Z_PARAM_LONG(base)",
				"Z_PARAM_LONG(multiplier)",
			},
		},
		{
			name: "void function",
			function: phpFunction{
				Name:       "doSomething",
				ReturnType: "void",
				Params: []phpParameter{
					{Name: "action", PhpType: "string"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(doSomething)",
				"doSomething(action);",
			},
		},
		{
			name: "bool function with default",
			function: phpFunction{
				Name:       "isEnabled",
				ReturnType: "bool",
				Params: []phpParameter{
					{Name: "flag", PhpType: "bool", HasDefault: true, DefaultValue: "true"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(isEnabled)",
				"zend_bool flag = 1;",
				"Z_PARAM_BOOL(flag)",
				"RETURN_BOOL(result)",
			},
		},
		{
			name: "float function",
			function: phpFunction{
				Name:       "calculate",
				ReturnType: "float",
				Params: []phpParameter{
					{Name: "value", PhpType: "float"},
				},
			},
			contains: []string{
				"PHP_FUNCTION(calculate)",
				"double value = 0.0;",
				"Z_PARAM_DOUBLE(value)",
				"RETURN_DOUBLE(result)",
			},
		},
	}

	generator := PHPFuncGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generate(tt.function)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "Generated code should contain '%s'", expected)
			}

			assert.True(t, strings.HasPrefix(result, "PHP_FUNCTION("), "Generated code should start with PHP_FUNCTION")
			assert.True(t, strings.HasSuffix(strings.TrimSpace(result), "}"), "Generated code should end with closing brace")
		})
	}
}

func TestPHPFunctionGenerator_GenerateParamDeclarations(t *testing.T) {
	tests := []struct {
		name     string
		params   []phpParameter
		contains []string
	}{
		{
			name: "string parameter",
			params: []phpParameter{
				{Name: "message", PhpType: "string"},
			},
			contains: []string{
				"zend_string *message = NULL;",
			},
		},
		{
			name: "int parameter",
			params: []phpParameter{
				{Name: "count", PhpType: "int"},
			},
			contains: []string{
				"zend_long count = 0;",
			},
		},
		{
			name: "bool with default",
			params: []phpParameter{
				{Name: "enabled", PhpType: "bool", HasDefault: true, DefaultValue: "true"},
			},
			contains: []string{
				"zend_bool enabled = 1;",
			},
		},
		{
			name: "float parameter with default",
			params: []phpParameter{
				{Name: "rate", PhpType: "float", HasDefault: true, DefaultValue: "1.5"},
			},
			contains: []string{
				"double rate = 1.5;",
			},
		},
	}

	parser := ParameterParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.generateParamDeclarations(tt.params)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "phpParameter declarations should contain '%s'", expected)
			}
		})
	}
}

func TestPHPFunctionGenerator_GenerateReturnCode(t *testing.T) {
	tests := []struct {
		name       string
		returnType string
		contains   []string
	}{
		{
			name:       "string return",
			returnType: "string",
			contains: []string{
				"RETURN_STR(result)",
				"RETURN_EMPTY_STRING()",
			},
		},
		{
			name:       "int return",
			returnType: "int",
			contains: []string{
				"RETURN_LONG(result)",
			},
		},
		{
			name:       "bool return",
			returnType: "bool",
			contains: []string{
				"RETURN_BOOL(result)",
			},
		},
		{
			name:       "float return",
			returnType: "float",
			contains: []string{
				"RETURN_DOUBLE(result)",
			},
		},
		{
			name:       "void return",
			returnType: "void",
			contains:   []string{},
		},
	}

	generator := PHPFuncGenerator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.generateReturnCode(tt.returnType)

			if len(tt.contains) == 0 {
				assert.Empty(t, result, "Return code should be empty for void")
				return
			}

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "Return code should contain '%s'", expected)
			}
		})
	}
}

func TestPHPFunctionGenerator_GenerateGoCallParams(t *testing.T) {
	tests := []struct {
		name     string
		params   []phpParameter
		expected string
	}{
		{
			name:     "no parameters",
			params:   []phpParameter{},
			expected: "",
		},
		{
			name: "simple string parameter",
			params: []phpParameter{
				{Name: "message", PhpType: "string"},
			},
			expected: "message",
		},
		{
			name: "int parameter",
			params: []phpParameter{
				{Name: "count", PhpType: "int"},
			},
			expected: "(long) count",
		},
		{
			name: "multiple parameters",
			params: []phpParameter{
				{Name: "name", PhpType: "string"},
				{Name: "age", PhpType: "int"},
			},
			expected: "name, (long) age",
		},
		{
			name: "bool and float parameters",
			params: []phpParameter{
				{Name: "enabled", PhpType: "bool"},
				{Name: "rate", PhpType: "float"},
			},
			expected: "(int) enabled, (double) rate",
		},
	}

	parser := ParameterParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.generateGoCallParams(tt.params)

			assert.Equal(t, tt.expected, result, "generateGoCallParams() mismatch")
		})
	}
}

func TestPHPFunctionGenerator_AnalyzeParameters(t *testing.T) {
	tests := []struct {
		name          string
		params        []phpParameter
		expectedReq   int
		expectedTotal int
	}{
		{
			name:          "no parameters",
			params:        []phpParameter{},
			expectedReq:   0,
			expectedTotal: 0,
		},
		{
			name: "all required",
			params: []phpParameter{
				{Name: "a", PhpType: "string"},
				{Name: "b", PhpType: "int"},
			},
			expectedReq:   2,
			expectedTotal: 2,
		},
		{
			name: "mixed required and optional",
			params: []phpParameter{
				{Name: "required", PhpType: "string"},
				{Name: "optional", PhpType: "int", HasDefault: true, DefaultValue: "10"},
			},
			expectedReq:   1,
			expectedTotal: 2,
		},
		{
			name: "all optional",
			params: []phpParameter{
				{Name: "opt1", PhpType: "string", HasDefault: true, DefaultValue: "hello"},
				{Name: "opt2", PhpType: "int", HasDefault: true, DefaultValue: "0"},
			},
			expectedReq:   0,
			expectedTotal: 2,
		},
	}

	parser := ParameterParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parser.analyzeParameters(tt.params)

			assert.Equal(t, tt.expectedReq, info.RequiredCount, "analyzeParameters() RequiredCount mismatch")
			assert.Equal(t, tt.expectedTotal, info.TotalCount, "analyzeParameters() TotalCount mismatch")
		})
	}
}
