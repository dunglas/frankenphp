package extgen

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

type SourceAnalyzer struct{}

func (sa *SourceAnalyzer) analyze(filename string) (imports []string, internalFunctions []string, err error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing file: %w", err)
	}

	for _, imp := range node.Imports {
		if imp.Path != nil {
			importPath := imp.Path.Value
			if imp.Name != nil {
				imports = append(imports, fmt.Sprintf("%s %s", imp.Name.Name, importPath))
			} else {
				imports = append(imports, importPath)
			}
		}
	}

	sourceContent, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("reading source file: %w", err)
	}

	internalFunctions = sa.extractInternalFunctions(string(sourceContent))

	return imports, internalFunctions, nil
}

func (sa *SourceAnalyzer) extractInternalFunctions(content string) []string {
	lines := strings.Split(content, "\n")
	var functions []string
	var currentFunc strings.Builder
	var inFunction bool
	var braceCount int
	var hasPHPFunc bool

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "func ") && !inFunction {
			inFunction = true
			braceCount = 0
			hasPHPFunc = false
			currentFunc.Reset()

			// look backwards for export_php comment
			for j := i - 1; j >= 0 && j >= i-5; j-- {
				prevLine := strings.TrimSpace(lines[j])
				if prevLine == "" {
					continue
				}
				if strings.Contains(prevLine, "export_php:") {
					hasPHPFunc = true
					break
				}
				if !strings.HasPrefix(prevLine, "//") {
					break
				}
			}
		}

		if inFunction {
			currentFunc.WriteString(line + "\n")

			for _, char := range line {
				switch char {
				case '{':
					braceCount++
				case '}':
					braceCount--
				}
			}

			if braceCount == 0 && strings.Contains(line, "}") {
				funcContent := currentFunc.String()

				if !hasPHPFunc {
					functions = append(functions, strings.TrimSpace(funcContent))
				}

				inFunction = false
				currentFunc.Reset()
			}
		}
	}

	return functions
}
