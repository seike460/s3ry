package ai

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// TestGenerator generates AI-powered automated tests
type TestGenerator struct {
	config   *GeneratorConfig
	analyzer *CodeAnalyzer
	patterns *TestPatterns
}

// GeneratorConfig holds configuration for test generation
type GeneratorConfig struct {
	OutputDir          string   `json:"output_dir"`
	TestSuffix         string   `json:"test_suffix"`
	PackagePattern     string   `json:"package_pattern"`
	ExcludePatterns    []string `json:"exclude_patterns"`
	CoverageTarget     float64  `json:"coverage_target"`
	MaxTestsPerFunc    int      `json:"max_tests_per_func"`
	GenerateEdgeCases  bool     `json:"generate_edge_cases"`
	GenerateBenchmarks bool     `json:"generate_benchmarks"`
}

// DefaultGeneratorConfig returns default configuration
func DefaultGeneratorConfig() *GeneratorConfig {
	return &GeneratorConfig{
		OutputDir:          ".",
		TestSuffix:         "_ai_test.go",
		PackagePattern:     "**/*.go",
		ExcludePatterns:    []string{"*_test.go", "vendor/**", ".git/**"},
		CoverageTarget:     80.0,
		MaxTestsPerFunc:    5,
		GenerateEdgeCases:  true,
		GenerateBenchmarks: true,
	}
}

// CodeAnalyzer analyzes code structure for test generation
type CodeAnalyzer struct {
	fileSet *token.FileSet
}

// NewCodeAnalyzer creates a new code analyzer
func NewCodeAnalyzer() *CodeAnalyzer {
	return &CodeAnalyzer{
		fileSet: token.NewFileSet(),
	}
}

// FunctionInfo holds information about a function
type FunctionInfo struct {
	Name       string
	Package    string
	Params     []ParamInfo
	Returns    []ReturnInfo
	IsExported bool
	Comments   []string
	Complexity int
	LineStart  int
	LineEnd    int
}

// ParamInfo holds parameter information
type ParamInfo struct {
	Name string
	Type string
}

// ReturnInfo holds return value information
type ReturnInfo struct {
	Name string
	Type string
}

// TestPatterns contains patterns for generating different types of tests
type TestPatterns struct {
	unitTestTemplate      *template.Template
	benchmarkTestTemplate *template.Template
	edgeCaseTemplate      *template.Template
}

// NewTestGenerator creates a new AI test generator
func NewTestGenerator(config *GeneratorConfig) (*TestGenerator, error) {
	if config == nil {
		config = DefaultGeneratorConfig()
	}

	analyzer := NewCodeAnalyzer()
	patterns, err := NewTestPatterns()
	if err != nil {
		return nil, fmt.Errorf("failed to create test patterns: %w", err)
	}

	return &TestGenerator{
		config:   config,
		analyzer: analyzer,
		patterns: patterns,
	}, nil
}

// NewTestPatterns creates test patterns with templates
func NewTestPatterns() (*TestPatterns, error) {
	unitTestTmpl := `func Test{{.FuncName}}(t *testing.T) {
	{{range .TestCases}}
	t.Run("{{.Name}}", func(t *testing.T) {
		{{.Setup}}
		
		{{.Call}}
		
		{{.Assertions}}
	})
	{{end}}
}
`

	benchmarkTmpl := `func Benchmark{{.FuncName}}(b *testing.B) {
	{{.Setup}}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		{{.Call}}
	}
}
`

	edgeCaseTmpl := `func Test{{.FuncName}}_EdgeCases(t *testing.T) {
	{{range .EdgeCases}}
	t.Run("{{.Name}}", func(t *testing.T) {
		{{.Setup}}
		
		{{.Call}}
		
		{{.Assertions}}
	})
	{{end}}
}
`

	unitTest, err := template.New("unitTest").Parse(unitTestTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unit test template: %w", err)
	}

	benchmarkTest, err := template.New("benchmarkTest").Parse(benchmarkTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse benchmark template: %w", err)
	}

	edgeCase, err := template.New("edgeCase").Parse(edgeCaseTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse edge case template: %w", err)
	}

	return &TestPatterns{
		unitTestTemplate:      unitTest,
		benchmarkTestTemplate: benchmarkTest,
		edgeCaseTemplate:      edgeCase,
	}, nil
}

// AnalyzeFile analyzes a Go file and extracts function information
func (a *CodeAnalyzer) AnalyzeFile(filename string) ([]*FunctionInfo, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	file, err := parser.ParseFile(a.fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	var functions []*FunctionInfo

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Name != nil {
				funcInfo := a.extractFunctionInfo(node, file.Name.Name)
				functions = append(functions, funcInfo)
			}
		}
		return true
	})

	return functions, nil
}

