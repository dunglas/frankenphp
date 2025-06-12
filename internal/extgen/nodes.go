package extgen

import (
	"strconv"
	"strings"
)

type phpFunction struct {
	name             string
	signature        string
	goFunction       string
	params           []phpParameter
	returnType       string
	isReturnNullable bool
	lineNumber       int
}

type phpParameter struct {
	name         string
	phpType      string
	isNullable   bool
	defaultValue string
	hasDefault   bool
}

type phpClass struct {
	name       string
	goStruct   string
	properties []phpClassProperty
	methods    []phpClassMethod
}

type phpClassMethod struct {
	name             string
	phpName          string
	signature        string
	goFunction       string
	params           []phpParameter
	returnType       string
	isReturnNullable bool
	lineNumber       int
	className        string // used by the "//export_php:method" directive
}

type phpClassProperty struct {
	name       string
	phpType    string
	goType     string
	isNullable bool
}

type phpConstant struct {
	name       string
	value      string
	phpType    string // "int", "string", "bool", "float"
	isIota     bool
	lineNumber int
	className  string // empty for global constants, set for class constants
}

// CValue returns the constant value in C-compatible format
func (c phpConstant) CValue() string {
	if c.phpType != "int" {
		return c.value
	}

	if strings.HasPrefix(c.value, "0o") {
		if val, err := strconv.ParseInt(c.value, 0, 64); err == nil {
			return strconv.FormatInt(val, 10)
		}
	}

	return c.value
}
