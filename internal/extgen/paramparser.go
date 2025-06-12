package extgen

import (
	"fmt"
	"strings"
)

type ParameterParser struct{}

type ParameterInfo struct {
	RequiredCount int
	TotalCount    int
}

func (pp *ParameterParser) analyzeParameters(params []phpParameter) ParameterInfo {
	info := ParameterInfo{TotalCount: len(params)}

	for _, param := range params {
		if !param.hasDefault {
			info.RequiredCount++
		}
	}

	return info
}

func (pp *ParameterParser) generateParamDeclarations(params []phpParameter) string {
	if len(params) == 0 {
		return ""
	}

	var declarations []string

	for _, param := range params {
		declarations = append(declarations, pp.generateSingleParamDeclaration(param)...)
	}

	return "    " + strings.Join(declarations, "\n    ")
}

func (pp *ParameterParser) generateSingleParamDeclaration(param phpParameter) []string {
	var decls []string

	switch param.phpType {
	case "string":
		decls = append(decls, fmt.Sprintf("zend_string *%s = NULL;", param.name))
		if param.isNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.name))
		}
	case "int":
		defaultVal := pp.getDefaultValue(param, "0")
		decls = append(decls, fmt.Sprintf("zend_long %s = %s;", param.name, defaultVal))
		if param.isNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.name))
		}
	case "float":
		defaultVal := pp.getDefaultValue(param, "0.0")
		decls = append(decls, fmt.Sprintf("double %s = %s;", param.name, defaultVal))
		if param.isNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.name))
		}
	case "bool":
		defaultVal := pp.getDefaultValue(param, "0")
		if param.hasDefault && param.defaultValue == "true" {
			defaultVal = "1"
		}
		decls = append(decls, fmt.Sprintf("zend_bool %s = %s;", param.name, defaultVal))
		if param.isNullable {
			decls = append(decls, fmt.Sprintf("zend_bool %s_is_null = 0;", param.name))
		}
	}

	return decls
}

func (pp *ParameterParser) getDefaultValue(param phpParameter, fallback string) string {
	if !param.hasDefault || param.defaultValue == "" {
		return fallback
	}
	return param.defaultValue
}

func (pp *ParameterParser) generateParamParsing(params []phpParameter, requiredCount int) string {
	if len(params) == 0 {
		return `    if (zend_parse_parameters_none() == FAILURE) {
        RETURN_THROWS();
    }`
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("    ZEND_PARSE_PARAMETERS_START(%d, %d)", requiredCount, len(params)))

	optionalStarted := false
	for _, param := range params {
		if param.hasDefault && !optionalStarted {
			builder.WriteString("\n        Z_PARAM_OPTIONAL")
			optionalStarted = true
		}

		builder.WriteString(pp.generateParamParsingMacro(param))
	}

	builder.WriteString("\n    ZEND_PARSE_PARAMETERS_END();")
	return builder.String()
}

func (pp *ParameterParser) generateParamParsingMacro(param phpParameter) string {
	if param.isNullable {
		switch param.phpType {
		case "string":
			return fmt.Sprintf("\n        Z_PARAM_STR_OR_NULL(%s, %s_is_null)", param.name, param.name)
		case "int":
			return fmt.Sprintf("\n        Z_PARAM_LONG_OR_NULL(%s, %s_is_null)", param.name, param.name)
		case "float":
			return fmt.Sprintf("\n        Z_PARAM_DOUBLE_OR_NULL(%s, %s_is_null)", param.name, param.name)
		case "bool":
			return fmt.Sprintf("\n        Z_PARAM_BOOL_OR_NULL(%s, %s_is_null)", param.name, param.name)
		default:
			return ""
		}
	} else {
		switch param.phpType {
		case "string":
			return fmt.Sprintf("\n        Z_PARAM_STR(%s)", param.name)
		case "int":
			return fmt.Sprintf("\n        Z_PARAM_LONG(%s)", param.name)
		case "float":
			return fmt.Sprintf("\n        Z_PARAM_DOUBLE(%s)", param.name)
		case "bool":
			return fmt.Sprintf("\n        Z_PARAM_BOOL(%s)", param.name)
		default:
			return ""
		}
	}
}

func (pp *ParameterParser) generateGoCallParams(params []phpParameter) string {
	if len(params) == 0 {
		return ""
	}

	var goParams []string
	for _, param := range params {
		goParams = append(goParams, pp.generateSingleGoCallParam(param))
	}

	return strings.Join(goParams, ", ")
}

func (pp *ParameterParser) generateSingleGoCallParam(param phpParameter) string {
	if param.isNullable {
		switch param.phpType {
		case "string":
			return fmt.Sprintf("%s_is_null ? NULL : %s", param.name, param.name)
		case "int":
			return fmt.Sprintf("%s_is_null ? NULL : &%s", param.name, param.name)
		case "float":
			return fmt.Sprintf("%s_is_null ? NULL : &%s", param.name, param.name)
		case "bool":
			return fmt.Sprintf("%s_is_null ? NULL : &%s", param.name, param.name)
		default:
			return param.name
		}
	} else {
		switch param.phpType {
		case "string":
			return param.name
		case "int":
			return fmt.Sprintf("(long) %s", param.name)
		case "float":
			return fmt.Sprintf("(double) %s", param.name)
		case "bool":
			return fmt.Sprintf("(int) %s", param.name)
		default:
			return param.name
		}
	}
}
