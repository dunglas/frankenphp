package extgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFunction(t *testing.T) {
	tests := []struct {
		name        string
		function    phpFunction
		expectError bool
	}{
		{
			name: "valid function",
			function: phpFunction{
				Name:       "validFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "param1", PhpType: "string"},
					{Name: "param2", PhpType: "int"},
				},
			},
			expectError: false,
		},
		{
			name: "valid function with nullable return",
			function: phpFunction{
				Name:             "nullableReturn",
				ReturnType:       "string",
				IsReturnNullable: true,
				Params: []phpParameter{
					{Name: "data", PhpType: "array"},
				},
			},
			expectError: false,
		},
		{
			name: "empty function name",
			function: phpFunction{
				Name:       "",
				ReturnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid function name - starts with number",
			function: phpFunction{
				Name:       "123invalid",
				ReturnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid function name - contains special chars",
			function: phpFunction{
				Name:       "invalid-name",
				ReturnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			function: phpFunction{
				Name:       "validName",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "123invalid", PhpType: "string"},
				},
			},
			expectError: true,
		},
		{
			name: "empty parameter name",
			function: phpFunction{
				Name:       "validName",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "", PhpType: "string"},
				},
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateFunction(tt.function)

			if tt.expectError {
				assert.Error(t, err, "validateFunction() should return an error for function %s", tt.function.Name)
			} else {
				assert.NoError(t, err, "validateFunction() should not return an error for function %s", tt.function.Name)
			}
		})
	}
}

func TestValidateReturnType(t *testing.T) {
	tests := []struct {
		name        string
		returnType  string
		expectError bool
	}{
		{
			name:        "valid string type",
			returnType:  "string",
			expectError: false,
		},
		{
			name:        "valid int type",
			returnType:  "int",
			expectError: false,
		},
		{
			name:        "valid array type",
			returnType:  "array",
			expectError: false,
		},
		{
			name:        "valid bool type",
			returnType:  "bool",
			expectError: false,
		},
		{
			name:        "valid float type",
			returnType:  "float",
			expectError: false,
		},
		{
			name:        "valid void type",
			returnType:  "void",
			expectError: false,
		},
		{
			name:        "invalid return type",
			returnType:  "invalidType",
			expectError: true,
		},
		{
			name:        "empty return type",
			returnType:  "",
			expectError: true,
		},
		{
			name:        "case sensitive - String should be invalid",
			returnType:  "String",
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateReturnType(tt.returnType)

			if tt.expectError {
				assert.Error(t, err, "validateReturnType(%s) should return an error", tt.returnType)
			} else {
				assert.NoError(t, err, "validateReturnType(%s) should not return an error", tt.returnType)
			}
		})
	}
}

