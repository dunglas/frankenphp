package extgen

import (
	_ "embed"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/stub.php.tpl
var templateContent string

type StubGenerator struct {
	Generator *Generator
}

func (sg *StubGenerator) generate() error {
	filename := filepath.Join(sg.Generator.BuildDir, sg.Generator.BaseName+".stub.php")
	content, err := sg.buildContent()
	if err != nil {
		return err
	}
	return WriteFile(filename, content)
}

func (sg *StubGenerator) buildContent() (string, error) {
	tmpl, err := template.New("stub.php.tpl").Funcs(template.FuncMap{
		"phpType": getPhpTypeAnnotation,
	}).Parse(templateContent)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	err = tmpl.Execute(&buf, sg.Generator)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getPhpTypeAnnotation converts Go constant type to PHP type annotation
func getPhpTypeAnnotation(goType string) string {
	switch goType {
	case "string", "bool", "float", "int":
		return goType
	default:
		return "int" // fallback
	}
}
