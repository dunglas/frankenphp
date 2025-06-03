package extgen

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single class",
			input: `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}`,
			expected: 1,
		},
		{
			name: "multiple classes",
			input: `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}

//export_php:class Product
type ProductStruct struct {
	Title string
	Price float64
}`,
			expected: 2,
		},
		{
			name: "no php classes",
			input: `package main

type RegularStruct struct {
	Data string
}`,
			expected: 0,
		},
		{
			name: "class with nullable fields",
			input: `package main

//export_php:class OptionalData
type OptionalStruct struct {
	Required string
	Optional *string
	Count    *int
}`,
			expected: 1,
		},
		{
			name: "class with methods",
			input: `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) string {
	return u.Name
}

//export_php:method User::setAge(int $age): void
func SetUserAge(u *UserStruct, age int) {
	u.Age = age
}`,
			expected: 1,
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

			parser := ClassParser{}
			classes, err := parser.parse(tmpfile.Name())
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			assert.Len(t, classes, tt.expected, "parse() got wrong number of classes")

			if tt.name == "single class" && len(classes) > 0 {
				class := classes[0]
				assert.Equal(t, "User", class.Name, "Expected class name 'User'")
				assert.Equal(t, "UserStruct", class.GoStruct, "Expected Go struct 'UserStruct'")
				assert.Len(t, class.Properties, 2, "Expected 2 properties")
			}

			if tt.name == "class with nullable fields" && len(classes) > 0 {
				class := classes[0]
				if len(class.Properties) >= 3 {
					assert.False(t, class.Properties[0].IsNullable, "Required field should not be nullable")
					assert.True(t, class.Properties[1].IsNullable, "Optional field should be nullable")
					assert.True(t, class.Properties[2].IsNullable, "Count field should be nullable")
				}
			}
		})
	}
}

func TestClassMethods(t *testing.T) {
	input := `package main

//export_php:class User
type UserStruct struct {
	Name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) string {
	return u.Name
}

//export_php:method User::setAge(int $age): void
func SetUserAge(u *UserStruct, age int) {
	u.Age = age
}

//export_php:method User::getInfo(string $prefix = "User"): string
func GetUserInfo(u UserStruct, prefix string) string {
	return prefix + ": " + u.Name
}`

	tmpfile, err := os.CreateTemp("", "test*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	parser := ClassParser{}
	classes, err := parser.parse(tmpfile.Name())
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	assert.Len(t, classes, 1, "Expected 1 class")
	if len(classes) != 1 {
		return
	}

	class := classes[0]
	assert.Len(t, class.Methods, 3, "Expected 3 methods")
	if len(class.Methods) != 3 {
		return
	}

	getName := class.Methods[0]
	assert.Equal(t, "getName", getName.Name, "Expected method name 'getName'")
	assert.Equal(t, "string", getName.ReturnType, "Expected return type 'string'")
	assert.Empty(t, getName.Params, "Expected 0 params")
	assert.Equal(t, "User", getName.ClassName, "Expected class name 'User'")

	setAge := class.Methods[1]
	assert.Equal(t, "setAge", setAge.Name, "Expected method name 'setAge'")
	assert.Equal(t, "void", setAge.ReturnType, "Expected return type 'void'")
	assert.Len(t, setAge.Params, 1, "Expected 1 param")
	if len(setAge.Params) > 0 {
		param := setAge.Params[0]
		assert.Equal(t, "age", param.Name, "Expected param name 'age'")
		assert.Equal(t, "int", param.Type, "Expected param type 'int'")
		assert.False(t, param.IsNullable, "Expected param to not be nullable")
		assert.False(t, param.HasDefault, "Expected param to not have default value")
	}

	getInfo := class.Methods[2]
	assert.Equal(t, "getInfo", getInfo.Name, "Expected method name 'getInfo'")
	assert.Equal(t, "string", getInfo.ReturnType, "Expected return type 'string'")
	assert.Len(t, getInfo.Params, 1, "Expected 1 param")
	if len(getInfo.Params) > 0 {
		param := getInfo.Params[0]
		assert.Equal(t, "prefix", param.Name, "Expected param name 'prefix'")
		assert.Equal(t, "string", param.Type, "Expected param type 'string'")
		assert.True(t, param.HasDefault, "Expected param to have default value")
		assert.Equal(t, "User", param.DefaultValue, "Expected default value 'User'")
	}
}

func TestMethodParameterParsing(t *testing.T) {
	tests := []struct {
		name          string
		paramStr      string
		expectedParam Parameter
		expectError   bool
	}{
		{
			name:     "simple int parameter",
			paramStr: "int $age",
			expectedParam: Parameter{
				Name:       "age",
				Type:       "int",
				IsNullable: false,
				HasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "nullable string parameter",
			paramStr: "?string $name",
			expectedParam: Parameter{
				Name:       "name",
				Type:       "string",
				IsNullable: true,
				HasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "parameter with default value",
			paramStr: "string $prefix = \"default\"",
			expectedParam: Parameter{
				Name:         "prefix",
				Type:         "string",
				IsNullable:   false,
				HasDefault:   true,
				DefaultValue: "default",
			},
			expectError: false,
		},
		{
			name:     "nullable parameter with default null",
			paramStr: "?int $count = null",
			expectedParam: Parameter{
				Name:         "count",
				Type:         "int",
				IsNullable:   true,
				HasDefault:   true,
				DefaultValue: "null",
			},
			expectError: false,
		},
		{
			name:        "invalid parameter format",
			paramStr:    "invalid",
			expectError: true,
		},
	}

	parser := ClassParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param, err := parser.parseMethodParameter(tt.paramStr)

			if tt.expectError {
				assert.Error(t, err, "Expected error for parameter '%s', but got none", tt.paramStr)
				return
			}

			assert.NoError(t, err, "parseMethodParameter(%s) error", tt.paramStr)
			if err != nil {
				return
			}

			assert.Equal(t, tt.expectedParam.Name, param.Name, "Expected name '%s'", tt.expectedParam.Name)
			assert.Equal(t, tt.expectedParam.Type, param.Type, "Expected type '%s'", tt.expectedParam.Type)
			assert.Equal(t, tt.expectedParam.IsNullable, param.IsNullable, "Expected IsNullable %v", tt.expectedParam.IsNullable)
			assert.Equal(t, tt.expectedParam.HasDefault, param.HasDefault, "Expected HasDefault %v", tt.expectedParam.HasDefault)
			assert.Equal(t, tt.expectedParam.DefaultValue, param.DefaultValue, "Expected DefaultValue '%s'", tt.expectedParam.DefaultValue)
		})
	}
}

