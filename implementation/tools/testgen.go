// Package tools - Test Generator Tool
// Generates unit tests for source files with language auto-detection.

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// testGenerator generates unit tests for source files.
type testGenerator struct{}

// newTestGenerator creates a new test generator.
func newTestGenerator() *testGenerator {
	return &testGenerator{}
}

// executeTestGen generates unit tests for a source file.
func (te *ToolExecutor) executeTestGen(params map[string]interface{}) *ToolResult {
	gen := newTestGenerator()

	// Get required parameter: path
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Check if file exists
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("file not found: %s", path),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %v", err),
		}
	}

	// Determine language
	language, ok := params["language"].(string)
	if !ok || language == "" {
		language = toolsDetectLanguage(path)
	}
	language = strings.ToLower(language)

	// Determine test framework
	framework, ok := params["test_framework"].(string)
	if !ok || framework == "" {
		switch language {
		case "go":
			framework = "testing"
		case "python":
			framework = "pytest"
		case "javascript", "typescript":
			framework = "jest"
		default:
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("unsupported language: %s. Supported: go, python, javascript, typescript", language),
			}
		}
	}

	// Determine output path
	output, ok := params["output"].(string)
	if !ok || output == "" {
		output = gen.defaultOutputPath(path, language)
	}

	// Generate tests
	var testContent string
	switch language {
	case "go":
		testContent, err = gen.generateGoTests(string(content), path, framework)
	case "python":
		testContent, err = gen.generatePythonTests(string(content), path, framework)
	case "javascript", "typescript":
		testContent, err = gen.generateJSTests(string(content), path, language, framework)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported language: %s", language),
		}
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to generate tests: %v", err),
		}
	}

	// Create parent directories if needed
	dir := filepath.Dir(output)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("cannot create directory: %v", err),
			}
		}
	}

	// Write test file
	if err := os.WriteFile(output, []byte(testContent), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write test file: %v", err),
		}
	}

	testCount := countGeneratedFunctions(string(content), language)

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Generated %d test case(s) for %s → %s", testCount, path, output),
		Path:    output,
		Extra: map[string]interface{}{
			"language":      language,
			"framework":     framework,
			"sourcePath":    path,
			"testPath":      output,
			"testCount":     testCount,
			"contentLength": len(testContent),
		},
	}
}

// toolsDetectLanguage is an alias to the detectLanguage function in tools.go.
func toolsDetectLanguage(path string) string {
	return detectLanguage(path)
}

// defaultOutputPath generates the default test file path.
func (g *testGenerator) defaultOutputPath(sourcePath string, language string) string {
	base := filepath.Base(sourcePath)
	dir := filepath.Dir(sourcePath)

	switch language {
	case "go":
		// Go: foo.go → foo_test.go
		name := strings.TrimSuffix(base, ".go")
		return filepath.Join(dir, name+"_test.go")
	case "python":
		// Python: foo.py → test_foo.py
		name := strings.TrimSuffix(base, ".py")
		return filepath.Join(dir, "test_"+name+".py")
	case "javascript", "typescript":
		// JS/TS: foo.js → foo.test.js or foo.spec.js
		name := strings.TrimSuffix(base, filepath.Ext(base))
		ext := filepath.Ext(base)
		return filepath.Join(dir, name+".test"+ext)
	}

	return sourcePath + ".test"
}

// ========================
// Go Test Generation
// ========================

// goFunction represents a function extracted from Go source.
type goFunction struct {
	Name        string
	PackageName string
	Receiver    string // for methods
	Params      string
	Results     string
	LineNum     int
	IsExported  bool
	Comment     string
}