func TestValidateClassProperty(t *testing.T) {
	tests := []struct {
		name        string
		prop        phpClassProperty
		expectError bool
	}{
		{
			name: "valid property",
			prop: phpClassProperty{
				Name:    "validProperty",
				PhpType: "string",
				goType:  "string",
			},
			expectError: false,
		},
		{
			name: "valid nullable property",
			prop: phpClassProperty{
				Name:       "nullableProperty",
				PhpType:    "int",
				goType:     "*int",
				IsNullable: true,
			},
			expectError: false,
		},
		{
			name: "empty property name",
			prop: phpClassProperty{
				Name:    "",
				PhpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid property name",
			prop: phpClassProperty{
				Name:    "123invalid",
				PhpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid property type",
			prop: phpClassProperty{
				Name:    "validName",
				PhpType: "invalidType",
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateClassProperty(tt.prop)

			if tt.expectError {
				assert.Error(t, err, "validateClassProperty() should return an error")
			} else {
				assert.NoError(t, err, "validateClassProperty() should not return an error")
			}
		})
	}
}

func TestValidateParameter(t *testing.T) {
	tests := []struct {
		name        string
		param       phpParameter
		expectError bool
	}{
		{
			name: "valid string parameter",
			param: phpParameter{
				Name:    "validParam",
				PhpType: "string",
			},
			expectError: false,
		},
		{
			name: "valid nullable parameter",
			param: phpParameter{
				Name:       "nullableParam",
				PhpType:    "int",
				IsNullable: true,
			},
			expectError: false,
		},
		{
			name: "valid parameter with default",
			param: phpParameter{
				Name:         "defaultParam",
				PhpType:      "string",
				HasDefault:   true,
				DefaultValue: "hello",
			},
			expectError: false,
		},
		{
			name: "empty parameter name",
			param: phpParameter{
				Name:    "",
				PhpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			param: phpParameter{
				Name:    "123invalid",
				PhpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter type",
			param: phpParameter{
				Name:    "validName",
				PhpType: "invalidType",
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateParameter(tt.param)

			if tt.expectError {
				assert.Error(t, err, "validateParameter() should return an error")
			} else {
				assert.NoError(t, err, "validateParameter() should not return an error")
			}
		})
	}
}

func TestValidateClass(t *testing.T) {
	tests := []struct {
		name        string
		class       phpClass
		expectError bool
	}{
		{
			name: "valid class",
			class: phpClass{
				Name:     "ValidClass",
				GoStruct: "ValidStruct",
				Properties: []phpClassProperty{
					{Name: "name", PhpType: "string"},
					{Name: "age", PhpType: "int"},
				},
			},
			expectError: false,
		},
		{
			name: "valid class with nullable properties",
			class: phpClass{
				Name:     "NullableClass",
				GoStruct: "NullableStruct",
				Properties: []phpClassProperty{
					{Name: "required", PhpType: "string", IsNullable: false},
					{Name: "optional", PhpType: "string", IsNullable: true},
				},
			},
			expectError: false,
		},
		{
			name: "empty class name",
			class: phpClass{
				Name:     "",
				GoStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid class name",
			class: phpClass{
				Name:     "123InvalidClass",
				GoStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid property",
			class: phpClass{
				Name:     "ValidClass",
				GoStruct: "ValidStruct",
				Properties: []phpClassProperty{
					{Name: "123invalid", PhpType: "string"},
				},
			},
			expectError: true,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateClass(tt.class)

			if tt.expectError {
				assert.Error(t, err, "validateClass() should return an error")
			} else {
				assert.NoError(t, err, "validateClass() should not return an error")
			}
		})
	}
}

func TestValidateScalarTypes(t *testing.T) {
	tests := []struct {
		name        string
		function    phpFunction
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid scalar parameters only",
			function: phpFunction{
				Name:       "validFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "stringParam", PhpType: "string"},
					{Name: "intParam", PhpType: "int"},
					{Name: "floatParam", PhpType: "float"},
					{Name: "boolParam", PhpType: "bool"},
				},
			},
			expectError: false,
		},
		{
			name: "valid nullable scalar parameters",
			function: phpFunction{
				Name:       "nullableFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "stringParam", PhpType: "string", IsNullable: true},
					{Name: "intParam", PhpType: "int", IsNullable: true},
				},
			},
			expectError: false,
		},
		{
			name: "valid void return type",
			function: phpFunction{
				Name:       "voidFunction",
				ReturnType: "void",
				Params: []phpParameter{
					{Name: "stringParam", PhpType: "string"},
				},
			},
			expectError: false,
		},
		{
			name: "invalid array parameter",
			function: phpFunction{
				Name:       "arrayFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "arrayParam", PhpType: "array"},
				},
			},
			expectError: true,
			errorMsg:    "parameter 1 (arrayParam) has unsupported type 'array'",
		},
		{
			name: "invalid object parameter",
			function: phpFunction{
				Name:       "objectFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "objectParam", PhpType: "object"},
				},
			},
			expectError: true,
			errorMsg:    "parameter 1 (objectParam) has unsupported type 'object'",
		},
		{
			name: "invalid mixed parameter",
			function: phpFunction{
				Name:       "mixedFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "mixedParam", PhpType: "mixed"},
				},
			},
			expectError: true,
			errorMsg:    "parameter 1 (mixedParam) has unsupported type 'mixed'",
		},
		{
			name: "invalid array return type",
			function: phpFunction{
				Name:       "arrayReturnFunction",
				ReturnType: "array",
				Params: []phpParameter{
					{Name: "stringParam", PhpType: "string"},
				},
			},
			expectError: true,
			errorMsg:    "return type 'array' is not supported",
		},
		{
			name: "invalid object return type",
			function: phpFunction{
				Name:       "objectReturnFunction",
				ReturnType: "object",
				Params: []phpParameter{
					{Name: "stringParam", PhpType: "string"},
				},
			},
			expectError: true,
			errorMsg:    "return type 'object' is not supported",
		},
		{
			name: "mixed scalar and invalid parameters",
			function: phpFunction{
				Name:       "mixedFunction",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "validParam", PhpType: "string"},
					{Name: "invalidParam", PhpType: "array"},
					{Name: "anotherValidParam", PhpType: "int"},
				},
			},
			expectError: true,
			errorMsg:    "parameter 2 (invalidParam) has unsupported type 'array'",
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateScalarTypes(tt.function)

			if tt.expectError {
				assert.Error(t, err, "validateScalarTypes() should return an error for function %s", tt.function.Name)
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "validateScalarTypes() should not return an error for function %s", tt.function.Name)
			}
		})
	}
}

