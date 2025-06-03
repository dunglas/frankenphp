package extgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFunction(t *testing.T) {
	tests := []struct {
		name        string
		function    PHPFunction
		expectError bool
	}{
		{
			name: "valid function",
			function: PHPFunction{
				Name:       "validFunction",
				ReturnType: "string",
				Params: []Parameter{
					{Name: "param1", Type: "string"},
					{Name: "param2", Type: "int"},
				},
			},
			expectError: false,
		},
		{
			name: "valid function with nullable return",
			function: PHPFunction{
				Name:             "nullableReturn",
				ReturnType:       "string",
				IsReturnNullable: true,
				Params: []Parameter{
					{Name: "data", Type: "array"},
				},
			},
			expectError: false,
		},
		{
			name: "empty function name",
			function: PHPFunction{
				Name:       "",
				ReturnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid function name - starts with number",
			function: PHPFunction{
				Name:       "123invalid",
				ReturnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid function name - contains special chars",
			function: PHPFunction{
				Name:       "invalid-name",
				ReturnType: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			function: PHPFunction{
				Name:       "validName",
				ReturnType: "string",
				Params: []Parameter{
					{Name: "123invalid", Type: "string"},
				},
			},
			expectError: true,
		},
		{
			name: "empty parameter name",
			function: PHPFunction{
				Name:       "validName",
				ReturnType: "string",
				Params: []Parameter{
					{Name: "", Type: "string"},
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
		prop        ClassProperty
		expectError bool
	}{
		{
			name: "valid property",
			prop: ClassProperty{
				Name:   "validProperty",
				Type:   "string",
				GoType: "string",
			},
			expectError: false,
		},
		{
			name: "valid nullable property",
			prop: ClassProperty{
				Name:       "nullableProperty",
				Type:       "int",
				GoType:     "*int",
				IsNullable: true,
			},
			expectError: false,
		},
		{
			name: "empty property name",
			prop: ClassProperty{
				Name: "",
				Type: "string",
			},
			expectError: true,
		},
		{
			name: "invalid property name",
			prop: ClassProperty{
				Name: "123invalid",
				Type: "string",
			},
			expectError: true,
		},
		{
			name: "invalid property type",
			prop: ClassProperty{
				Name: "validName",
				Type: "invalidType",
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
		param       Parameter
		expectError bool
	}{
		{
			name: "valid string parameter",
			param: Parameter{
				Name: "validParam",
				Type: "string",
			},
			expectError: false,
		},
		{
			name: "valid nullable parameter",
			param: Parameter{
				Name:       "nullableParam",
				Type:       "int",
				IsNullable: true,
			},
			expectError: false,
		},
		{
			name: "valid parameter with default",
			param: Parameter{
				Name:         "defaultParam",
				Type:         "string",
				HasDefault:   true,
				DefaultValue: "hello",
			},
			expectError: false,
		},
		{
			name: "empty parameter name",
			param: Parameter{
				Name: "",
				Type: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter name",
			param: Parameter{
				Name: "123invalid",
				Type: "string",
			},
			expectError: true,
		},
		{
			name: "invalid parameter type",
			param: Parameter{
				Name: "validName",
				Type: "invalidType",
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
		class       PHPClass
		expectError bool
	}{
		{
			name: "valid class",
			class: PHPClass{
				Name:     "ValidClass",
				GoStruct: "ValidStruct",
				Properties: []ClassProperty{
					{Name: "name", Type: "string"},
					{Name: "age", Type: "int"},
				},
			},
			expectError: false,
		},
		{
			name: "valid class with nullable properties",
			class: PHPClass{
				Name:     "NullableClass",
				GoStruct: "NullableStruct",
				Properties: []ClassProperty{
					{Name: "required", Type: "string", IsNullable: false},
					{Name: "optional", Type: "string", IsNullable: true},
				},
			},
			expectError: false,
		},
		{
			name: "empty class name",
			class: PHPClass{
				Name:     "",
				GoStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid class name",
			class: PHPClass{
				Name:     "123InvalidClass",
				GoStruct: "ValidStruct",
			},
			expectError: true,
		},
		{
			name: "invalid property",
			class: PHPClass{
				Name:     "ValidClass",
				GoStruct: "ValidStruct",
				Properties: []ClassProperty{
					{Name: "123invalid", Type: "string"},
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