// generateGoTests generates Go unit tests using the testing package.
func (g *testGenerator) generateGoTests(source string, sourcePath string, framework string) (string, error) {
	// Extract package name
	pkgName := extractPackageName(source)
	if pkgName == "" {
		pkgName = "main"
	}

	// Extract functions and methods
	funcs := g.extractGoFunctions(source)

	// Generate imports
	var imports []string
	imports = append(imports, "\"testing\"")

	// Check if we need any additional imports based on types used
	imports = append(imports, g.extractGoImports(source))

	// Build test file
	var sb strings.Builder

	// Package declaration
	sb.WriteString(fmt.Sprintf("package %s\n\n", pkgName))

	// Imports
	if len(imports) > 0 {
		sb.WriteString("import (\n")
		for _, imp := range imports {
			if imp != "" {
				sb.WriteString("\t" + imp + "\n")
			}
		}
		sb.WriteString(")\n\n")
	}

	// Generate tests for each function/method
	for _, fn := range funcs {
		if fn.Comment != "" {
			sb.WriteString(fmt.Sprintf("// %s\n", fn.Comment))
		}
		g.generateGoTestFunction(&sb, fn, pkgName)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// extractPackageName extracts the package name from Go source.
func extractPackageName(source string) string {
	re := regexp.MustCompile(`^package\s+(\w+)`)
	matches := re.FindStringSubmatch(source)
	if len(matches) > 1 {
		return matches[1]
	}
	return "main"
}

// extractGoImports extracts necessary import paths from Go source.
func (g *testGenerator) extractGoImports(source string) string {
	// Check for common imports used in test scenarios
	var imports []string

	// Check if source uses fmt (for assertions in tests)
	if strings.Contains(source, "fmt.") {
		imports = append(imports, "\"fmt\"")
	}
	// Check for slices usage
	if strings.Contains(source, "[]") && !strings.Contains(source, "import") {
		// No extra import needed for slices
	}
	// Check for context
	if strings.Contains(source, "context.") {
		imports = append(imports, "\"context\"")
	}
	// Check for errors
	if strings.Contains(source, "errors.") {
		imports = append(imports, "\"errors\"")
	}
	// Check for io
	if strings.Contains(source, "io.") {
		imports = append(imports, "\"io\"")
	}
	// Check for net/http
	if strings.Contains(source, "net/http") {
		imports = append(imports, "\"net/http\"")
	}
	// Check for encoding/json
	if strings.Contains(source, "encoding/json") {
		imports = append(imports, "\"encoding/json\"")
	}
	// Check for log
	if strings.Contains(source, "log.") {
		imports = append(imports, "\"log\"")
	}
	// Check for os
	if strings.Contains(source, "os.") {
		imports = append(imports, "\"os\"")
	}
	// Check for strings
	if strings.Contains(source, "strings.") {
		imports = append(imports, "\"strings\"")
	}
	// Check for math
	if strings.Contains(source, "math.") {
		imports = append(imports, "\"math\"")
	}
	// Check for time
	if strings.Contains(source, "time.") {
		imports = append(imports, "\"time\"")
	}
	// Check for sync
	if strings.Contains(source, "sync.") {
		imports = append(imports, "\"sync\"")
	}

	return strings.Join(imports, "\n")
}

// extractGoFunctions extracts exported functions and methods from Go source.
func (g *testGenerator) extractGoFunctions(source string) []goFunction {
	var funcs []goFunction

	// Pattern for exported functions: func Name(params) (results)
	funcPattern := regexp.MustCompile(`^func\s+(\(*\w+)\s+([A-Z]\w*)\s*\(([^)]*)\)\s*(\([^)]*\))?\s*\{?`)
	lines := strings.Split(source, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Match functions with receiver
		matches := funcPattern.FindStringSubmatch(line)
		if matches != nil {
			receiver := matches[1]
			if receiver == "func" {
				receiver = ""
			}
			name := matches[2]
			params := strings.TrimSpace(matches[3])
			results := strings.TrimSpace(matches[4])
			if results != "" {
				results = results[1 : len(results)-1] // Remove parentheses
			}

			// Get comment above the function
			comment := ""
			for j := i - 1; j >= 0 && j >= i-5; j-- {
				trimmed := strings.TrimSpace(lines[j])
				if strings.HasPrefix(trimmed, "//") {
					comment = strings.TrimPrefix(trimmed, "//")
					comment = strings.TrimSpace(comment)
				} else if trimmed == "" {
					break
				} else {
					break
				}
			}

			funcs = append(funcs, goFunction{
				Name:        name,
				Receiver:    receiver,
				Params:      params,
				Results:     results,
				LineNum:     i + 1,
				IsExported:  true,
				Comment:     comment,
			})
		}
	}

	// Also try to find unexported functions and generate test variants
	unexportedPattern := regexp.MustCompile(`^func\s+([a-z]\w*)\s*\(([^)]*)\)\s*(\([^)]*\))?\s*\{?`)
	for i, line := range lines {
		line = strings.TrimSpace(line)
		matches := unexportedPattern.FindStringSubmatch(line)
		if matches != nil {
			name := matches[1]
			params := strings.TrimSpace(matches[2])
			results := strings.TrimSpace(matches[3])
			if results != "" {
				results = results[1 : len(results)-1]
			}

			comment := ""
			for j := i - 1; j >= 0 && j >= i-5; j-- {
				trimmed := strings.TrimSpace(lines[j])
				if strings.HasPrefix(trimmed, "//") {
					comment = strings.TrimPrefix(trimmed, "//")
					comment = strings.TrimSpace(comment)
				} else {
					break
				}
			}

			funcs = append(funcs, goFunction{
				Name:        name,
				Receiver:    "",
				Params:      params,
				Results:     results,
				LineNum:     i + 1,
				IsExported:  false,
				Comment:     comment,
			})
		}
	}

	return funcs
}

// generateGoTestFunction generates a test function for a single Go function/method.
func (g *testGenerator) generateGoTestFunction(sb *strings.Builder, fn goFunction, pkgName string) {
	testName := "Test" + fn.Name
	if fn.Receiver != "" {
		// Method: TestStruct_Method
		testName = "Test" + fn.Receiver + "_" + fn.Name
	}

	sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", testName))

	// Parse parameters to generate test cases
	params := parseGoParams(fn.Params)
	results := parseGoParams(fn.Results)

	hasErrorReturn := len(results) > 0 && hasErrorType(results)
	hasBoolReturn := len(results) > 0 && hasBoolType(results)
	hasInterfaceReturn := len(results) > 0 && hasInterfaceType(results)

	// Generate table-driven test cases
	sb.WriteString("\t// Table-driven test cases\n")
	sb.WriteString("\ttests := []struct {\n")
	sb.WriteString("\t\tname   string\n")

	// Generate test case fields based on parameters
	for _, p := range params {
		fieldName := sanitizeIdentifier(p.Name)
		if len(params) > 1 {
			fieldName = strings.Title(fieldName)
		} else {
			fieldName = "input"
		}
		// Use proper Go type
		goType := goParamToType(p.Type)
		if goType == "" {
			goType = p.Type
		}
		sb.WriteString(fmt.Sprintf("\t\t%s %s\n", fieldName, goType))
	}

	sb.WriteString("\t}{\n")

	// Generate test case values based on parameter types
	g.generateGoTestCases(sb, params, hasErrorReturn, hasBoolReturn, hasInterfaceReturn)

	sb.WriteString("\t}\n\n")

	// Test loop
	sb.WriteString("\tfor _, tt := range tests {\n")
	sb.WriteString("\t\tt.Run(tt.name, func(t *testing.T) {\n")

	// Build call expression
	callExpr := g.generateGoCallExpression(fn)
	sb.WriteString("\t\t\t")

	if hasErrorReturn {
		sb.WriteString("result, err := ")
	} else if hasBoolReturn {
		sb.WriteString("result := ")
	} else if len(results) > 0 && !hasErrorReturn && !hasBoolReturn {
		sb.WriteString("result := ")
	} else {
		sb.WriteString("var result struct{}\n")
	}

	sb.WriteString(callExpr)
	sb.WriteString("\n")

	// Assertions based on return types
	g.generateGoAssertions(sb, fn, hasErrorReturn, hasBoolReturn)

	sb.WriteString("\t\t})\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")
}

// parseGoParams parses a parameter string like "name1 type1, name2 type2".
func parseGoParams(paramStr string) []struct {
	Name string
	Type string
} {
	var params []struct {
		Name string
		Type string
	}

	if paramStr == "" {
		return params
	}

	// Split by comma, handling nested types in parentheses
	var current string
	depth := 0
	for _, ch := range paramStr {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		}
		if ch == ',' && depth == 0 {
			p := parseGoParam(current)
			params = append(params, p)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if strings.TrimSpace(current) != "" {
		p := parseGoParam(current)
		params = append(params, p)
	}

	return params
}

// parseGoParam parses a single parameter like "name type" or "name ...type".
func parseGoParam(param string) struct {
	Name string
	Type string
} {
	param = strings.TrimSpace(param)
	parts := strings.Fields(param)
	if len(parts) == 0 {
		return struct{ Name, Type string }{}
	}

	// Last part is the type, everything before is the name
	name := strings.Join(parts[:len(parts)-1], " ")
	if name == "" {
		name = fmt.Sprintf("arg%d", len(parts))
	}
	typ := parts[len(parts)-1]

	// Handle variadic parameters
	if strings.HasPrefix(typ, "...") {
		typ = "[]" + typ[3:]
	}

	return struct{ Name, Type string }{Name: name, Type: typ}
}

// goParamToType converts a Go parameter type to a proper Go type string.
func goParamToType(typ string) string {
	typ = strings.TrimSpace(typ)
	// Handle pointer types
	if strings.HasPrefix(typ, "*") {
		return "*" + goParamToType(typ[1:])
	}
	// Handle slice types
	if strings.HasPrefix(typ, "[]") {
		return "[]" + goParamToType(typ[2:])
	}
	// Handle map types
	if strings.HasPrefix(typ, "map[") {
		return typ
	}
	// Handle interface types
	if typ == "interface{}" || typ == "any" {
		return "interface{}"
	}
	// Handle basic types
	return typ
}

// hasErrorType checks if any result is an error type.
func hasErrorType(results []struct{ Name, Type string }) bool {
	for _, r := range results {
		if r.Type == "error" || r.Type == "*error" {
			return true
		}
	}
	return false
}

// hasBoolType checks if any result is a bool type.
func hasBoolType(results []struct{ Name, Type string }) bool {
	for _, r := range results {
		if r.Type == "bool" {
			return true
		}
	}
	return false
}

// hasInterfaceType checks if any result is an interface type.
func hasInterfaceType(results []struct{ Name, Type string }) bool {
	for _, r := range results {
		if r.Type == "interface{}" || r.Type == "any" {
			return true
		}
	}
	return false
}

// generateGoTestCases generates test case values for table-driven tests.
func (g *testGenerator) generateGoTestCases(sb *strings.Builder, params []struct {
	Name, Type string
}, hasErrorReturn, hasBoolReturn, hasInterfaceReturn bool) {
	// Generate test cases based on parameter types

	// Case 1: Normal/expected input
	sb.WriteString("\t\t{\n\t\t\tname: \"normal_case\", // Default values for all params\n")
	g.writeGoCaseValues(sb, params, "default")
	sb.WriteString("\t\t},\n")

	// Case 2: Edge cases
	sb.WriteString("\t\t{\n\t\t\tname: \"edge_case_empty\", // Empty/zero values\n")
	g.writeGoCaseValues(sb, params, "empty")
	sb.WriteString("\t\t},\n")

	// Case 3: Negative/invalid cases
	sb.WriteString("\t\t{\n\t\t\tname: \"invalid_input\", // Negative or invalid values\n")
	g.writeGoCaseValues(sb, params, "negative")
	sb.WriteString("\t\t},\n")

	// Case 4: Boundary cases (if there are numeric params)
	hasNumeric := false
	for _, p := range params {
		if isNumericType(p.Type) {
			hasNumeric = true
			break
		}
	}
	if hasNumeric {
		sb.WriteString("\t\t{\n\t\t\tname: \"boundary_values\", // Boundary values\n")
		g.writeGoCaseValues(sb, params, "boundary")
		sb.WriteString("\t\t},\n")
	}

	// Case 5: Slice/array edge cases (if there are slice params)
	hasSlice := false
	for _, p := range params {
		if strings.HasPrefix(p.Type, "[]") {
			hasSlice = true
			break
		}
	}
	if hasSlice {
		sb.WriteString("\t\t{\n\t\t\tname: \"nil_slice\", // nil slice\n")
		g.writeGoCaseValues(sb, params, "nil")
		sb.WriteString("\t\t},\n")
	}
}

// writeGoCaseValues writes the field values for a test case.
func (g *testGenerator) writeGoCaseValues(sb *strings.Builder, params []struct {
	Name, Type string
}, caseType string) {
	for i, p := range params {
		fieldName := sanitizeIdentifier(p.Name)
		if len(params) == 1 {
			fieldName = "input"
		} else {
			fieldName = strings.Title(fieldName)
		}

		value := g.getDefaultValue(p.Type, caseType)
		if i == len(params)-1 {
			sb.WriteString(fmt.Sprintf("\t\t\t%s: %s,\n", fieldName, value))
		} else {
			sb.WriteString(fmt.Sprintf("\t\t\t%s: %s, // ", fieldName, value))
		}
	}
}

// getDefaultValue returns a default value for a Go type.
func (g *testGenerator) getDefaultValue(typ, caseType string) string {
	switch {
	case typ == "string" || typ == "*string":
		switch caseType {
		case "empty":
			return `""`
		case "negative":
			return `"<invalid>"`
		case "boundary":
			return `"<boundary>"`
		case "nil":
			return `nil`
		default:
			return `"<valid_value>"`
		}
	case isNumericType(typ):
		switch caseType {
		case "empty":
			return "0"
		case "negative":
			return "-1"
		case "boundary":
			return "1000000"
		default:
			return "42"
		}
	case typ == "bool" || typ == "*bool":
		switch caseType {
		case "empty":
			return "false"
		case "negative":
			return "false"
		default:
			return "true"
		}
	case strings.HasPrefix(typ, "[]"):
		switch caseType {
		case "empty":
			return "[]int{}"
		case "nil":
			return "nil"
		case "negative":
			return "[]int{-1}"
		default:
			return "[]int{1, 2, 3}"
		}
	case strings.HasPrefix(typ, "map["):
		switch caseType {
		case "empty":
			return "map[string]string{}"
		case "nil":
			return "nil"
		default:
			return "map[string]string{\"key\": \"value\"}"
		}
	case typ == "error" || typ == "*error":
		switch caseType {
		case "negative":
			return "fmt.Errorf(\"expected error\")"
		default:
			return "nil"
		}
	default:
		return "nil"
	}
}

// isNumericType checks if a type is numeric.
func isNumericType(typ string) bool {
	numericTypes := []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "float"}
	for _, t := range numericTypes {
		if typ == t || typ == "*"+t {
			return true
		}
	}
	return false
}

// generateGoCallExpression generates the function call expression.
func (g *testGenerator) generateGoCallExpression(fn goFunction) string {
	parts := []string{}
	for i, p := range fn.ParamsList() {
		fieldName := sanitizeIdentifier(p.Name)
		if len(fn.ParamsList()) == 1 {
			fieldName = "tt.input"
		} else {
			fieldName = fmt.Sprintf("tt.%s", strings.Title(fieldName))
		}
		parts = append(parts, fieldName)
		_ = i
	}

	called := fn.Name
	if fn.Receiver != "" {
		called = fmt.Sprintf("tt.%s.%s", strings.Title(fn.Receiver), fn.Name)
	}

	return fmt.Sprintf("%s(%s)", called, strings.Join(parts, ", "))
}

// generateGoAssertions generates assertion code based on return types.
func (g *testGenerator) generateGoAssertions(sb *strings.Builder, fn goFunction, hasErrorReturn, hasBoolReturn bool) {
	results := parseGoParams(fn.Results)

	if hasErrorReturn {
		// Check that error is nil for normal case
		sb.WriteString("\t\t\tif tt.name == \"normal_case\" && err != nil {\n")
		sb.WriteString(fmt.Sprintf("\t\t\t\tt.Errorf(\"%%s() error = %%v, want nil\", tt.name, err)\n"))
		sb.WriteString("\t\t\t}\n\n")

		// Check error cases
		sb.WriteString("\t\t\tif tt.name == \"invalid_input\" && err == nil {\n")
		sb.WriteString(fmt.Sprintf("\t\t\t\tt.Errorf(\"%%s() expected error for invalid input, got nil\", tt.name)\n"))
		sb.WriteString("\t\t\t}\n")
	} else if hasBoolReturn {
		sb.WriteString("\t\t\t// Verify boolean result\n")
		sb.WriteString("\t\t\tif tt.name == \"normal_case\" {\n")
		sb.WriteString(fmt.Sprintf("\t\t\t\t// %s() returns expected value\n", fn.Name))
		sb.WriteString("\t\t\t}\n")
	} else if len(results) > 0 {
		sb.WriteString("\t\t\t// Verify return value\n")
		sb.WriteString(fmt.Sprintf("\t\t\tif tt.name == \"normal_case\" {\n"))
		sb.WriteString(fmt.Sprintf("\t\t\t\t// %s() returned: %%v\n", fn.Name))
		sb.WriteString(fmt.Sprintf("\t\t\t\t// t.Logf(\"result: %%v\", result)\n"))
		sb.WriteString("\t\t\t}\n")
	}
}

// ParamsList returns the parsed parameters.
func (fn *goFunction) ParamsList() []struct{ Name, Type string } {
	return parseGoParams(fn.Params)
}

// ========================
// Python Test Generation
// ========================

// pythonFunction represents a Python function/class extracted from source.
type pythonFunction struct {
	Name       string
	Params     string
	HasReturn  bool
	ReturnType string
	IsMethod   bool
	Class      string
	IsAsync    bool
	LineNum    int
	Docstring  string
}

// generatePythonTests generates Python unit tests using pytest.
func (g *testGenerator) generatePythonTests(source string, sourcePath string, framework string) (string, error) {
	// Extract module name
	moduleName := filepath.Base(sourcePath)
	if strings.HasSuffix(moduleName, ".py") {
		moduleName = strings.TrimSuffix(moduleName, ".py")
	}

	// Extract functions and classes
	funcs, classes := g.extractPythonFunctions(source)

	// Build test file
	var sb strings.Builder

	sb.WriteString("# Auto-generated test file for " + moduleName + "\n")
	sb.WriteString("# Generated by testgen tool\n\n")

	// Imports
	sb.WriteString("import pytest\n")
	if framework == "pytest" {
		sb.WriteString("from unittest.mock import patch, MagicMock\n\n")
	}
	sb.WriteString("# Import the module to test\n")
	sb.WriteString("from " + moduleName + " import (\n")

	// Collect names to import
	neededNames := make([]string, 0)
	for _, fn := range funcs {
		if !fn.IsMethod {
			neededNames = append(neededNames, fn.Name)
		}
	}
	for _, c := range classes {
		neededNames = append(neededNames, c.Name)
	}

	if len(neededNames) > 0 {
		// Sort for deterministic output
		sort.Strings(neededNames)
		// Remove duplicates
		seen := make(map[string]bool)
		for _, name := range neededNames {
			if !seen[name] {
				seen[name] = true
				sb.WriteString("\t" + name + ",\n")
			}
		}
	}
	sb.WriteString(")\n\n")

	// Generate tests for functions
	for _, fn := range funcs {
		if !fn.IsMethod {
			g.generatePythonTestFunction(&sb, fn, moduleName)
			sb.WriteString("\n")
		}
	}

	// Generate tests for classes and methods
	for _, c := range classes {
		g.generatePythonTestClass(&sb, c, funcs)
		sb.WriteString("\n")
	}

	// Generate pytest main guard
	sb.WriteString("if __name__ == \"__main__\":\n")
	sb.WriteString("\tpytest.main([__file__])\n")

	return sb.String(), nil
}

// extractPythonFunctions extracts functions and classes from Python source.
func (g *testGenerator) extractPythonFunctions(source string) ([]pythonFunction, []struct {
	Name    string
	Methods []pythonFunction
}) {
	var funcs []pythonFunction
	var classes []struct {
		Name    string
		Methods []pythonFunction
	}

	lines := strings.Split(source, "\n")
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			i++
			continue
		}

		// Detect class definitions
		classRe := regexp.MustCompile(`^class\s+(\w+)`)
		if matches := classRe.FindStringSubmatch(trimmed); matches != nil {
			className := matches[1]
			var methods []pythonFunction

			// Find methods in the class (indented block)
			i++
			for i < len(lines) {
				innerLine := strings.TrimSpace(lines[i])

				// End of class (de-indentation)
				if innerLine != "" && !strings.HasPrefix(lines[i], "\t") && !strings.HasPrefix(lines[i], "    ") && !strings.HasPrefix(trimmed, "class") {
				break
			}

				// Detect method definitions
				methodRe := regexp.MustCompile(`^(async\s+)?def\s+(\w+)\s*\(([^)]*)\)\s*:\s*(#.*)?$`)
				if matches := methodRe.FindStringSubmatch(innerLine); matches != nil {
					isAsync := matches[1] != ""
					methodName := matches[2]
					params := strings.TrimSpace(matches[3])

					// Check if it starts with underscore (private)
					if strings.HasPrefix(methodName, "_") && methodName != "__init__" {
						i++
						continue
					}

					// Get docstring
					docstring := ""
					for j := i + 1; j < len(lines) && j <= i+3; j++ {
						nextLine := strings.TrimSpace(lines[j])
						if strings.HasPrefix(nextLine, `"""`) || strings.HasPrefix(nextLine, `'''`) {
							docstring = nextLine
							break
						}
					}

					methods = append(methods, pythonFunction{
						Name:      methodName,
						Params:    params,
						IsMethod:  true,
						Class:     className,
						IsAsync:   isAsync,
						LineNum:   i + 1,
						Docstring: docstring,
					})
				}
				i++
			}

			if len(methods) > 0 {
				classes = append(classes, struct {
					Name    string
					Methods []pythonFunction
				}{Name: className, Methods: methods})
			}
			continue
		}

		// Detect function definitions
		funcRe := regexp.MustCompile(`^(async\s+)?def\s+(\w+)\s*\(([^)]*)\)\s*:\s*(#.*)?$`)
		if matches := funcRe.FindStringSubmatch(trimmed); matches != nil {
			isAsync := matches[1] != ""
			funcName := matches[2]
			params := strings.TrimSpace(matches[3])

			// Skip private functions
			if strings.HasPrefix(funcName, "_") && funcName != "__init__" {
				i++
				continue
			}

			// Check return type hint
			returnType := ""
			hasReturn := false
			for j := i + 1; j < len(lines) && j <= i+5; j++ {
				nextLine := strings.TrimSpace(lines[j])
				if strings.Contains(nextLine, "->") {
					parts := strings.Split(nextLine, "->")
					if len(parts) > 1 {
						returnType = strings.TrimSpace(parts[len(parts)-1])
						returnType = strings.Split(returnType, ":")[0] // Remove inline comment
						hasReturn = true
					}
				}
				if strings.HasPrefix(nextLine, "return") {
					hasReturn = true
				}
			}

			// Get docstring
			docstring := ""
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				nextLine := strings.TrimSpace(lines[j])
				if strings.HasPrefix(nextLine, `"""`) || strings.HasPrefix(nextLine, `'''`) {
					docstring = nextLine
					break
				}
			}

			funcs = append(funcs, pythonFunction{
				Name:       funcName,
				Params:     params,
				HasReturn:  hasReturn,
				ReturnType: returnType,
				IsMethod:   false,
				IsAsync:    isAsync,
				LineNum:    i + 1,
				Docstring:  docstring,
			})
		}

		i++
	}

	return funcs, classes
}