func TestValidateGoFunctionSignature(t *testing.T) {
	tests := []struct {
		name        string
		phpFunc     phpFunction
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid Go function signature",
			phpFunc: phpFunction{
				Name:       "testFunc",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "name", PhpType: "string"},
					{Name: "count", PhpType: "int"},
				},
				goFunction: `func testFunc(name *C.zend_string, count int64) unsafe.Pointer {
	return nil
}`,
			},
			expectError: false,
		},
		{
			name: "valid void return type",
			phpFunc: phpFunction{
				Name:       "voidFunc",
				ReturnType: "void",
				Params: []phpParameter{
					{Name: "message", PhpType: "string"},
				},
				goFunction: `func voidFunc(message *C.zend_string) {
	// Do something
}`,
			},
			expectError: false,
		},
		{
			name: "no Go function provided",
			phpFunc: phpFunction{
				Name:       "noGoFunc",
				ReturnType: "string",
				Params:     []phpParameter{},
				goFunction: "",
			},
			expectError: true,
			errorMsg:    "no Go function found",
		},
		{
			name: "parameter count mismatch",
			phpFunc: phpFunction{
				Name:       "countMismatch",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "param1", PhpType: "string"},
					{Name: "param2", PhpType: "int"},
				},
				goFunction: `func countMismatch(param1 *C.zend_string) unsafe.Pointer {
	return nil
}`,
			},
			expectError: true,
			errorMsg:    "parameter count mismatch: PHP function has 2 parameters but Go function has 1",
		},
		{
			name: "parameter type mismatch",
			phpFunc: phpFunction{
				Name:       "typeMismatch",
				ReturnType: "string",
				Params: []phpParameter{
					{Name: "name", PhpType: "string"},
					{Name: "count", PhpType: "int"},
				},
				goFunction: `func typeMismatch(name *C.zend_string, count string) unsafe.Pointer {
	return nil
}`,
			},
			expectError: true,
			errorMsg:    "parameter 2 type mismatch: PHP 'int' requires Go type 'int64' but found 'string'",
		},
		{
			name: "return type mismatch",
			phpFunc: phpFunction{
				Name:       "returnMismatch",
				ReturnType: "int",
				Params: []phpParameter{
					{Name: "value", PhpType: "string"},
				},
				goFunction: `func returnMismatch(value *C.zend_string) string {
	return ""
}`,
			},
			expectError: true,
			errorMsg:    "return type mismatch: PHP 'int' requires Go return type 'int64' but found 'string'",
		},
		{
			name: "valid bool parameter and return",
			phpFunc: phpFunction{
				Name:       "boolFunc",
				ReturnType: "bool",
				Params: []phpParameter{
					{Name: "flag", PhpType: "bool"},
				},
				goFunction: `func boolFunc(flag bool) bool {
	return flag
}`,
			},
			expectError: false,
		},
		{
			name: "valid float parameter and return",
			phpFunc: phpFunction{
				Name:       "floatFunc",
				ReturnType: "float",
				Params: []phpParameter{
					{Name: "value", PhpType: "float"},
				},
				goFunction: `func floatFunc(value float64) float64 {
	return value * 2.0
}`,
			},
			expectError: false,
		},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateGoFunctionSignatureWithOptions(tt.phpFunc, false)

			if tt.expectError {
				assert.Error(t, err, "validateGoFunctionSignature() should return an error for function %s", tt.phpFunc.Name)
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "validateGoFunctionSignature() should not return an error for function %s", tt.phpFunc.Name)
			}
		})
	}
}

func TestPhpTypeToGoType(t *testing.T) {
	tests := []struct {
		phpType    string
		isNullable bool
		expected   string
	}{
		{"string", false, "*C.zend_string"},
		{"string", true, "*C.zend_string"}, // String is already a pointer, no change for nullable
		{"int", false, "int64"},
		{"int", true, "*int64"}, // Nullable int becomes pointer to int64
		{"float", false, "float64"},
		{"float", true, "*float64"}, // Nullable float becomes pointer to float64
		{"bool", false, "bool"},
		{"bool", true, "*bool"}, // Nullable bool becomes pointer to bool
		{"unknown", false, "interface{}"},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.phpType, func(t *testing.T) {
			result := validator.phpTypeToGoType(tt.phpType, tt.isNullable)
			assert.Equal(t, tt.expected, result, "phpTypeToGoType(%s, %v) should return %s", tt.phpType, tt.isNullable, tt.expected)
		})
	}
}

func TestPhpReturnTypeToGoType(t *testing.T) {
	tests := []struct {
		phpReturnType string
		isNullable    bool
		expected      string
	}{
		{"void", false, ""},
		{"void", true, ""},
		{"string", false, "unsafe.Pointer"},
		{"string", true, "unsafe.Pointer"},
		{"int", false, "int64"},
		{"int", true, "int64"},
		{"float", false, "float64"},
		{"float", true, "float64"},
		{"bool", false, "bool"},
		{"bool", true, "bool"},
		{"unknown", false, "interface{}"},
	}

	validator := Validator{}
	for _, tt := range tests {
		t.Run(tt.phpReturnType, func(t *testing.T) {
			result := validator.phpReturnTypeToGoType(tt.phpReturnType, tt.isNullable)
			assert.Equal(t, tt.expected, result, "phpReturnTypeToGoType(%s, %v) should return %s", tt.phpReturnType, tt.isNullable, tt.expected)
		})
	}
}
