package extgen

type SourceParser struct{}

// EXPERIMENTAL
func (p *SourceParser) ParseFunctions(filename string) ([]PHPFunction, error) {
	functionParser := NewFuncParserDefRegex()
	return functionParser.parse(filename)
}

// EXPERIMENTAL
func (p *SourceParser) ParseClasses(filename string) ([]PHPClass, error) {
	classParser := ClassParser{}
	return classParser.parse(filename)
}

// EXPERIMENTAL
func (p *SourceParser) ParseConstants(filename string) ([]PHPConstant, error) {
	constantParser := NewConstantParserWithDefRegex()
	return constantParser.parse(filename)
}