// generatePythonTestFunction generates a pytest test function for a single Python function.
func (g *testGenerator) generatePythonTestFunction(sb *strings.Builder, fn pythonFunction, moduleName string) {
	// Parse parameters
	params := parsePythonParams(fn.Params)

	// Remove 'self' parameter for standalone functions
	var testParams []struct{ Name, Type string }
	for _, p := range params {
		if p.Name != "self" && p.Name != "cls" {
			testParams = append(testParams, p)
		}
	}

	// Generate test function name
	testName := "test_" + fn.Name

	if fn.IsAsync {
		sb.WriteString(fmt.Sprintf("import pytest\n\n"))
		sb.WriteString(fmt.Sprintf("@pytest.mark.asyncio\n"))
	}

	sb.WriteString(fmt.Sprintf("def %s():\n", testName))

	if fn.Docstring != "" {
		sb.WriteString(fmt.Sprintf("\t\"\"\"Test for %s.\"\"\"\n", fn.Name))
	}

	// Generate test cases with @pytest.mark.parametrize
	if len(testParams) > 0 {
		// Create parameter fixtures
		paramNames := make([]string, len(testParams))
		paramValues := make([][]string, len(testParams))

		for i, p := range testParams {
			paramNames[i] = p.Name
			paramValues[i] = g.getDefaultPythonValues(p.Type)
		}

		// Generate parametrize decorator
		var paramStr string
		for i, name := range paramNames {
			if i > 0 {
				paramStr += ", "
			}
			paramStr += name
		}

		// Generate test cases
		numCases := len(paramValues[0])
		if numCases == 0 {
			numCases = 3
		}

		// Build parametrize values
		var valuesStr string
		for c := 0; c < numCases; c++ {
			if c > 0 {
				valuesStr += "\n\t\t"
			}
			valuesStr += "("
			for i := range paramNames {
				if i > 0 {
					valuesStr += ", "
				}
				if c < len(paramValues[i]) {
					valuesStr += paramValues[i][c]
				} else {
					valuesStr += paramValues[i][0]
				}
			}
			valuesStr += ")"
		}

		sb.WriteString(fmt.Sprintf("@pytest.mark.parametrize(\"%s\", [\n%s])\n", paramStr, valuesStr))
	}

	// Test body
	sb.WriteString("\t# Arrange\n")

	// Build arguments
	var args []string
	for _, p := range testParams {
		args = append(args, p.Name)
	}

	if fn.IsAsync {
		sb.WriteString("\t# Act\n")
		sb.WriteString(fmt.Sprintf("\tresult = await %s(%s)\n", fn.Name, strings.Join(args, ", ")))
	} else {
		sb.WriteString("\t# Act\n")
		sb.WriteString(fmt.Sprintf("\tresult = %s(%s)\n", fn.Name, strings.Join(args, ", ")))
	}

	// Assert
	sb.WriteString("\t# Assert\n")
	if fn.HasReturn {
		sb.WriteString(fmt.Sprintf("\tassert result is not None  # %s should return a value\n", fn.Name))
	} else {
		sb.WriteString(fmt.Sprintf("\t# %s may or may not return a value\n", fn.Name))
	}

	// Error handling test
	sb.WriteString(fmt.Sprintf("\t# Test with invalid input\n"))
	sb.WriteString("\twith pytest.raises(Exception):\n")
	var invalidArgs []string
	for _, p := range testParams {
		invalidArgs = append(invalidArgs, g.getInvalidValue(p.Type))
	}
	sb.WriteString(fmt.Sprintf("\t\t%s(%s)\n", fn.Name, strings.Join(invalidArgs, ", ")))

	sb.WriteString("\n")
}

