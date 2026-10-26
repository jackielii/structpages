package structpages

import (
	"strings"
	"testing"
)

// Test boundReceiverFromFuncName with various edge cases
func TestBoundReceiverFromFuncName(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		pkgPath  string
		typeName string
	}{
		{
			name:     "pointer receiver with full path",
			funcName: "github.com/user/pkg.(*TypeName).Method-fm",
			pkgPath:  "github.com/user/pkg",
			typeName: "TypeName",
		},
		{
			name:     "value receiver with full path",
			funcName: "github.com/user/pkg.TypeName.Method-fm",
			pkgPath:  "github.com/user/pkg",
			typeName: "TypeName",
		},
		{
			name:     "generic receiver",
			funcName: "github.com/user/pkg.Gen[...].Method-fm",
			pkgPath:  "github.com/user/pkg",
			typeName: "Gen[...]",
		},
		{
			name:     "dot in the last path element is escaped",
			funcName: "gopkg.in/yaml%2ev3.(*Node).Method-fm",
			pkgPath:  "gopkg.in/yaml.v3",
			typeName: "Node",
		},
		{
			name:     "dot in an earlier path element",
			funcName: "example.com/a.b/pkg.T.Method-fm",
			pkgPath:  "example.com/a.b/pkg",
			typeName: "T",
		},
		{
			name:     "pointer receiver in main",
			funcName: "main.(*MyType).Method-fm",
			pkgPath:  "main",
			typeName: "MyType",
		},
		{
			name:     "value receiver in main",
			funcName: "main.MyType.Method-fm",
			pkgPath:  "main",
			typeName: "MyType",
		},
		{
			name:     "no dots in name",
			funcName: "NoDotsAtAll",
		},
		{
			name:     "no dot after the last slash",
			funcName: "example.com/nodot",
		},
		{
			name:     "pointer receiver without closing paren",
			funcName: "main.(*BrokenType.Method",
		},
		{
			name:     "value receiver without method",
			funcName: "main.TypeNameOnly",
		},
		{
			name:     "only package name",
			funcName: "main.",
		},
		{
			name:     "malformed escape",
			funcName: "example.com/bad%zz.T.Method-fm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgPath, typeName := boundReceiverFromFuncName(tt.funcName)
			if typeName != tt.typeName || (typeName != "" && pkgPath != tt.pkgPath) {
				t.Errorf("boundReceiverFromFuncName(%q) = %q, %q, want %q, %q",
					tt.funcName, pkgPath, typeName, tt.pkgPath, tt.typeName)
			}
		})
	}
}

// Test extractMethodNameFromFullName edge cases
func TestExtractMethodNameFromFullName(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		expected string
	}{
		{
			name:     "method with -fm suffix",
			fullName: "github.com/user/pkg.(*Type).Method-fm",
			expected: "Method",
		},
		{
			name:     "method without suffix",
			fullName: "github.com/user/pkg.(*Type).Method",
			expected: "Method",
		},
		{
			name:     "no dots in name",
			fullName: "NoDotsAtAll",
			expected: "NoDotsAtAll",
		},
		{
			name:     "ending with dot",
			fullName: "main.Type.",
			expected: "",
		},
		{
			name:     "pointer method",
			fullName: "main.(*Type).DoSomething-fm",
			expected: "DoSomething",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMethodNameFromFullName(tt.fullName)
			if result != tt.expected {
				t.Errorf("extractMethodNameFromFullName(%q) = %q, want %q",
					tt.fullName, result, tt.expected)
			}
		})
	}
}

// Test isMethodPattern edge cases
func TestIsMethodPattern(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		expected bool
	}{
		{
			name:     "pointer receiver with method",
			fullName: "main.(*Type).Method",
			expected: true,
		},
		{
			name:     "value receiver with method",
			fullName: "main.Type.Method",
			expected: true,
		},
		{
			name:     "with -fm suffix",
			fullName: "github.com/pkg.(*Type).Method-fm",
			expected: true,
		},
		{
			name:     "no dots",
			fullName: "NoPattern",
			expected: false,
		},
		{
			name:     "only one dot",
			fullName: "package.Type",
			expected: false,
		},
		{
			name:     "function without receiver",
			fullName: "main.Function",
			expected: false,
		},
		{
			name:     "empty string",
			fullName: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMethodPattern(tt.fullName)
			if result != tt.expected {
				t.Errorf("isMethodPattern(%q) = %v, want %v",
					tt.fullName, result, tt.expected)
			}
		})
	}
}

// Test extractMethodInfo with edge cases for coverage
func TestExtractMethodInfo_EdgeCases(t *testing.T) {
	// Test with invalid input (non-function)
	_, err := extractMethodInfo("not a function")
	if err == nil {
		t.Error("Expected error for non-function input")
	}
	if !strings.Contains(err.Error(), "not a function") {
		t.Errorf("Expected 'not a function' error, got: %v", err)
	}

	// Test with nil
	_, err = extractMethodInfo(nil)
	if err == nil {
		t.Error("Expected error for nil input")
	}

	// Test with a type that's not a function
	_, err = extractMethodInfo(123)
	if err == nil {
		t.Error("Expected error for non-function type")
	}
}
