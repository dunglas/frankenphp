package extgen

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"text/template"
)

//go:embed templates/README.md.tpl
var docFileContent string

type DocumentationGenerator struct {
	generator *Generator
}

type DocTemplateData struct {
	BaseName  string
	Functions []phpFunction
	Classes   []phpClass
}

func (dg *DocumentationGenerator) generate() error {
	filename := filepath.Join(dg.generator.BuildDir, "README.md")
	content, err := dg.generateMarkdown()
	if err != nil {
		return err
	}
	return WriteFile(filename, content)
}

func (dg *DocumentationGenerator) generateMarkdown() (string, error) {
	tmpl, err := template.New("readme").Parse(docFileContent)
	if err != nil {
		return "", err
	}

	data := DocTemplateData{
		BaseName:  dg.generator.BaseName,
		Functions: dg.generator.Functions,
		Classes:   dg.generator.Classes,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
