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
				name:       "validFunction",
				returnType: "string",
				params: []phpParameter{
					{name: "param1", phpType: "string"},
					{name: "param2", phpType: "int"},
				},
			},
			expectError: false,
		},
		{
			name: "valid function with nullable return",
			function: phpFunction{
				name:             "nullableReturn",
				returnType:       "string",
				isReturnNullable: true,
				params: []phpParameter{
					{name: "data", phpType: "array"},
				},
			},
			expectError: false,
		},
		{
			name: "empty function name",
			function: phpFunction{
				name:       "",
				returnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid function name - starts with number",
			function: phpFunction{
				name:       "123invalid",
				returnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid function name - contains special chars",
			function: phpFunction{
				name:       "invalid-name",
				returnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			function: phpFunction{
				name:       "validName",
				returnType: "string",
				params: []phpParameter{
					{name: "123invalid", phpType: "string"},
				},
			},
			expectError: true,
		},
		{
			name: "empty parameter name",
			function: phpFunction{
				name:       "validName",
				returnType: "string",
				params: []phpParameter{
					{name: "", phpType: "string"},
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
				assert.Error(t, err, "validateFunction() should return an error for function %s", tt.function.name)
			} else {
				assert.NoError(t, err, "validateFunction() should not return an error for function %s", tt.function.name)
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
				name:    "validProperty",
				phpType: "string",
				goType:  "string",
			},
			expectError: false,
		},
		{
			name: "valid nullable property",
			prop: phpClassProperty{
				name:       "nullableProperty",
				phpType:    "int",
				goType:     "*int",
				isNullable: true,
			},
			expectError: false,
		},
		{
			name: "empty property name",
			prop: phpClassProperty{
				name:    "",
				phpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid property name",
			prop: phpClassProperty{
				name:    "123invalid",
				phpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid property type",
			prop: phpClassProperty{
				name:    "validName",
				phpType: "invalidType",
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
				name:    "validParam",
				phpType: "string",
			},
			expectError: false,
		},
		{
			name: "valid nullable parameter",
			param: phpParameter{
				name:       "nullableParam",
				phpType:    "int",
				isNullable: true,
			},
			expectError: false,
		},
		{
			name: "valid parameter with default",
			param: phpParameter{
				name:         "defaultParam",
				phpType:      "string",
				hasDefault:   true,
				defaultValue: "hello",
			},
			expectError: false,
		},
		{
			name: "empty parameter name",
			param: phpParameter{
				name:    "",
				phpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			param: phpParameter{
				name:    "123invalid",
				phpType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter type",
			param: phpParameter{
				name:    "validName",
				phpType: "invalidType",
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
				name:     "ValidClass",
				goStruct: "ValidStruct",
				properties: []phpClassProperty{
					{name: "name", phpType: "string"},
					{name: "age", phpType: "int"},
				},
			},
			expectError: false,
		},
		{
			name: "valid class with nullable properties",
			class: phpClass{
				name:     "NullableClass",
				goStruct: "NullableStruct",
				properties: []phpClassProperty{
					{name: "required", phpType: "string", isNullable: false},
					{name: "optional", phpType: "string", isNullable: true},
				},
			},
			expectError: false,
		},
		{
			name: "empty class name",
			class: phpClass{
				name:     "",
				goStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid class name",
			class: phpClass{
				name:     "123InvalidClass",
				goStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid property",
			class: phpClass{
				name:     "ValidClass",
				goStruct: "ValidStruct",
				properties: []phpClassProperty{
					{name: "123invalid", phpType: "string"},
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
