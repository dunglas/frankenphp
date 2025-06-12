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

func (v *Validator) validateFunction(fn phpFunction) error {
	if fn.name == "" {
		return fmt.Errorf("function name cannot be empty")
	}

	if !functionNameRegex.MatchString(fn.name) {
		return fmt.Errorf("invalid function name: %s", fn.name)
	}

	for i, param := range fn.params {
		if err := v.validateParameter(param); err != nil {
			return fmt.Errorf("parameter %d (%s): %w", i, param.name, err)
		}
	}

	if err := v.validateReturnType(fn.returnType); err != nil {
		return fmt.Errorf("return type: %w", err)
	}

	return nil
}

func (v *Validator) validateParameter(param phpParameter) error {
	if param.name == "" {
		return fmt.Errorf("parameter name cannot be empty")
	}

	if !parameterNameRegex.MatchString(param.name) {
		return fmt.Errorf("invalid parameter name: %s", param.name)
	}

	validTypes := []string{"string", "int", "float", "bool", "array", "object", "mixed"}
	if !v.isValidType(param.phpType, validTypes) {
		return fmt.Errorf("invalid parameter type: %s", param.phpType)
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

func (v *Validator) validateClass(class phpClass) error {
	if class.name == "" {
		return fmt.Errorf("class name cannot be empty")
	}

	if !classNameRegex.MatchString(class.name) {
		return fmt.Errorf("invalid class name: %s", class.name)
	}

	for i, prop := range class.properties {
		if err := v.validateClassProperty(prop); err != nil {
			return fmt.Errorf("property %d (%s): %w", i, prop.name, err)
		}
	}

	return nil
}

func (v *Validator) validateClassProperty(prop phpClassProperty) error {
	if prop.name == "" {
		return fmt.Errorf("property name cannot be empty")
	}

	if !propNameRegex.MatchString(prop.name) {
		return fmt.Errorf("invalid property name: %s", prop.name)
	}

	validTypes := []string{"string", "int", "float", "bool", "array", "object", "mixed"}
	if !v.isValidType(prop.phpType, validTypes) {
		return fmt.Errorf("invalid property type: %s", prop.phpType)
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