// extractFunctionInfo extracts detailed information from a function declaration
func (a *CodeAnalyzer) extractFunctionInfo(funcDecl *ast.FuncDecl, packageName string) *FunctionInfo {
	info := &FunctionInfo{
		Name:       funcDecl.Name.Name,
		Package:    packageName,
		IsExported: funcDecl.Name.IsExported(),
		Complexity: 1, // Basic complexity calculation
	}

	// Extract parameters
	if funcDecl.Type.Params != nil {
		for _, param := range funcDecl.Type.Params.List {
			paramType := a.typeToString(param.Type)
			for _, name := range param.Names {
				info.Params = append(info.Params, ParamInfo{
					Name: name.Name,
					Type: paramType,
				})
			}
		}
	}

	// Extract return values
	if funcDecl.Type.Results != nil {
		for i, result := range funcDecl.Type.Results.List {
			returnType := a.typeToString(result.Type)
			name := fmt.Sprintf("result%d", i)
			if len(result.Names) > 0 {
				name = result.Names[0].Name
			}
			info.Returns = append(info.Returns, ReturnInfo{
				Name: name,
				Type: returnType,
			})
		}
	}

	// Extract comments
	if funcDecl.Doc != nil {
		for _, comment := range funcDecl.Doc.List {
			info.Comments = append(info.Comments, strings.TrimSpace(comment.Text))
		}
	}

	// Calculate complexity (simplified)
	if funcDecl.Body != nil {
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
				info.Complexity++
			}
			return true
		})
	}

	return info
}

// typeToString converts an AST type to string representation
func (a *CodeAnalyzer) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return a.typeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + a.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + a.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + a.typeToString(t.Key) + "]" + a.typeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func"
	default:
		return "unknown"
	}
}

// TestCase represents a generated test case
type TestCase struct {
	Name       string
	Setup      string
	Call       string
	Assertions string
}

// GenerateTestsForFile generates tests for all functions in a file
func (g *TestGenerator) GenerateTestsForFile(filename string) error {
	functions, err := g.analyzer.AnalyzeFile(filename)
	if err != nil {
		return fmt.Errorf("failed to analyze file %s: %w", filename, err)
	}

	for _, function := range functions {
		if !function.IsExported {
			continue // Skip unexported functions
		}

		err := g.generateTestsForFunction(function, filename)
		if err != nil {
			return fmt.Errorf("failed to generate tests for function %s: %w", function.Name, err)
		}
	}

	return nil
}

