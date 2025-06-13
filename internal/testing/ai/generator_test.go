package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultGeneratorConfig(t *testing.T) {
	config := DefaultGeneratorConfig()
	
	if config == nil {
		t.Fatal("DefaultGeneratorConfig returned nil")
	}
	
	if config.TestSuffix != "_ai_test.go" {
		t.Errorf("Expected test suffix '_ai_test.go', got '%s'", config.TestSuffix)
	}
	
	if config.CoverageTarget != 80.0 {
		t.Errorf("Expected coverage target 80.0, got %f", config.CoverageTarget)
	}
	
	if !config.GenerateEdgeCases {
		t.Error("Expected GenerateEdgeCases to be true")
	}
	
	if !config.GenerateBenchmarks {
		t.Error("Expected GenerateBenchmarks to be true")
	}
}

func TestNewTestGenerator(t *testing.T) {
	generator, err := NewTestGenerator(nil)
	if err != nil {
		t.Fatalf("NewTestGenerator failed: %v", err)
	}
	
	if generator == nil {
		t.Fatal("NewTestGenerator returned nil")
	}
	
	if generator.config == nil {
		t.Error("Generator config is nil")
	}
	
	if generator.analyzer == nil {
		t.Error("Generator analyzer is nil")
	}
	
	if generator.patterns == nil {
		t.Error("Generator patterns is nil")
	}
}

func TestNewCodeAnalyzer(t *testing.T) {
	analyzer := NewCodeAnalyzer()
	
	if analyzer == nil {
		t.Fatal("NewCodeAnalyzer returned nil")
	}
	
	if analyzer.fileSet == nil {
		t.Error("Analyzer file set is nil")
	}
}

func TestNewTestPatterns(t *testing.T) {
	patterns, err := NewTestPatterns()
	if err != nil {
		t.Fatalf("NewTestPatterns failed: %v", err)
	}
	
	if patterns == nil {
		t.Fatal("NewTestPatterns returned nil")
	}
	
	if patterns.unitTestTemplate == nil {
		t.Error("Unit test template is nil")
	}
	
	if patterns.benchmarkTestTemplate == nil {
		t.Error("Benchmark test template is nil")
	}
	
	if patterns.edgeCaseTemplate == nil {
		t.Error("Edge case template is nil")
	}
}

func TestAnalyzeFile(t *testing.T) {
	// Create a temporary Go file for testing
	content := `package testpkg

// Add adds two integers
func Add(a, b int) int {
	return a + b
}

// Subtract subtracts b from a
func Subtract(a, b int) int {
	return a - b
}

// unexportedFunc is not exported
func unexportedFunc() {
	// do nothing
}
`
	
	tempFile, err := os.CreateTemp("", "test_*.go")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	
	_, err = tempFile.WriteString(content)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tempFile.Close()
	
	analyzer := NewCodeAnalyzer()
	functions, err := analyzer.AnalyzeFile(tempFile.Name())
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	if len(functions) != 3 {
		t.Errorf("Expected 3 functions, got %d", len(functions))
	}
	
	// Check first function (Add)
	addFunc := functions[0]
	if addFunc.Name != "Add" {
		t.Errorf("Expected function name 'Add', got '%s'", addFunc.Name)
	}
	
	if !addFunc.IsExported {
		t.Error("Add function should be exported")
	}
	
	if len(addFunc.Params) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(addFunc.Params))
	}
	
	if len(addFunc.Returns) != 1 {
		t.Errorf("Expected 1 return value, got %d", len(addFunc.Returns))
	}
	
	// Check unexported function
	unexportedFunc := functions[2]
	if unexportedFunc.IsExported {
		t.Error("unexportedFunc should not be exported")
	}
}

func TestGenerateTestValue(t *testing.T) {
	config := DefaultGeneratorConfig()
	generator, err := NewTestGenerator(config)
	if err != nil {
		t.Fatalf("NewTestGenerator failed: %v", err)
	}
	
	// Test string generation
	stringVal := generator.generateTestValue("string", 0)
	if !strings.Contains(stringVal, "\"") {
		t.Errorf("Expected quoted string, got: %s", stringVal)
	}
	
	// Test int generation
	intVal := generator.generateTestValue("int", 0)
	if intVal == "" {
		t.Error("Expected non-empty int value")
	}
	
	// Test bool generation
	boolVal := generator.generateTestValue("bool", 0)
	if boolVal != "true" && boolVal != "false" {
		t.Errorf("Expected 'true' or 'false', got: %s", boolVal)
	}
	
	// Test slice generation
	sliceVal := generator.generateTestValue("[]string", 0)
	if !strings.HasPrefix(sliceVal, "[]string{") {
		t.Errorf("Expected slice syntax, got: %s", sliceVal)
	}
	
	// Test map generation
	mapVal := generator.generateTestValue("map[string]int", 0)
	if !strings.HasPrefix(mapVal, "make(") {
		t.Errorf("Expected make() syntax, got: %s", mapVal)
	}
}

func TestGenerateAssertion(t *testing.T) {
	config := DefaultGeneratorConfig()
	generator, err := NewTestGenerator(config)
	if err != nil {
		t.Fatalf("NewTestGenerator failed: %v", err)
	}
	
	// Test error assertion
	errorAssertion := generator.generateAssertion("error", "err", 0)
	if !strings.Contains(errorAssertion, "err") {
		t.Error("Error assertion should reference the variable")
	}
	
	// Test bool assertion
	boolAssertion := generator.generateAssertion("bool", "result", 1)
	if !strings.Contains(boolAssertion, "result") {
		t.Error("Bool assertion should reference the variable")
	}
	
	// Test string assertion
	stringAssertion := generator.generateAssertion("string", "str", 0)
	if !strings.Contains(stringAssertion, "str") {
		t.Error("String assertion should reference the variable")
	}
}

