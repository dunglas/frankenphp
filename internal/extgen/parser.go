package extgen

type SourceParser struct{}

func (p *SourceParser) ParseFunctions(filename string) ([]phpFunction, error) {
	functionParser := &FuncParser{}
	return functionParser.parse(filename)
}

func (p *SourceParser) ParseClasses(filename string) ([]phpClass, error) {
	classParser := classParser{}
	return classParser.parse(filename)
}

func (p *SourceParser) ParseConstants(filename string) ([]phpConstant, error) {
	constantParser := &ConstantParser{}
	return constantParser.parse(filename)
}

func (p *SourceParser) ParseNamespace(filename string) (string, error) {
	namespaceParser := NamespaceParser{}
	return namespaceParser.parse(filename)
}

func (p *SourceParser) ParseModule(filename string) (*phpModule, error) {
	moduleParser := &ModuleParser{}
	return moduleParser.parse(filename)
}
