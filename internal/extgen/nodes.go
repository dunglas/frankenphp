package extgen

import (
	"strconv"
	"strings"
)

type PHPFunction struct {
	Name             string
	Signature        string
	GoFunction       string
	Params           []Parameter
	ReturnType       string
	IsReturnNullable bool
	LineNumber       int
}

type Parameter struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	HasDefault   bool
}

type PHPClass struct {
	Name       string
	GoStruct   string
	Properties []ClassProperty
	Methods    []ClassMethod
}

type ClassMethod struct {
	Name             string
	PHPName          string
	Signature        string
	GoFunction       string
	Params           []Parameter
	ReturnType       string
	IsReturnNullable bool
	LineNumber       int
	ClassName        string // used by the "//export_php:method" directive
}

type ClassProperty struct {
	Name       string
	Type       string
	GoType     string
	IsNullable bool
}

type PHPConstant struct {
	Name       string
	Value      string
	Type       string // "int", "string", "bool", "float"
	IsIota     bool
	LineNumber int
	ClassName  string // empty for global constants, set for class constants
}

// CValue returns the constant value in C-compatible format
func (c PHPConstant) CValue() string {
	if c.Type != "int" {
		return c.Value
	}

	if strings.HasPrefix(c.Value, "0o") {
		if val, err := strconv.ParseInt(c.Value, 0, 64); err == nil {
			return strconv.FormatInt(val, 10)
		}
	}

	return c.Value
}