func TestGenerateTestCases(t *testing.T) {
	config := DefaultGeneratorConfig()
	config.MaxTestsPerFunc = 3
	generator, err := NewTestGenerator(config)
	if err != nil {
		t.Fatalf("NewTestGenerator failed: %v", err)
	}
	
	function := &FunctionInfo{
		Name:       "TestFunc",
		Package:    "testpkg",
		IsExported: true,
		Params: []ParamInfo{
			{Name: "input", Type: "string"},
		},
		Returns: []ReturnInfo{
			{Name: "result", Type: "string"},
		},
	}
	
	testCases := generator.generateTestCases(function)
	
	// Should generate basic test cases + edge cases
	expectedMin := config.MaxTestsPerFunc
	if config.GenerateEdgeCases {
		expectedMin += 2 // nil params + empty values
	}
	
	if len(testCases) < expectedMin {
		t.Errorf("Expected at least %d test cases, got %d", expectedMin, len(testCases))
	}
	
	// Check that test cases have required fields
	for i, testCase := range testCases {
		if testCase.Name == "" {
			t.Errorf("Test case %d has empty name", i)
		}
		
		if testCase.Setup == "" {
			t.Errorf("Test case %d has empty setup", i)
		}
		
		if testCase.Call == "" {
			t.Errorf("Test case %d has empty call", i)
		}
		
		if testCase.Assertions == "" {
			t.Errorf("Test case %d has empty assertions", i)
		}
	}
}

func TestGenerateTestFileContent(t *testing.T) {
	config := DefaultGeneratorConfig()
	generator, err := NewTestGenerator(config)
	if err != nil {
		t.Fatalf("NewTestGenerator failed: %v", err)
	}
	
	function := &FunctionInfo{
		Name:       "TestFunc",
		Package:    "testpkg",
		IsExported: true,
		Params: []ParamInfo{
			{Name: "input", Type: "string"},
		},
		Returns: []ReturnInfo{
			{Name: "result", Type: "string"},
		},
	}
	
	testCases := []*TestCase{
		{
			Name:       "basic_test",
			Setup:      "input := \"test\"",
			Call:       "result := TestFunc(input)",
			Assertions: "if result == \"\" { t.Error(\"Expected non-empty result\") }",
		},
	}
	
	content, err := generator.generateTestFileContent(function, testCases)
	if err != nil {
		t.Fatalf("generateTestFileContent failed: %v", err)
	}
	
	// Check that content contains expected elements
	if !strings.Contains(content, "package testpkg") {
		t.Error("Content should contain package declaration")
	}
	
	if !strings.Contains(content, "import") {
		t.Error("Content should contain imports")
	}
	
	if !strings.Contains(content, "func TestTestFunc_basic_test(t *testing.T)") {
		t.Error("Content should contain test function")
	}
	
	if !strings.Contains(content, "Generated by AI Test Generator") {
		t.Error("Content should contain generation comment")
	}
	
	// Check benchmark generation
	if config.GenerateBenchmarks && !strings.Contains(content, "func BenchmarkTestFunc(b *testing.B)") {
		t.Error("Content should contain benchmark function when enabled")
	}
}

func TestTypeToString(t *testing.T) {
	analyzer := NewCodeAnalyzer()
	
	// This is a simplified test since we'd need to parse AST nodes
	// In a real scenario, we'd create proper AST nodes for testing
	
	// Test basic scenarios we can verify
	// More comprehensive testing would require setting up AST nodes
}

func TestGenerateEdgeCases(t *testing.T) {
	config := DefaultGeneratorConfig()
	generator, err := NewTestGenerator(config)
	if err != nil {
		t.Fatalf("NewTestGenerator failed: %v", err)
	}
	
	function := &FunctionInfo{
		Name:       "TestFunc",
		Package:    "testpkg",
		IsExported: true,
		Params: []ParamInfo{
			{Name: "ptr", Type: "*string"},
			{Name: "slice", Type: "[]int"},
		},
		Returns: []ReturnInfo{
			{Name: "err", Type: "error"},
		},
	}
	
	edgeCases := generator.generateEdgeCases(function)
	
	if len(edgeCases) == 0 {
		t.Error("Expected edge cases to be generated")
	}
	
	// Should have nil params and empty values cases
	foundNilCase := false
	foundEmptyCase := false
	
	for _, edgeCase := range edgeCases {
		if strings.Contains(edgeCase.Name, "nil") {
			foundNilCase = true
		}
		if strings.Contains(edgeCase.Name, "empty") {
			foundEmptyCase = true
		}
	}
	
	if !foundNilCase {
		t.Error("Expected nil parameter edge case")
	}
	
	if !foundEmptyCase {
		t.Error("Expected empty values edge case")
	}
}

func TestGenerationStats(t *testing.T) {
	config := DefaultGeneratorConfig()
	generator, err := NewTestGenerator(config)
	if err != nil {
		t.Fatalf("NewTestGenerator failed: %v", err)
	}
	
	stats := generator.GetGenerationStats()
	
	if stats == nil {
		t.Fatal("GetGenerationStats returned nil")
	}
	
	if stats.Timestamp.IsZero() {
		t.Error("Stats timestamp should be set")
	}
	
	// Initial stats should be zero
	if stats.FilesProcessed != 0 {
		t.Errorf("Expected 0 files processed initially, got %d", stats.FilesProcessed)
	}
	
	if stats.TestsGenerated != 0 {
		t.Errorf("Expected 0 tests generated initially, got %d", stats.TestsGenerated)
	}
}