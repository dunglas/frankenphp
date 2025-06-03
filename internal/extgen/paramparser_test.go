package extgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParameterParser_AnalyzeParameters(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		params   []Parameter
		expected ParameterInfo
	}{
		{
			name:   "no parameters",
			params: []Parameter{},
			expected: ParameterInfo{
				RequiredCount: 0,
				TotalCount:    0,
			},
		},
		{
			name: "all required parameters",
			params: []Parameter{
				{Name: "name", Type: "string", HasDefault: false},
				{Name: "count", Type: "int", HasDefault: false},
			},
			expected: ParameterInfo{
				RequiredCount: 2,
				TotalCount:    2,
			},
		},
		{
			name: "mixed required and optional parameters",
			params: []Parameter{
				{Name: "name", Type: "string", HasDefault: false},
				{Name: "count", Type: "int", HasDefault: true, DefaultValue: "10"},
				{Name: "enabled", Type: "bool", HasDefault: true, DefaultValue: "true"},
			},
			expected: ParameterInfo{
				RequiredCount: 1,
				TotalCount:    3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.analyzeParameters(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateParamDeclarations(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		params   []Parameter
		expected string
	}{
		{
			name:     "no parameters",
			params:   []Parameter{},
			expected: "",
		},
		{
			name: "string parameter",
			params: []Parameter{
				{Name: "message", Type: "string", HasDefault: false},
			},
			expected: "    zend_string *message = NULL;",
		},
		{
			name: "nullable string parameter",
			params: []Parameter{
				{Name: "message", Type: "string", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_string *message = NULL;\n    zend_bool message_is_null = 0;",
		},
		{
			name: "int parameter with default",
			params: []Parameter{
				{Name: "count", Type: "int", HasDefault: true, DefaultValue: "42"},
			},
			expected: "    zend_long count = 42;",
		},
		{
			name: "nullable int parameter",
			params: []Parameter{
				{Name: "count", Type: "int", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_long count = 0;\n    zend_bool count_is_null = 0;",
		},
		{
			name: "bool parameter with true default",
			params: []Parameter{
				{Name: "enabled", Type: "bool", HasDefault: true, DefaultValue: "true"},
			},
			expected: "    zend_bool enabled = 1;",
		},
		{
			name: "nullable bool parameter",
			params: []Parameter{
				{Name: "enabled", Type: "bool", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_bool enabled = 0;\n    zend_bool enabled_is_null = 0;",
		},
		{
			name: "float parameter",
			params: []Parameter{
				{Name: "ratio", Type: "float", HasDefault: false},
			},
			expected: "    double ratio = 0.0;",
		},
		{
			name: "nullable float parameter",
			params: []Parameter{
				{Name: "ratio", Type: "float", HasDefault: false, IsNullable: true},
			},
			expected: "    double ratio = 0.0;\n    zend_bool ratio_is_null = 0;",
		},
		{
			name: "multiple parameters",
			params: []Parameter{
				{Name: "name", Type: "string", HasDefault: false},
				{Name: "count", Type: "int", HasDefault: true, DefaultValue: "10"},
			},
			expected: "    zend_string *name = NULL;\n    zend_long count = 10;",
		},
		{
			name: "mixed nullable and non-nullable parameters",
			params: []Parameter{
				{Name: "name", Type: "string", HasDefault: false, IsNullable: false},
				{Name: "count", Type: "int", HasDefault: false, IsNullable: true},
			},
			expected: "    zend_string *name = NULL;\n    zend_long count = 0;\n    zend_bool count_is_null = 0;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateParamDeclarations(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateParamParsing(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name          string
		params        []Parameter
		requiredCount int
		expected      string
	}{
		{
			name:          "no parameters",
			params:        []Parameter{},
			requiredCount: 0,
			expected: `    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }`,
		},
		{
			name: "single required string parameter",
			params: []Parameter{
				{Name: "message", Type: "string", HasDefault: false},
			},
			requiredCount: 1,
			expected: `    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STR(message)
    ZEND_PARSE_PARAMETERS_END();`,
		},
		{
			name: "mixed required and optional parameters",
			params: []Parameter{
				{Name: "name", Type: "string", HasDefault: false},
				{Name: "count", Type: "int", HasDefault: true, DefaultValue: "10"},
				{Name: "enabled", Type: "bool", HasDefault: true, DefaultValue: "true"},
			},
			requiredCount: 1,
			expected: `    ZEND_PARSE_PARAMETERS_START(1, 3)
        Z_PARAM_STR(name)
        Z_PARAM_OPTIONAL
        Z_PARAM_LONG(count)
        Z_PARAM_BOOL(enabled)
    ZEND_PARSE_PARAMETERS_END();`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateParamParsing(tt.params, tt.requiredCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateGoCallParams(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		params   []Parameter
		expected string
	}{
		{
			name:     "no parameters",
			params:   []Parameter{},
			expected: "",
		},
		{
			name: "single string parameter",
			params: []Parameter{
				{Name: "message", Type: "string"},
			},
			expected: "message",
		},
		{
			name: "multiple parameters of different types",
			params: []Parameter{
				{Name: "name", Type: "string"},
				{Name: "count", Type: "int"},
				{Name: "ratio", Type: "float"},
				{Name: "enabled", Type: "bool"},
			},
			expected: "name, (long) count, (double) ratio, (int) enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateGoCallParams(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateParamParsingMacro(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    Parameter
		expected string
	}{
		{
			name:     "string parameter",
			param:    Parameter{Name: "message", Type: "string"},
			expected: "\n        Z_PARAM_STR(message)",
		},
		{
			name:     "nullable string parameter",
			param:    Parameter{Name: "message", Type: "string", IsNullable: true},
			expected: "\n        Z_PARAM_STR_OR_NULL(message, message_is_null)",
		},
		{
			name:     "int parameter",
			param:    Parameter{Name: "count", Type: "int"},
			expected: "\n        Z_PARAM_LONG(count)",
		},
		{
			name:     "nullable int parameter",
			param:    Parameter{Name: "count", Type: "int", IsNullable: true},
			expected: "\n        Z_PARAM_LONG_OR_NULL(count, count_is_null)",
		},
		{
			name:     "float parameter",
			param:    Parameter{Name: "ratio", Type: "float"},
			expected: "\n        Z_PARAM_DOUBLE(ratio)",
		},
		{
			name:     "nullable float parameter",
			param:    Parameter{Name: "ratio", Type: "float", IsNullable: true},
			expected: "\n        Z_PARAM_DOUBLE_OR_NULL(ratio, ratio_is_null)",
		},
		{
			name:     "bool parameter",
			param:    Parameter{Name: "enabled", Type: "bool"},
			expected: "\n        Z_PARAM_BOOL(enabled)",
		},
		{
			name:     "nullable bool parameter",
			param:    Parameter{Name: "enabled", Type: "bool", IsNullable: true},
			expected: "\n        Z_PARAM_BOOL_OR_NULL(enabled, enabled_is_null)",
		},
		{
			name:     "unknown type",
			param:    Parameter{Name: "unknown", Type: "unknown"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateParamParsingMacro(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GetDefaultValue(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    Parameter
		fallback string
		expected string
	}{
		{
			name:     "parameter without default",
			param:    Parameter{Name: "count", Type: "int", HasDefault: false},
			fallback: "0",
			expected: "0",
		},
		{
			name:     "parameter with default value",
			param:    Parameter{Name: "count", Type: "int", HasDefault: true, DefaultValue: "42"},
			fallback: "0",
			expected: "42",
		},
		{
			name:     "parameter with empty default value",
			param:    Parameter{Name: "count", Type: "int", HasDefault: true, DefaultValue: ""},
			fallback: "0",
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.getDefaultValue(tt.param, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateSingleGoCallParam(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    Parameter
		expected string
	}{
		{
			name:     "string parameter",
			param:    Parameter{Name: "message", Type: "string"},
			expected: "message",
		},
		{
			name:     "nullable string parameter",
			param:    Parameter{Name: "message", Type: "string", IsNullable: true},
			expected: "message_is_null ? NULL : message",
		},
		{
			name:     "int parameter",
			param:    Parameter{Name: "count", Type: "int"},
			expected: "(long) count",
		},
		{
			name:     "nullable int parameter",
			param:    Parameter{Name: "count", Type: "int", IsNullable: true},
			expected: "count_is_null ? NULL : &count",
		},
		{
			name:     "float parameter",
			param:    Parameter{Name: "ratio", Type: "float"},
			expected: "(double) ratio",
		},
		{
			name:     "nullable float parameter",
			param:    Parameter{Name: "ratio", Type: "float", IsNullable: true},
			expected: "ratio_is_null ? NULL : &ratio",
		},
		{
			name:     "bool parameter",
			param:    Parameter{Name: "enabled", Type: "bool"},
			expected: "(int) enabled",
		},
		{
			name:     "nullable bool parameter",
			param:    Parameter{Name: "enabled", Type: "bool", IsNullable: true},
			expected: "enabled_is_null ? NULL : &enabled",
		},
		{
			name:     "unknown type",
			param:    Parameter{Name: "unknown", Type: "unknown"},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateSingleGoCallParam(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_GenerateSingleParamDeclaration(t *testing.T) {
	pp := &ParameterParser{}

	tests := []struct {
		name     string
		param    Parameter
		expected []string
	}{
		{
			name:     "string parameter",
			param:    Parameter{Name: "message", Type: "string", HasDefault: false},
			expected: []string{"zend_string *message = NULL;"},
		},
		{
			name:     "nullable string parameter",
			param:    Parameter{Name: "message", Type: "string", HasDefault: false, IsNullable: true},
			expected: []string{"zend_string *message = NULL;", "zend_bool message_is_null = 0;"},
		},
		{
			name:     "int parameter with default",
			param:    Parameter{Name: "count", Type: "int", HasDefault: true, DefaultValue: "42"},
			expected: []string{"zend_long count = 42;"},
		},
		{
			name:     "nullable int parameter",
			param:    Parameter{Name: "count", Type: "int", HasDefault: false, IsNullable: true},
			expected: []string{"zend_long count = 0;", "zend_bool count_is_null = 0;"},
		},
		{
			name:     "bool parameter with true default",
			param:    Parameter{Name: "enabled", Type: "bool", HasDefault: true, DefaultValue: "true"},
			expected: []string{"zend_bool enabled = 1;"},
		},
		{
			name:     "nullable bool parameter",
			param:    Parameter{Name: "enabled", Type: "bool", HasDefault: false, IsNullable: true},
			expected: []string{"zend_bool enabled = 0;", "zend_bool enabled_is_null = 0;"},
		},
		{
			name:     "bool parameter with false default",
			param:    Parameter{Name: "disabled", Type: "bool", HasDefault: true, DefaultValue: "false"},
			expected: []string{"zend_bool disabled = false;"},
		},
		{
			name:     "float parameter",
			param:    Parameter{Name: "ratio", Type: "float", HasDefault: false},
			expected: []string{"double ratio = 0.0;"},
		},
		{
			name:     "nullable float parameter",
			param:    Parameter{Name: "ratio", Type: "float", HasDefault: false, IsNullable: true},
			expected: []string{"double ratio = 0.0;", "zend_bool ratio_is_null = 0;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pp.generateSingleParamDeclaration(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParameterParser_Integration(t *testing.T) {
	pp := &ParameterParser{}

	params := []Parameter{
		{Name: "name", Type: "string", HasDefault: false},
		{Name: "count", Type: "int", HasDefault: true, DefaultValue: "10"},
		{Name: "enabled", Type: "bool", HasDefault: true, DefaultValue: "true"},
	}

	info := pp.analyzeParameters(params)
	assert.Equal(t, 1, info.RequiredCount)
	assert.Equal(t, 3, info.TotalCount)

	declarations := pp.generateParamDeclarations(params)
	expectedDeclarations := []string{
		"zend_string *name = NULL;",
		"zend_long count = 10;",
		"zend_bool enabled = 1;",
	}
	for _, expected := range expectedDeclarations {
		assert.Contains(t, declarations, expected)
	}

	parsing := pp.generateParamParsing(params, info.RequiredCount)
	assert.Contains(t, parsing, "ZEND_PARSE_PARAMETERS_START(1, 3)")
	assert.Contains(t, parsing, "Z_PARAM_OPTIONAL")

	goCallParams := pp.generateGoCallParams(params)
	assert.Equal(t, "name, (long) count, (int) enabled", goCallParams)
}
