package extgen

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/extension.c.tpl
var cFileContent string

type cFileGenerator struct {
	generator *Generator
}

type cTemplateData struct {
	BaseName  string
	Functions []phpFunction
	Classes   []phpClass
	Constants []phpConstant
	Version   string
}

func (cg *cFileGenerator) generate() error {
	filename := filepath.Join(cg.generator.BuildDir, cg.generator.BaseName+".c")
	content, err := cg.buildContent()
	if err != nil {
		return err
	}
	return WriteFile(filename, content)
}

func (cg *cFileGenerator) buildContent() (string, error) {
	var builder strings.Builder

	templateContent, err := cg.getTemplateContent()
	if err != nil {
		return "", err
	}
	builder.WriteString(templateContent)

	for _, fn := range cg.generator.Functions {
		fnGen := PHPFuncGenerator{paramParser: &ParameterParser{}}
		builder.WriteString(fnGen.generate(fn))
	}

	return builder.String(), nil
}

func (cg *cFileGenerator) getTemplateContent() (string, error) {
	tmpl, err := template.New("cfile").Funcs(template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	}).Parse(cFileContent)

	if err != nil {
		return "", err
	}

	data := cTemplateData{
		BaseName:  cg.generator.BaseName,
		Functions: cg.generator.Functions,
		Classes:   cg.generator.Classes,
		Constants: cg.generator.Constants,
		Version:   "1.0.0",
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