// generatePythonTestClass generates pytest tests for a class.
func (g *testGenerator) generatePythonTestClass(sb *strings.Builder, class struct {
	Name    string
	Methods []pythonFunction
}, standaloneFuncs []pythonFunction) {
	sb.WriteString(fmt.Sprintf("class Test%s:\n", class.Name))

	for _, method := range class.Methods {
		// Generate test for this method
		params := parsePythonParams(method.Params)
		var testParams []struct{ Name, Type string }
		for _, p := range params {
			if p.Name != "self" && p.Name != "cls" {
				testParams = append(testParams, p)
			}
		}

		testName := "test_" + method.Name
		sb.WriteString(fmt.Sprintf("\tdef %s(self):\n", testName))

		// Arrange: create instance
		sb.WriteString(fmt.Sprintf("\t\tinstance = %s()\n", class.Name))

		// Act
		var args []string
		for _, p := range testParams {
			args = append(args, p.Name)
		}

		// Use mock values for parameters
		var mockArgs []string
		for _, p := range testParams {
			mockArgs = append(mockArgs, g.getDefaultPythonValues(p.Type)[0])
		}

		sb.WriteString(fmt.Sprintf("\t\t# Act\n"))
		sb.WriteString(fmt.Sprintf("\t\tresult = instance.%s(%s)\n", method.Name, strings.Join(mockArgs, ", ")))

		// Assert
		sb.WriteString(fmt.Sprintf("\t\t# Assert\n"))
		if method.HasReturn {
			sb.WriteString(fmt.Sprintf("\t\tassert result is not None\n"))
		} else {
			sb.WriteString(fmt.Sprintf("\t\t# Method may not return a value\n"))
		}
		sb.WriteString("\n")
	}
}

