package extgen

import (
	"strconv"
	"strings"
)

type phpFunction struct {
	Name             string
	Signature        string
	GoFunction       string
	Params           []phpParameter
	ReturnType       string
	IsReturnNullable bool
	lineNumber       int
}

type phpParameter struct {
	Name         string
	PhpType      string
	IsNullable   bool
	DefaultValue string
	HasDefault   bool
}

type phpClass struct {
	Name       string
	GoStruct   string
	Properties []phpClassProperty
	Methods    []phpClassMethod
}

type phpClassMethod struct {
	Name             string
	PhpName          string
	Signature        string
	GoFunction       string
	Wrapper          string
	Params           []phpParameter
	ReturnType       string
	isReturnNullable bool
	lineNumber       int
	ClassName        string // used by the "//export_php:method" directive
}

type phpClassProperty struct {
	Name       string
	PhpType    string
	GoType     string
	IsNullable bool
}

type phpConstant struct {
	Name       string
	Value      string
	PhpType    string // "int", "string", "bool", "float"
	IsIota     bool
	lineNumber int
	ClassName  string // empty for global constants, set for class constants
}

// CValue returns the constant value in C-compatible format
func (c phpConstant) CValue() string {
	if c.PhpType != "int" {
		return c.Value
	}

	if strings.HasPrefix(c.Value, "0o") {
		if val, err := strconv.ParseInt(c.Value, 0, 64); err == nil {
			return strconv.FormatInt(val, 10)
		}
	}

	return c.Value
}
