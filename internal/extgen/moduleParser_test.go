package extgen

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModuleParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *phpModule
	}{
		{
			name: "both init and shutdown",
			input: `package main

//export_php:module init=initializeModule, shutdown=cleanupModule
func initializeModule() {
	// Initialization code
}

func cleanupModule() {
	// Cleanup code
}`,
			expected: &phpModule{
				InitFunc:     "initializeModule",
				ShutdownFunc: "cleanupModule",
			},
		},
		{
			name: "only init function",
			input: `package main

//export_php:module init=initializeModule
func initializeModule() {
	// Initialization code
}`,
			expected: &phpModule{
				InitFunc:     "initializeModule",
				ShutdownFunc: "",
			},
		},
		{
			name: "only shutdown function",
			input: `package main

//export_php:module shutdown=cleanupModule
func cleanupModule() {
	// Cleanup code
}`,
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "cleanupModule",
			},
		},
		{
			name: "with extra whitespace",
			input: `package main

//export_php:module   init = initModule ,  shutdown = shutdownModule  
func initModule() {
	// Initialization code
}

func shutdownModule() {
	// Cleanup code
}`,
			expected: &phpModule{
				InitFunc:     "initModule",
				ShutdownFunc: "shutdownModule",
			},
		},
		{
			name: "no module directive",
			input: `package main

func regularFunction() {
	// Just a regular Go function
}`,
			expected: nil,
		},
		{
			name: "empty module directive",
			input: `package main

//export_php:module
func someFunction() {
	// Some code
}`,
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "",
			},
		},
		{
			name: "invalid module directive format",
			input: `package main

//export_php:module init:initFunc, shutdown:shutdownFunc
func initFunc() {
	// Initialization code
}

func shutdownFunc() {
	// Cleanup code
}`,
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "",
			},
		},
		{
			name: "unrecognized keys",
			input: `package main

//export_php:module start=startFunc, stop=stopFunc
func startFunc() {
	// Start code
}

func stopFunc() {
	// Stop code
}`,
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0644))

			parser := &ModuleParser{}
			module, err := parser.parse(fileName)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, module, "parse() should return nil for no module directive")
			} else {
				assert.NotNil(t, module, "parse() should not return nil")
				assert.Equal(t, tt.expected.InitFunc, module.InitFunc, "InitFunc mismatch")
				assert.Equal(t, tt.expected.ShutdownFunc, module.ShutdownFunc, "ShutdownFunc mismatch")
			}
		})
	}
}

func TestParseModuleInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     string
		expected *phpModule
	}{
		{
			name: "both init and shutdown",
			info: "init=initializeModule, shutdown=cleanupModule",
			expected: &phpModule{
				InitFunc:     "initializeModule",
				ShutdownFunc: "cleanupModule",
			},
		},
		{
			name: "only init",
			info: "init=initializeModule",
			expected: &phpModule{
				InitFunc:     "initializeModule",
				ShutdownFunc: "",
			},
		},
		{
			name: "only shutdown",
			info: "shutdown=cleanupModule",
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "cleanupModule",
			},
		},
		{
			name: "with extra whitespace",
			info: "  init = initModule ,  shutdown = shutdownModule  ",
			expected: &phpModule{
				InitFunc:     "initModule",
				ShutdownFunc: "shutdownModule",
			},
		},
		{
			name: "empty string",
			info: "",
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "",
			},
		},
		{
			name: "invalid format - no equals sign",
			info: "init initFunc, shutdown shutdownFunc",
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "",
			},
		},
		{
			name: "invalid format - wrong separator",
			info: "init=initFunc; shutdown=shutdownFunc",
			expected: &phpModule{
				InitFunc:     "initFunc",
				ShutdownFunc: "",
			},
		},
		{
			name: "unrecognized keys",
			info: "start=startFunc, stop=stopFunc",
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "",
			},
		},
		{
			name: "mixed valid and invalid",
			info: "init=initFunc, invalid, shutdown=shutdownFunc",
			expected: &phpModule{
				InitFunc:     "initFunc",
				ShutdownFunc: "shutdownFunc",
			},
		},
		{
			name: "reversed order",
			info: "shutdown=shutdownFunc, init=initFunc",
			expected: &phpModule{
				InitFunc:     "initFunc",
				ShutdownFunc: "shutdownFunc",
			},
		},
	}

	parser := &ModuleParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := parser.parseModuleInfo(tt.info)
			
			assert.NoError(t, err, "parseModuleInfo() unexpected error")
			assert.NotNil(t, module, "parseModuleInfo() should not return nil")
			assert.Equal(t, tt.expected.InitFunc, module.InitFunc, "InitFunc mismatch")
			assert.Equal(t, tt.expected.ShutdownFunc, module.ShutdownFunc, "ShutdownFunc mismatch")
		})
	}
}

func TestModuleParserFileErrors(t *testing.T) {
	parser := &ModuleParser{}
	
	// Test with non-existent file
	module, err := parser.parse("non_existent_file.go")
	assert.Error(t, err, "parse() should return error for non-existent file")
	assert.Nil(t, module, "parse() should return nil for non-existent file")
}