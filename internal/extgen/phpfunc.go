package extgen

import (
	"fmt"
	"strings"
)

type PHPFuncGenerator struct {
	paramParser *ParameterParser
}

func (pfg *PHPFuncGenerator) generate(fn phpFunction) string {
	var builder strings.Builder

	paramInfo := pfg.paramParser.analyzeParameters(fn.params)

	builder.WriteString(fmt.Sprintf("PHP_FUNCTION(%s)\n{\n", fn.name))

	if decl := pfg.paramParser.generateParamDeclarations(fn.params); decl != "" {
		builder.WriteString(decl + "\n")
	}

	builder.WriteString(pfg.paramParser.generateParamParsing(fn.params, paramInfo.RequiredCount) + "\n")

	builder.WriteString(pfg.generateGoCall(fn) + "\n")

	if returnCode := pfg.generateReturnCode(fn.returnType); returnCode != "" {
		builder.WriteString(returnCode + "\n")
	}

	builder.WriteString("}\n\n")

	return builder.String()
}

func (pfg *PHPFuncGenerator) generateGoCall(fn phpFunction) string {
	callParams := pfg.paramParser.generateGoCallParams(fn.params)

	if fn.returnType == "void" {
		return fmt.Sprintf("    %s(%s);", fn.name, callParams)
	}

	if fn.returnType == "string" {
		return fmt.Sprintf("    zend_string *result = %s(%s);", fn.name, callParams)
	}

	return fmt.Sprintf("    %s result = %s(%s);", pfg.getCReturnType(fn.returnType), fn.name, callParams)
}

func (pfg *PHPFuncGenerator) getCReturnType(returnType string) string {
	switch returnType {
	case "string":
		return "zend_string*"
	case "int":
		return "long"
	case "float":
		return "double"
	case "bool":
		return "int"
	default:
		return "void"
	}
}

func (pfg *PHPFuncGenerator) generateReturnCode(returnType string) string {
	switch returnType {
	case "string":
		return `    if (result) {
        RETURN_STR(result);
    } else {
        RETURN_EMPTY_STRING();
    }`
	case "int":
		return `    RETURN_LONG(result);`
	case "float":
		return `    RETURN_DOUBLE(result);`
	case "bool":
		return `    RETURN_BOOL(result);`
	default:
		return ""
	}
}
