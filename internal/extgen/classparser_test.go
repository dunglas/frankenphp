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
	name string
	Age  int
}`,
			expected: 1,
		},
		{
			name: "multiple classes",
			input: `package main

//export_php:class User
type UserStruct struct {
	name string
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
	name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) string {
	return u.name
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
				assert.Equal(t, "User", class.name, "Expected class name 'User'")
				assert.Equal(t, "UserStruct", class.goStruct, "Expected Go struct 'UserStruct'")
				assert.Len(t, class.properties, 2, "Expected 2 properties")
			}

			if tt.name == "class with nullable fields" && len(classes) > 0 {
				class := classes[0]
				if len(class.properties) >= 3 {
					assert.False(t, class.properties[0].isNullable, "Required field should not be nullable")
					assert.True(t, class.properties[1].isNullable, "Optional field should be nullable")
					assert.True(t, class.properties[2].isNullable, "Count field should be nullable")
				}
			}
		})
	}
}

func TestClassMethods(t *testing.T) {
	input := `package main

//export_php:class User
type UserStruct struct {
	name string
	Age  int
}

//export_php:method User::getName(): string
func GetUserName(u UserStruct) string {
	return u.name
}

//export_php:method User::setAge(int $age): void
func SetUserAge(u *UserStruct, age int) {
	u.Age = age
}

//export_php:method User::getInfo(string $prefix = "User"): string
func GetUserInfo(u UserStruct, prefix string) string {
	return prefix + ": " + u.name
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
	assert.Len(t, class.methods, 3, "Expected 3 methods")
	if len(class.methods) != 3 {
		return
	}

	getName := class.methods[0]
	assert.Equal(t, "getName", getName.name, "Expected method name 'getName'")
	assert.Equal(t, "string", getName.returnType, "Expected return type 'string'")
	assert.Empty(t, getName.params, "Expected 0 params")
	assert.Equal(t, "User", getName.className, "Expected class name 'User'")

	setAge := class.methods[1]
	assert.Equal(t, "setAge", setAge.name, "Expected method name 'setAge'")
	assert.Equal(t, "void", setAge.returnType, "Expected return type 'void'")
	assert.Len(t, setAge.params, 1, "Expected 1 param")
	if len(setAge.params) > 0 {
		param := setAge.params[0]
		assert.Equal(t, "age", param.name, "Expected param name 'age'")
		assert.Equal(t, "int", param.phpType, "Expected param type 'int'")
		assert.False(t, param.isNullable, "Expected param to not be nullable")
		assert.False(t, param.hasDefault, "Expected param to not have default value")
	}

	getInfo := class.methods[2]
	assert.Equal(t, "getInfo", getInfo.name, "Expected method name 'getInfo'")
	assert.Equal(t, "string", getInfo.returnType, "Expected return type 'string'")
	assert.Len(t, getInfo.params, 1, "Expected 1 param")
	if len(getInfo.params) > 0 {
		param := getInfo.params[0]
		assert.Equal(t, "prefix", param.name, "Expected param name 'prefix'")
		assert.Equal(t, "string", param.phpType, "Expected param type 'string'")
		assert.True(t, param.hasDefault, "Expected param to have default value")
		assert.Equal(t, "User", param.defaultValue, "Expected default value 'User'")
	}
}

func TestMethodParameterParsing(t *testing.T) {
	tests := []struct {
		name          string
		paramStr      string
		expectedParam phpParameter
		expectError   bool
	}{
		{
			name:     "simple int parameter",
			paramStr: "int $age",
			expectedParam: phpParameter{
				name:       "age",
				phpType:    "int",
				isNullable: false,
				hasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "nullable string parameter",
			paramStr: "?string $name",
			expectedParam: phpParameter{
				name:       "name",
				phpType:    "string",
				isNullable: true,
				hasDefault: false,
			},
			expectError: false,
		},
		{
			name:     "parameter with default value",
			paramStr: "string $prefix = \"default\"",
			expectedParam: phpParameter{
				name:         "prefix",
				phpType:      "string",
				isNullable:   false,
				hasDefault:   true,
				defaultValue: "default",
			},
			expectError: false,
		},
		{
			name:     "nullable parameter with default null",
			paramStr: "?int $count = null",
			expectedParam: phpParameter{
				name:         "count",
				phpType:      "int",
				isNullable:   true,
				hasDefault:   true,
				defaultValue: "null",
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

			assert.Equal(t, tt.expectedParam.name, param.name, "Expected name '%s'", tt.expectedParam.name)
			assert.Equal(t, tt.expectedParam.phpType, param.phpType, "Expected type '%s'", tt.expectedParam.phpType)
			assert.Equal(t, tt.expectedParam.isNullable, param.isNullable, "Expected isNullable %v", tt.expectedParam.isNullable)
			assert.Equal(t, tt.expectedParam.hasDefault, param.hasDefault, "Expected hasDefault %v", tt.expectedParam.hasDefault)
			assert.Equal(t, tt.expectedParam.defaultValue, param.defaultValue, "Expected defaultValue '%s'", tt.expectedParam.defaultValue)
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
			assert.Len(t, class.properties, len(tt.expected), "Expected %d properties", len(tt.expected))
			if len(class.properties) != len(tt.expected) {
				return
			}

			for i, expectedType := range tt.expected {
				assert.Equal(t, expectedType, class.properties[i].phpType, "Property %d: expected type %s", i, expectedType)
			}
		})
	}
}
