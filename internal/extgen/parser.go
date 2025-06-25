package extgen

type SourceParser struct{}

// EXPERIMENTAL
func (p *SourceParser) ParseFunctions(filename string) ([]phpFunction, error) {
	functionParser := NewFuncParserDefRegex()
	return functionParser.parse(filename)
}

// EXPERIMENTAL
func (p *SourceParser) ParseClasses(filename string) ([]phpClass, error) {
	classParser := classParser{}
	return classParser.parse(filename)
}

// EXPERIMENTAL
func (p *SourceParser) ParseConstants(filename string) ([]phpConstant, error) {
	constantParser := NewConstantParserWithDefRegex()
	return constantParser.parse(filename)
}
