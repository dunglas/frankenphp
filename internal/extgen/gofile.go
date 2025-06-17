package extgen

import (
	"fmt"
	"path/filepath"
	"strings"
)

type GoFileGenerator struct {
	generator *Generator
}

func (gg *GoFileGenerator) generate() error {
	filename := filepath.Join(gg.generator.BuildDir, gg.generator.BaseName+".go")
	content, err := gg.buildContent()
	if err != nil {
		return fmt.Errorf("building Go file content: %w", err)
	}
	return WriteFile(filename, content)
}

func (gg *GoFileGenerator) buildContent() (string, error) {
	sourceAnalyzer := SourceAnalyzer{}
	imports, internalFunctions, err := sourceAnalyzer.analyze(gg.generator.SourceFile)
	if err != nil {
		return "", fmt.Errorf("analyzing source file: %w", err)
	}

	var builder strings.Builder

	cleanPackageName := SanitizePackageName(gg.generator.BaseName)
	builder.WriteString(fmt.Sprintf(`package %s

/*
#include <stdlib.h>
#include "%s.h"
*/
import "C"
import "runtime/cgo"
`, cleanPackageName, gg.generator.BaseName))

	for _, imp := range imports {
		if imp == `"C"` {
			continue
		}

		builder.WriteString(fmt.Sprintf("import %s\n", imp))
	}

	builder.WriteString(`
func init() {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.ext_module_entry))
}
`)

	for _, constant := range gg.generator.Constants {
		builder.WriteString(fmt.Sprintf("const %s = %s\n", constant.Name, constant.Value))
	}

	if len(gg.generator.Constants) > 0 {
		builder.WriteString("\n")
	}

	for _, internalFunc := range internalFunctions {
		builder.WriteString(internalFunc + "\n\n")
	}

	for _, fn := range gg.generator.Functions {
		builder.WriteString(fmt.Sprintf("//export %s\n%s\n", fn.Name, fn.goFunction))
	}

	for _, class := range gg.generator.Classes {
		builder.WriteString(fmt.Sprintf("type %s struct {\n", class.GoStruct))
		for _, prop := range class.Properties {
			builder.WriteString(fmt.Sprintf("	%s %s\n", prop.Name, prop.goType))
		}
		builder.WriteString("}\n\n")
	}

	if len(gg.generator.Classes) > 0 {
		builder.WriteString(`
//export registerGoObject
func registerGoObject(obj interface{}) C.uintptr_t {
	handle := cgo.NewHandle(obj)
	return C.uintptr_t(handle)
}

//export getGoObject
func getGoObject(handle C.uintptr_t) interface{} {
	h := cgo.Handle(handle)
	return h.value()
}

//export removeGoObject
func removeGoObject(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	h.Delete()
}

`)
	}

	for _, class := range gg.generator.Classes {
		builder.WriteString(fmt.Sprintf(`//export create_%s_object
func create_%s_object() C.uintptr_t {
	obj := &%s{}
	return registerGoObject(obj)
}

`, class.GoStruct, class.GoStruct, class.GoStruct))

		for _, method := range class.Methods {
			if method.goFunction != "" {
				builder.WriteString(method.goFunction)
				builder.WriteString("\n\n")
			}
		}

		for _, method := range class.Methods {
			builder.WriteString(fmt.Sprintf("//export %s_wrapper\n", method.Name))
			builder.WriteString(gg.generateMethodWrapper(method, class))
			builder.WriteString("\n")
		}
	}

	return builder.String(), nil
}

func (gg *GoFileGenerator) generateMethodWrapper(method phpClassMethod, class phpClass) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("func %s_wrapper(handle C.uintptr_t", method.Name))

	for _, param := range method.Params {
		if param.PhpType == "string" {
			builder.WriteString(fmt.Sprintf(", %s *C.zend_string", param.Name))
		} else {
			goType := gg.phpTypeToGoType(param.PhpType)
			if param.IsNullable {
				goType = "*" + goType
			}
			builder.WriteString(fmt.Sprintf(", %s %s", param.Name, goType))
		}
	}

	if method.ReturnType != "void" {
		if method.ReturnType == "string" {
			builder.WriteString(") unsafe.Pointer {\n")
		} else {
			goReturnType := gg.phpTypeToGoType(method.ReturnType)
			builder.WriteString(fmt.Sprintf(") %s {\n", goReturnType))
		}
	} else {
		builder.WriteString(") {\n")
	}

	builder.WriteString("	obj := getGoObject(handle)\n")
	builder.WriteString("	if obj == nil {\n")
	if method.ReturnType != "void" {
		if method.ReturnType == "string" {
			builder.WriteString("		return nil\n")
		} else {
			builder.WriteString(fmt.Sprintf("		var zero %s\n", gg.phpTypeToGoType(method.ReturnType)))
			builder.WriteString("		return zero\n")
		}
	} else {
		builder.WriteString("		return\n")
	}
	builder.WriteString("	}\n")
	builder.WriteString(fmt.Sprintf("	structObj := obj.(*%s)\n", class.GoStruct))

	builder.WriteString("	")
	if method.ReturnType != "void" {
		builder.WriteString("return ")
	}

	builder.WriteString(fmt.Sprintf("structObj.%s(", gg.goMethodName(method.Name)))

	for i, param := range method.Params {
		if i > 0 {
			builder.WriteString(", ")
		}

		builder.WriteString(param.Name)
	}

	builder.WriteString(")\n")
	builder.WriteString("}")

	return builder.String()
}

