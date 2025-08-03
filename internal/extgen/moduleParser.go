package extgen

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var phpModuleParser = regexp.MustCompile(`//\s*export_php:module\s*(.*)`)

// phpModule represents a PHP module with optional init and shutdown functions
type phpModule struct {
	InitFunc     string // Name of the init function
	ShutdownFunc string // Name of the shutdown function
}

// ModuleParser parses PHP module directives from Go source files
type ModuleParser struct{}

// parse parses the source file for PHP module directives
func (mp *ModuleParser) parse(filename string) (*phpModule, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := phpModuleParser.FindStringSubmatch(line); matches != nil {
			moduleInfo := strings.TrimSpace(matches[1])
			return mp.parseModuleInfo(moduleInfo)
		}
	}

	// No module directive found
	return nil, nil
}

// parseModuleInfo parses the module info string to extract init and shutdown function names
func (mp *ModuleParser) parseModuleInfo(moduleInfo string) (*phpModule, error) {
	module := &phpModule{}
	
	// Split the module info by commas
	parts := strings.Split(moduleInfo, ",")
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Split each part by equals sign
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			continue
		}
		
		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])
		
		switch key {
		case "init":
			module.InitFunc = value
		case "shutdown":
			module.ShutdownFunc = value
		}
	}
	
	return module, nil
}