func TestGoTypeToPHPType(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "string"},
		{"*string", "string"},
		{"int", "int"},
		{"int64", "int"},
		{"*int", "int"},
		{"float64", "float"},
		{"*float32", "float"},
		{"bool", "bool"},
		{"*bool", "bool"},
		{"[]string", "array"},
		{"map[string]int", "array"},
		{"*[]int", "array"},
		{"interface{}", "mixed"},
		{"CustomType", "mixed"},
	}

	parser := ClassParser{}
	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := parser.goTypeToPHPType(tt.goType)
			assert.Equal(t, tt.expected, result, "goTypeToPHPType(%s) = %s, want %s", tt.goType, result, tt.expected)
		})
	}
}

func TestTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "basic types",
			input: `package main

//export_php:class TestClass
type TestStruct struct {
	StringField string
	IntField    int
	FloatField  float64
	BoolField   bool
}`,
			expected: []string{"string", "int", "float", "bool"},
		},
		{
			name: "pointer types",
			input: `package main

//export_php:class NullableClass
type NullableStruct struct {
	NullableString *string
	NullableInt    *int
	NullableFloat  *float64
	NullableBool   *bool
}`,
			expected: []string{"string", "int", "float", "bool"},
		},
		{
			name: "collection types",
			input: `package main

//export_php:class CollectionClass
type CollectionStruct struct {
	StringSlice []string
	IntMap      map[string]int
	MixedSlice  []interface{}
}`,
			expected: []string{"array", "array", "array"},
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

			parser := ClassParser{}
			classes, err := parser.parse(tmpfile.Name())
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}

			assert.Len(t, classes, 1, "Expected 1 class")
			if len(classes) != 1 {
				return
			}

			class := classes[0]
			assert.Len(t, class.Properties, len(tt.expected), "Expected %d properties", len(tt.expected))
			if len(class.Properties) != len(tt.expected) {
				return
			}

			for i, expectedType := range tt.expected {
				assert.Equal(t, expectedType, class.Properties[i].Type, "Property %d: expected type %s", i, expectedType)
			}
		})
	}
}