// generateTestsForFunction generates tests for a specific function
func (g *TestGenerator) generateTestsForFunction(function *FunctionInfo, sourceFile string) error {
	testCases := g.generateTestCases(function)

	// Generate output filename
	dir := filepath.Dir(sourceFile)
	base := filepath.Base(sourceFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	outputFile := filepath.Join(dir, name+g.config.TestSuffix)

	// Generate test content
	content, err := g.generateTestFileContent(function, testCases)
	if err != nil {
		return fmt.Errorf("failed to generate test content: %w", err)
	}

	// Write to file
	err = os.WriteFile(outputFile, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write test file %s: %w", outputFile, err)
	}

	return nil
}

// generateTestCases creates test cases for a function using AI-like heuristics
func (g *TestGenerator) generateTestCases(function *FunctionInfo) []*TestCase {
	var testCases []*TestCase

	// Generate basic test cases based on function signature
	for i := 0; i < g.config.MaxTestsPerFunc; i++ {
		testCase := &TestCase{
			Name:       fmt.Sprintf("test_%s_%d", strings.ToLower(function.Name), i+1),
			Setup:      g.generateSetup(function, i),
			Call:       g.generateCall(function, i),
			Assertions: g.generateAssertions(function, i),
		}
		testCases = append(testCases, testCase)
	}

	// Generate edge cases if enabled
	if g.config.GenerateEdgeCases {
		edgeCases := g.generateEdgeCases(function)
		testCases = append(testCases, edgeCases...)
	}

	return testCases
}

// generateSetup creates setup code for a test case
func (g *TestGenerator) generateSetup(function *FunctionInfo, caseIndex int) string {
	var setup strings.Builder

	setup.WriteString("// Setup test data\n")

	for _, param := range function.Params {
		value := g.generateTestValue(param.Type, caseIndex)
		setup.WriteString(fmt.Sprintf("\t%s := %s\n", param.Name, value))
	}

	return setup.String()
}

// generateCall creates the function call for a test case
func (g *TestGenerator) generateCall(function *FunctionInfo, caseIndex int) string {
	var args []string
	for _, param := range function.Params {
		args = append(args, param.Name)
	}

	if len(function.Returns) > 0 {
		var returnVars []string
		for i := range function.Returns {
			returnVars = append(returnVars, fmt.Sprintf("result%d", i))
		}
		return fmt.Sprintf("%s := %s(%s)",
			strings.Join(returnVars, ", "),
			function.Name,
			strings.Join(args, ", "))
	}

	return fmt.Sprintf("%s(%s)", function.Name, strings.Join(args, ", "))
}

// generateAssertions creates assertions for a test case
func (g *TestGenerator) generateAssertions(function *FunctionInfo, caseIndex int) string {
	var assertions strings.Builder

	assertions.WriteString("// Verify results\n")

	for i, returnInfo := range function.Returns {
		varName := fmt.Sprintf("result%d", i)
		assertion := g.generateAssertion(returnInfo.Type, varName, caseIndex)
		assertions.WriteString(fmt.Sprintf("\t%s\n", assertion))
	}

	if len(function.Returns) == 0 {
		assertions.WriteString("\t// Function has no return values to verify\n")
	}

	return assertions.String()
}

// generateTestValue creates test values based on type
func (g *TestGenerator) generateTestValue(paramType string, caseIndex int) string {
	switch {
	case paramType == "string":
		testStrings := []string{`"test"`, `""`, `"hello world"`, `"special chars: !@#$%"`}
		return testStrings[caseIndex%len(testStrings)]
	case paramType == "int" || paramType == "int64" || paramType == "int32":
		testInts := []string{"0", "1", "-1", "100", "999999"}
		return testInts[caseIndex%len(testInts)]
	case paramType == "bool":
		if caseIndex%2 == 0 {
			return "true"
		}
		return "false"
	case strings.HasPrefix(paramType, "[]"):
		elementType := paramType[2:]
		return fmt.Sprintf("[]%s{%s}", elementType, g.generateTestValue(elementType, caseIndex))
	case strings.HasPrefix(paramType, "map["):
		return "make(" + paramType + ")"
	case strings.Contains(paramType, "interface"):
		return "nil"
	default:
		return "nil"
	}
}

// generateAssertion creates assertions based on return type
func (g *TestGenerator) generateAssertion(returnType, varName string, caseIndex int) string {
	switch {
	case returnType == "error":
		if caseIndex%2 == 0 {
			return fmt.Sprintf("if %s != nil { t.Errorf(\"Expected no error, got: %%v\", %s) }", varName, varName)
		}
		return fmt.Sprintf("if %s == nil { t.Error(\"Expected error, got nil\") }", varName)
	case returnType == "bool":
		expected := "true"
		if caseIndex%2 == 0 {
			expected = "false"
		}
		return fmt.Sprintf("if %s != %s { t.Errorf(\"Expected %s, got %%v\", %s) }", varName, expected, expected, varName)
	case returnType == "string":
		return fmt.Sprintf("if %s == \"\" { t.Error(\"Expected non-empty string\") }", varName)
	case returnType == "int" || returnType == "int64" || returnType == "int32":
		return fmt.Sprintf("if %s < 0 { t.Errorf(\"Expected non-negative value, got %%d\", %s) }", varName, varName)
	default:
		return fmt.Sprintf("if %s == nil { t.Error(\"Expected non-nil result\") }", varName)
	}
}

// generateEdgeCases creates edge case test scenarios
func (g *TestGenerator) generateEdgeCases(function *FunctionInfo) []*TestCase {
	var edgeCases []*TestCase

	// Nil parameter cases
	edgeCases = append(edgeCases, &TestCase{
		Name:       "edge_case_nil_params",
		Setup:      g.generateNilSetup(function),
		Call:       g.generateCall(function, 0),
		Assertions: g.generateErrorAssertion(),
	})

	// Empty/zero value cases
	edgeCases = append(edgeCases, &TestCase{
		Name:       "edge_case_empty_values",
		Setup:      g.generateEmptySetup(function),
		Call:       g.generateCall(function, 0),
		Assertions: g.generateEmptyAssertion(function),
	})

	return edgeCases
}

// generateNilSetup creates setup with nil parameters
func (g *TestGenerator) generateNilSetup(function *FunctionInfo) string {
	var setup strings.Builder
	setup.WriteString("// Setup with nil parameters\n")

	for _, param := range function.Params {
		if strings.Contains(param.Type, "*") || strings.Contains(param.Type, "interface") {
			setup.WriteString(fmt.Sprintf("\tvar %s %s // nil value\n", param.Name, param.Type))
		} else {
			setup.WriteString(fmt.Sprintf("\t%s := %s\n", param.Name, g.generateTestValue(param.Type, 0)))
		}
	}

	return setup.String()
}

// generateEmptySetup creates setup with empty/zero values
func (g *TestGenerator) generateEmptySetup(function *FunctionInfo) string {
	var setup strings.Builder
	setup.WriteString("// Setup with empty/zero values\n")

	for _, param := range function.Params {
		var value string
		switch param.Type {
		case "string":
			value = `""`
		case "int", "int64", "int32", "float64", "float32":
			value = "0"
		case "bool":
			value = "false"
		default:
			if strings.HasPrefix(param.Type, "[]") {
				value = fmt.Sprintf("%s{}", param.Type)
			} else if strings.HasPrefix(param.Type, "map[") {
				value = fmt.Sprintf("make(%s)", param.Type)
			} else {
				value = "nil"
			}
		}
		setup.WriteString(fmt.Sprintf("\t%s := %s\n", param.Name, value))
	}

	return setup.String()
}

// generateErrorAssertion creates assertion expecting an error
func (g *TestGenerator) generateErrorAssertion() string {
	return "\t// Should handle nil parameters gracefully\n\tif err := recover(); err == nil {\n\t\tt.Error(\"Expected panic or error with nil parameters\")\n\t}"
}

// generateEmptyAssertion creates assertion for empty value cases
func (g *TestGenerator) generateEmptyAssertion(function *FunctionInfo) string {
	return "\t// Should handle empty values appropriately\n\t// Add specific assertions based on expected behavior"
}

// generateTestFileContent generates the complete test file content
func (g *TestGenerator) generateTestFileContent(function *FunctionInfo, testCases []*TestCase) (string, error) {
	var content strings.Builder

	// Package declaration
	content.WriteString(fmt.Sprintf("package %s\n\n", function.Package))

	// Imports
	content.WriteString("import (\n\t\"testing\"\n)\n\n")

	// Generated timestamp comment
	content.WriteString(fmt.Sprintf("// Generated by AI Test Generator on %s\n", time.Now().Format(time.RFC3339)))
	content.WriteString("// This file contains automatically generated tests\n\n")

	// Generate unit tests
	for _, testCase := range testCases {
		content.WriteString(fmt.Sprintf("func Test%s_%s(t *testing.T) {\n", function.Name, strings.Title(testCase.Name)))
		content.WriteString(testCase.Setup)
		content.WriteString("\n")
		content.WriteString("\t" + testCase.Call + "\n")
		content.WriteString("\n")
		content.WriteString(testCase.Assertions)
		content.WriteString("}\n\n")
	}

	// Generate benchmark test if enabled
	if g.config.GenerateBenchmarks {
		content.WriteString(g.generateBenchmarkTest(function))
	}

	return content.String(), nil
}

// generateBenchmarkTest creates a benchmark test for the function
func (g *TestGenerator) generateBenchmarkTest(function *FunctionInfo) string {
	var benchmark strings.Builder

	benchmark.WriteString(fmt.Sprintf("func Benchmark%s(b *testing.B) {\n", function.Name))
	benchmark.WriteString(g.generateSetup(function, 0))
	benchmark.WriteString("\n\tb.ResetTimer()\n")
	benchmark.WriteString("\tfor i := 0; i < b.N; i++ {\n")
	benchmark.WriteString("\t\t" + g.generateCall(function, 0) + "\n")
	benchmark.WriteString("\t}\n")
	benchmark.WriteString("}\n\n")

	return benchmark.String()
}

// GenerateTestsForDirectory generates tests for all Go files in a directory
func (g *TestGenerator) GenerateTestsForDirectory(dir string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Check exclude patterns
		for _, pattern := range g.config.ExcludePatterns {
			if matched, _ := filepath.Match(pattern, path); matched {
				return nil
			}
		}

		return g.GenerateTestsForFile(path)
	})

	return err
}

// GetGenerationStats returns statistics about test generation
func (g *TestGenerator) GetGenerationStats() *GenerationStats {
	return &GenerationStats{
		FilesProcessed: 0, // Would be tracked during generation
		TestsGenerated: 0,
		Coverage:       0.0,
		Timestamp:      time.Now(),
	}
}

// GenerationStats holds statistics about test generation
type GenerationStats struct {
	FilesProcessed int       `json:"files_processed"`
	TestsGenerated int       `json:"tests_generated"`
	Coverage       float64   `json:"coverage"`
	Timestamp      time.Time `json:"timestamp"`
}