type GoMethodSignature struct {
	MethodName string
	Params     []GoParameter
	ReturnType string
}

type GoParameter struct {
	Name string
	Type string
}

func (gg *GoFileGenerator) parseGoMethodSignature(goFunction string) (*GoMethodSignature, error) {
	lines := strings.Split(goFunction, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty function")
	}

	funcLine := strings.TrimSpace(lines[0])

	if !strings.HasPrefix(funcLine, "func ") {
		return nil, fmt.Errorf("not a function")
	}

	parts := strings.Split(funcLine, ")")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid function signature")
	}

	methodPart := strings.TrimSpace(parts[1])

	spaceIndex := strings.Index(methodPart, "(")
	if spaceIndex == -1 {
		return nil, fmt.Errorf("no parameters found")
	}

	methodName := strings.TrimSpace(methodPart[:spaceIndex])

	paramStart := strings.Index(methodPart, "(")
	paramEnd := strings.LastIndex(methodPart, ")")
	if paramStart == -1 || paramEnd == -1 || paramStart >= paramEnd {
		return nil, fmt.Errorf("invalid parameter section")
	}

	paramSection := methodPart[paramStart+1 : paramEnd]
	var params []GoParameter

	if strings.TrimSpace(paramSection) != "" {
		paramParts := strings.Split(paramSection, ",")
		for _, paramPart := range paramParts {
			paramPart = strings.TrimSpace(paramPart)
			if paramPart == "" {
				continue
			}

			parts := strings.Fields(paramPart)
			if len(parts) >= 2 {
				params = append(params, GoParameter{
					Name: parts[0],
					Type: strings.Join(parts[1:], " "),
				})
			}
		}
	}

	returnType := ""
	if strings.Contains(methodPart, ") ") && !strings.HasSuffix(methodPart, ") {") {
		afterParen := strings.Split(methodPart, ") ")
		if len(afterParen) > 1 {
			returnPart := strings.TrimSpace(afterParen[1])
			if strings.HasSuffix(returnPart, " {") {
				returnType = strings.TrimSpace(returnPart[:len(returnPart)-2])
			}
		}
	}

	return &GoMethodSignature{
		MethodName: methodName,
		Params:     params,
		ReturnType: returnType,
	}, nil
}

func (gg *GoFileGenerator) generateMethodWrapperFallback(method phpClassMethod, class phpClass) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("func %s_wrapper(objectID uint64", method.Name))

	for _, param := range method.Params {
		goType := gg.phpTypeToGoType(param.PhpType)
		builder.WriteString(fmt.Sprintf(", %s %s", param.Name, goType))
	}

	if method.ReturnType != "void" {
		goReturnType := gg.phpTypeToGoType(method.ReturnType)
		builder.WriteString(fmt.Sprintf(") %s {\n", goReturnType))
	} else {
		builder.WriteString(") {\n")
	}

	builder.WriteString("	objPtr := getGoObject(objectID)\n")
	builder.WriteString(fmt.Sprintf("	obj := (*%s)(objPtr)\n", class.GoStruct))

	builder.WriteString("	")
	if method.ReturnType != "void" {
		builder.WriteString("return ")
	}

	builder.WriteString(fmt.Sprintf("structObj.%s(", gg.goMethodName(method.Name)))

	for i, param := range method.Params {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(param.Name)
	}

	builder.WriteString(")\n")
	builder.WriteString("}")

	return builder.String()
}

func (gg *GoFileGenerator) phpTypeToGoType(phpType string) string {
	typeMap := map[string]string{
		"string": "string",
		"int":    "int64",
		"float":  "float64",
		"bool":   "bool",
		"array":  "[]interface{}",
		"mixed":  "interface{}",
		"void":   "",
	}

	if goType, exists := typeMap[phpType]; exists {
		return goType
	}

	return "interface{}"
}

func (gg *GoFileGenerator) goMethodName(phpMethodName string) string {
	if len(phpMethodName) == 0 {
		return phpMethodName
	}

	return strings.ToUpper(phpMethodName[:1]) + phpMethodName[1:]
}
