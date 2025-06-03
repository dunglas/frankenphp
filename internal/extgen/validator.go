package extgen

import (
	"fmt"
	"regexp"
)

var functionNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var parameterNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var classNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var propNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type Validator struct{}

func (v *Validator) validateFunction(fn PHPFunction) error {
	if fn.Name == "" {
		return fmt.Errorf("function name cannot be empty")
	}

	if !functionNameRegex.MatchString(fn.Name) {
		return fmt.Errorf("invalid function name: %s", fn.Name)
	}

	for i, param := range fn.Params {
		if err := v.validateParameter(param); err != nil {
			return fmt.Errorf("parameter %d (%s): %w", i, param.Name, err)
		}
	}

	if err := v.validateReturnType(fn.ReturnType); err != nil {
		return fmt.Errorf("return type: %w", err)
	}

	return nil
}

func (v *Validator) validateParameter(param Parameter) error {
	if param.Name == "" {
		return fmt.Errorf("parameter name cannot be empty")
	}

	if !parameterNameRegex.MatchString(param.Name) {
		return fmt.Errorf("invalid parameter name: %s", param.Name)
	}

	validTypes := []string{"string", "int", "float", "bool", "array", "object", "mixed"}
	if !v.isValidType(param.Type, validTypes) {
		return fmt.Errorf("invalid parameter type: %s", param.Type)
	}

	return nil
}

func (v *Validator) validateReturnType(returnType string) error {
	validReturnTypes := []string{"void", "string", "int", "float", "bool", "array", "object", "mixed", "null", "true", "false"}
	if !v.isValidType(returnType, validReturnTypes) {
		return fmt.Errorf("invalid return type: %s", returnType)
	}
	return nil
}

func (v *Validator) validateClass(class PHPClass) error {
	if class.Name == "" {
		return fmt.Errorf("class name cannot be empty")
	}

	if !classNameRegex.MatchString(class.Name) {
		return fmt.Errorf("invalid class name: %s", class.Name)
	}

	for i, prop := range class.Properties {
		if err := v.validateClassProperty(prop); err != nil {
			return fmt.Errorf("property %d (%s): %w", i, prop.Name, err)
		}
	}

	return nil
}

func (v *Validator) validateClassProperty(prop ClassProperty) error {
	if prop.Name == "" {
		return fmt.Errorf("property name cannot be empty")
	}

	if !propNameRegex.MatchString(prop.Name) {
		return fmt.Errorf("invalid property name: %s", prop.Name)
	}

	validTypes := []string{"string", "int", "float", "bool", "array", "object", "mixed"}
	if !v.isValidType(prop.Type, validTypes) {
		return fmt.Errorf("invalid property type: %s", prop.Type)
	}

	return nil
}

func (v *Validator) isValidType(typeStr string, validTypes []string) bool {
	for _, valid := range validTypes {
		if typeStr == valid {
			return true
		}
	}
	return false
}