// parsePythonParams parses Python function parameters.
func parsePythonParams(paramStr string) []struct{ Name, Type string } {
	var params []struct{ Name, Type string }

	paramStr = strings.TrimSpace(paramStr)
	if paramStr == "" || paramStr == "self" {
		return params
	}

	// Split by comma, handling defaults
	parts := splitPythonParams(paramStr)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "self" || part == "cls" {
			continue
		}

		// Handle type hints: "name: Type = default"
		name, typ := parsePythonParam(part)
		params = append(params, struct{ Name, Type string }{Name: name, Type: typ})
	}

	return params
}

// splitPythonParams splits parameters by comma, handling defaults with commas.
func splitPythonParams(paramStr string) []string {
	var parts []string
	current := ""
	depth := 0
	for _, ch := range paramStr {
		if ch == '(' || ch == '[' || ch == '{' {
			depth++
		} else if ch == ')' || ch == ']' || ch == '}' {
			depth--
		} else if ch == ',' && depth == 0 {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if strings.TrimSpace(current) != "" {
		parts = append(parts, current)
	}
	return parts
}

// parsePythonParam extracts name and type from a parameter string.
func parsePythonParam(param string) (string, string) {
	param = strings.TrimSpace(param)

	// Handle type hints: "name: Type = default" or "name: Type"
	parts := strings.Split(param, ":")
	name := parts[0]
	typ := ""

	if len(parts) > 1 {
		rest := strings.Join(parts[1:], ":")
		// Handle default values
		if idx := strings.Index(rest, "="); idx > 0 {
			typ = strings.TrimSpace(rest[:idx])
		} else {
			typ = strings.TrimSpace(rest)
		}
	}

	// Clean up name
	name = strings.TrimSpace(name)
	if name == "" {
		name = "arg"
	}

	return name, typ
}

// getDefaultValueForPythonType returns default values for Python types.
func (g *testGenerator) getDefaultValueForPythonType(typ, caseType string) string {
	switch {
	case typ == "str" || typ == "string" || typ == "":
		switch caseType {
		case "empty":
			return `""`
		case "invalid":
			return `"<invalid>"`
		default:
			return `"value"`
		}
	case typ == "int" || typ == "float" || isNumericTypePython(typ):
		switch caseType {
		case "empty":
			return "0"
		case "invalid":
			return "-1"
		default:
			return "42"
		}
	case typ == "bool":
		return "True"
	case typ == "list" || typ == "List" || strings.HasPrefix(typ, "list[") || strings.HasPrefix(typ, "List["):
		switch caseType {
		case "empty":
			return "[]"
		case "invalid":
			return "[-1]"
		default:
			return "[1, 2, 3]"
		}
	case typ == "dict" || typ == "Dict" || strings.HasPrefix(typ, "dict[") || strings.HasPrefix(typ, "Dict["):
		switch caseType {
		case "empty":
			return "{}"
		case "invalid":
			return `{"key": "invalid"}`
		default:
			return `{"key": "value"}`
		}
	case strings.HasSuffix(typ, "Optional[str]"):
		return "None"
	case strings.HasSuffix(typ, "Optional[int]"):
		return "None"
	default:
		return "None"
	}
}

// isNumericTypePython checks if a Python type is numeric.
func isNumericTypePython(typ string) bool {
	numericTypes := []string{"int", "float", "Integer", "Float"}
	for _, t := range numericTypes {
		if typ == t {
			return true
		}
	}
	return false
}

// getDefaultPythonValues returns test values for a Python parameter type.
func (g *testGenerator) getDefaultPythonValues(typ string) []string {
	return []string{
		g.getDefaultValueForPythonType(typ, "normal"),
		g.getDefaultValueForPythonType(typ, "empty"),
		g.getDefaultValueForPythonType(typ, "invalid"),
	}
}

// getInvalidValue returns an invalid value for a Python parameter type.
func (g *testGenerator) getInvalidValue(typ string) string {
	switch {
	case typ == "int" || typ == "float" || isNumericTypePython(typ):
		return "None"
	case typ == "str" || typ == "string":
		return "None"
	case typ == "bool":
		return "None"
	default:
		return "None"
	}
}

// ========================
// JavaScript/TypeScript Test Generation
// ========================

// jsFunction represents a JS/TS function extracted from source.
type jsFunction struct {
	Name       string
	Params     string
	HasReturn  bool
	IsExported bool
	IsAsync    bool
	Type       string // "function", "arrow", "method"
	Class      string
	LineNum    int
	Comment    string
}

// generateJSTests generates Jest/JS test suites.
func (g *testGenerator) generateJSTests(source string, sourcePath string, language string, framework string) (string, error) {
	// Extract module name
	moduleName := filepath.Base(sourcePath)
	if strings.HasSuffix(moduleName, ".js") {
		moduleName = strings.TrimSuffix(moduleName, ".js")
	} else if strings.HasSuffix(moduleName, ".ts") {
		moduleName = strings.TrimSuffix(moduleName, ".ts")
	}

	// Extract functions and classes
	funcs, classes := g.extractJSFunctions(source)

	// Build test file
	var sb strings.Builder

	sb.WriteString(`// Auto-generated test file for ` + moduleName + "\n")
	sb.WriteString(`// Generated by testgen tool` + "\n\n")

	// Import statement
	sb.WriteString(fmt.Sprintf("import { %s } from './%s';\n", g.getJSExports(funcs, classes), moduleName))

	if language == "typescript" {
		sb.WriteString(fmt.Sprintf("import type { %s } from './%s';\n", g.getJSTypes(funcs, classes), moduleName))
	}

	sb.WriteString("\n")

	// Generate test suites
	for _, fn := range funcs {
		if fn.IsExported {
			g.generateJSTestFunction(&sb, fn, language)
		}
	}

	for _, c := range classes {
		g.generateJSTestClass(&sb, c, language)
	}

	return sb.String(), nil
}

// extractJSFunctions extracts functions and classes from JS/TS source.
func (g *testGenerator) extractJSFunctions(source string) ([]jsFunction, []struct {
	Name    string
	Methods []jsFunction
}) {
	var funcs []jsFunction
	var classes []struct {
		Name    string
		Methods []jsFunction
	}

	lines := strings.Split(source, "\n")
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Detect class definitions
		classRe := regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?class\s+(\w+)`)
		if matches := classRe.FindStringSubmatch(line); matches != nil {
			className := matches[1]
			var methods []jsFunction

			i++
			for i < len(lines) {
				innerLine := strings.TrimSpace(lines[i])

				// Detect method definitions
				methodRe := regexp.MustCompile(`^(async\s+)?(\w+)\s*\(([^)]*)\)\s*[:{]`)
				if matches := methodRe.FindStringSubmatch(innerLine); matches != nil {
					isAsync := matches[1] != ""
					methodName := matches[2]
					params := strings.TrimSpace(matches[3])

					if strings.HasPrefix(methodName, "constructor") || methodName == "constructor" {
						i++
						continue
					}

					methods = append(methods, jsFunction{
						Name:       methodName,
						Params:     params,
						IsExported: true,
						IsAsync:    isAsync,
						Type:       "method",
						Class:      className,
						LineNum:    i + 1,
					})
				}

				// End of class (de-indentation or end of file)
				if innerLine != "" && !strings.HasPrefix(lines[i], "\t") && !strings.HasPrefix(lines[i], "    ") && innerLine != "}" && !strings.HasPrefix(innerLine, "}") {
					break
				}

				i++
			}

			if len(methods) > 0 {
				classes = append(classes, struct {
					Name    string
					Methods []jsFunction
				}{Name: className, Methods: methods})
			}
			continue
		}

		// Detect exported function declarations
		funcRe := regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+(\w+)\s*\(([^)]*)\)`)
		if matches := funcRe.FindStringSubmatch(line); matches != nil {
			funcName := matches[1]
			params := strings.TrimSpace(matches[2])

			// Check for async
			isAsync := strings.Contains(line, "async")

			// Check for return
			hasReturn := false
			for j := i + 1; j < len(lines) && j <= i+10; j++ {
				if strings.Contains(lines[j], "return") {
					hasReturn = true
					break
				}
			}

			funcs = append(funcs, jsFunction{
				Name:       funcName,
				Params:     params,
				HasReturn:  hasReturn,
				IsExported: strings.Contains(line, "export"),
				IsAsync:    isAsync,
				Type:       "function",
				LineNum:    i + 1,
			})
		}

		// Detect exported arrow functions
		arrowRe := regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(async\s*)?\(`)
		if matches := arrowRe.FindStringSubmatch(line); matches != nil {
			funcName := matches[1]
			isAsync := matches[2] != ""

			// Get params from the next part or same line
			params := ""
			rest := strings.TrimPrefix(line, matches[0])
			if idx := strings.Index(rest, "=>"); idx > 0 {
				params = strings.TrimSpace(rest[:idx])
			}

			// Check for return
			hasReturn := false
			for j := i + 1; j < len(lines) && j <= i+10; j++ {
				if strings.Contains(lines[j], "return") {
					hasReturn = true
					break
				}
			}

			funcs = append(funcs, jsFunction{
				Name:       funcName,
				Params:     params,
				HasReturn:  hasReturn,
				IsExported: strings.Contains(line, "export"),
				IsAsync:    isAsync,
				Type:       "arrow",
				LineNum:    i + 1,
			})
		}

		i++
	}

	return funcs, classes
}

// getJSExports returns export names for the import statement.
func (g *testGenerator) getJSExports(funcs []jsFunction, classes []struct {
	Name    string
	Methods []jsFunction
}) string {
	var names []string
	for _, fn := range funcs {
		if fn.IsExported {
			names = append(names, fn.Name)
		}
	}
	for _, c := range classes {
		names = append(names, c.Name)
	}
	if len(names) == 0 {
		return "*"
	}
	return strings.Join(names, ", ")
}

// getJSTypes returns type names for TypeScript.
func (g *testGenerator) getJSTypes(funcs []jsFunction, classes []struct {
	Name    string
	Methods []jsFunction
}) string {
	// For simplicity, include class names as types
	var names []string
	for _, c := range classes {
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}

// generateJSTestFunction generates a Jest test function.
func (g *testGenerator) generateJSTestFunction(sb *strings.Builder, fn jsFunction, language string) {
	sb.WriteString(fmt.Sprintf("describe('%s', () => {\n", fn.Name))

	// Parse parameters
	params := parseJSParams(fn.Params)

	// Generate test cases
	cases := []struct {
		Name  string
		Args  string
		Setup string
	}{
		{
			Name:  "should handle normal input",
			Args:  g.getDefaultJSArgs(params, "normal"),
			Setup: "// Normal case: valid input",
		},
		{
			Name:  "should handle empty/zero input",
			Args:  g.getDefaultJSArgs(params, "empty"),
			Setup: "// Edge case: empty or zero values",
		},
		{
			Name:  "should handle invalid input",
			Args:  g.getDefaultJSArgs(params, "invalid"),
			Setup: "// Error case: invalid input",
		},
	}

	for _, tc := range cases {
		sb.WriteString(fmt.Sprintf("\t// %s\n", tc.Setup))
		sb.WriteString(fmt.Sprintf("\ttest('%s', () => {\n", tc.Name))

		if fn.HasReturn {
			sb.WriteString(fmt.Sprintf("\t\tconst result = %s(%s);\n", fn.Name, tc.Args))
			sb.WriteString(fmt.Sprintf("\t\texpect(result).toBeDefined();\n"))
		} else {
			sb.WriteString(fmt.Sprintf("\t\t%s(%s);\n", fn.Name, tc.Args))
			sb.WriteString(fmt.Sprintf("\t\t// No return value expected\n"))
		}

		// Error handling
		sb.WriteString(fmt.Sprintf("\t\t// Verify no unhandled exceptions\n"))
		sb.WriteString(fmt.Sprintf("\t\texpect(() => %s(%s)).not.toThrow();\n", fn.Name, tc.Args))

		sb.WriteString("\t});\n\n")
	}

	// Error handling test
	sb.WriteString(fmt.Sprintf("\ttest('should throw on invalid input', () => {\n"))
	sb.WriteString(fmt.Sprintf("\t\texpect(() => %s(%s)).toThrow();\n", fn.Name, g.getDefaultJSArgs(params, "invalid")))
	sb.WriteString("\t});\n")

	sb.WriteString("});\n\n")
}

// generateJSTestClass generates a Jest test class.
func (g *testGenerator) generateJSTestClass(sb *strings.Builder, class struct {
	Name    string
	Methods []jsFunction
}, language string) {
	sb.WriteString(fmt.Sprintf("describe('%s', () => {\n", class.Name))
	sb.WriteString(fmt.Sprintf("\tlet instance;\n\n"))

	sb.WriteString(fmt.Sprintf("\tbeforeEach(() => {\n"))
	sb.WriteString(fmt.Sprintf("\t\tinstance = new %s();\n", class.Name))
	sb.WriteString("\t});\n\n")

	for _, method := range class.Methods {
		params := parseJSParams(method.Params)

		sb.WriteString(fmt.Sprintf("\tdescribe('method %s', () => {\n", method.Name))
		sb.WriteString(fmt.Sprintf("\t\ttest('should execute successfully', () => {\n"))

		args := g.getDefaultJSArgs(params, "normal")
		if method.HasReturn {
			sb.WriteString(fmt.Sprintf("\t\t\tconst result = instance.%s(%s);\n", method.Name, args))
			sb.WriteString(fmt.Sprintf("\t\t\texpect(result).toBeDefined();\n"))
		} else {
			sb.WriteString(fmt.Sprintf("\t\t\texpect(() => instance.%s(%s)).not.toThrow();\n", method.Name, args))
		}

		sb.WriteString("\t\t});\n")
		sb.WriteString("\t});\n\n")
	}

	sb.WriteString("});\n\n")
}

// parseJSParams parses JavaScript function parameters.
func parseJSParams(paramStr string) []struct{ Name, Type string } {
	var params []struct{ Name, Type string }

	paramStr = strings.TrimSpace(paramStr)
	if paramStr == "" {
		return params
	}

	// Split by comma
	parts := strings.Split(paramStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "this" {
			continue
		}

		// Handle destructuring: { prop1, prop2 }
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			inner := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			for _, p := range strings.Split(inner, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					params = append(params, struct{ Name, Type string }{Name: p})
				}
			}
			continue
		}

		// Handle default values: "name = default"
		name := part
		if idx := strings.Index(part, "="); idx > 0 {
			name = strings.TrimSpace(part[:idx])
		}
		// Handle type annotations: "name: Type"
		if idx := strings.Index(name, ":"); idx > 0 {
			name = strings.TrimSpace(name[:idx])
		}

		if name != "" {
			params = append(params, struct{ Name, Type string }{Name: name})
		}
	}

	return params
}

// getDefaultJSArgs returns default argument values for JS parameters.
func (g *testGenerator) getDefaultJSArgs(params []struct{ Name, Type string }, caseType string) string {
	var args []string
	for _, p := range params {
		args = append(args, g.getDefaultJSValue(p.Name, caseType))
	}
	return strings.Join(args, ", ")
}

// getDefaultJSValue returns a default value for a JS parameter.
func (g *testGenerator) getDefaultJSValue(name string, caseType string) string {
	switch caseType {
	case "normal":
		return `"default"`
	case "empty":
		return `""`
	case "invalid":
		return `null`
	default:
		return `"default"`
	}
}

// ========================
// Utility Functions
// ========================

// countGeneratedFunctions counts the number of functions that would be generated.
func countGeneratedFunctions(source string, language string) int {
	gen := &testGenerator{}
	switch language {
	case "go":
		return len(gen.extractGoFunctions(source))
	case "python":
		funcs, _ := gen.extractPythonFunctions(source)
		return len(funcs)
	case "javascript", "typescript":
		funcs, _ := gen.extractJSFunctions(source)
		return len(funcs)
	default:
		return 0
	}
}

// sanitizeIdentifier sanitizes a string to be a valid Go identifier.
func sanitizeIdentifier(name string) string {
	name = strings.TrimSpace(name)
	// Replace invalid characters
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = re.ReplaceAllString(name, "_")
	// Ensure it doesn't start with a digit
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	return name
}
