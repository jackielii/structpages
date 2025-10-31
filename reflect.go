package structpages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
)

// methodInfo holds extracted information about a method expression or function
type methodInfo struct {
	methodName       string
	receiverType     reflect.Type // nil for bound methods and standalone functions
	receiverTypeName string       // for bound methods
	isBound          bool
	isFunction       bool // true for standalone functions (not methods)
}

// extractMethodInfo extracts method information from either:
// - Unbound method expressions: Type.Method or (*Type).Method
// - Bound method values: instance.Method
// - Standalone functions: packageName.FunctionName
func extractMethodInfo(methodExpr any) (*methodInfo, error) {
	v := reflect.ValueOf(methodExpr)
	if v.Kind() != reflect.Func {
		return nil, errors.New("not a function")
	}

	// Get function metadata
	funcPC := v.Pointer()
	fn := runtime.FuncForPC(funcPC)
	if fn == nil {
		return nil, errors.New("cannot get function info")
	}

	fullName := fn.Name()

	// Extract method name (works for both bound and unbound)
	methodName := extractMethodNameFromFullName(fullName)
	if methodName == "" {
		return nil, errors.New("failed to extract method name from expression")
	}

	funcType := v.Type()

	// Check if this is a bound method (instance.Method)
	// Bound methods have "-fm" suffix in the name (receiver already bound)
	// The number of input parameters varies based on the method's arguments
	// Method pattern in the name: "package.(*Type).Method-fm"
	isBound := strings.Contains(fullName, "-fm")

	if isBound {
		// Extract receiver type name from function name
		typeName := extractReceiverTypeNameFromFuncName(fullName)
		if typeName == "" {
			return nil, fmt.Errorf("cannot extract receiver type from bound method: %s", fullName)
		}
		return &methodInfo{
			methodName:       methodName,
			receiverTypeName: typeName,
			isBound:          true,
			isFunction:       false,
		}, nil
	}

	// Check if this is a standalone function (not a method)
	// Standalone functions have the pattern: "package.FunctionName"
	// Methods have the pattern: "package.(*Type).Method" or "package.Type.Method"
	isStandaloneFunc := !isMethodPattern(fullName)

	if isStandaloneFunc {
		// It's a standalone function, not a method
		return &methodInfo{
			methodName:   methodName,
			receiverType: nil, // No receiver type
			isBound:      false,
			isFunction:   true,
		}, nil
	}

	// Unbound method expression - extract receiver from first parameter
	if funcType.NumIn() == 0 {
		return nil, errors.New("failed to extract receiver type from method expression")
	}

	receiverType := funcType.In(0)
	if receiverType.Kind() == reflect.Pointer {
		receiverType = receiverType.Elem()
	}

	return &methodInfo{
		methodName:   methodName,
		receiverType: receiverType,
		isBound:      false,
		isFunction:   false,
	}, nil
}

// extractMethodName extracts the method name from a method expression using reflection.
// It handles both unbound method expressions and bound method values.
// This is a convenience wrapper around extractMethodInfo for backward compatibility.
func extractMethodName(methodExpr any) string {
	info, err := extractMethodInfo(methodExpr)
	if err != nil {
		return ""
	}
	return info.methodName
}

// extractReceiverType extracts the receiver type from a method expression.
// Returns the receiver type (as a non-pointer type for consistency) or nil if extraction fails.
// For bound method values (instance.Method), returns nil.
// This is a convenience wrapper around extractMethodInfo for backward compatibility.
func extractReceiverType(methodExpr any) reflect.Type {
	info, err := extractMethodInfo(methodExpr)
	if err != nil {
		return nil
	}
	return info.receiverType // nil for bound methods
}

// extractMethodNameFromFullName extracts the method name from a full function name.
// Examples:
//   - "github.com/user/pkg.(*Type).Method-fm" -> "Method"
//   - "github.com/user/pkg.Type.Method" -> "Method"
//   - "main.Type.Method-fm" -> "Method"
func extractMethodNameFromFullName(fullName string) string {
	// Find the last dot
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot == -1 {
		return fullName
	}

	methodName := fullName[lastDot+1:]

	// Remove "-fm" suffix if present (bound methods)
	if idx := strings.Index(methodName, "-fm"); idx != -1 {
		methodName = methodName[:idx]
	}

	return methodName
}

// isMethodPattern checks if a full function name represents a method (vs standalone function).
// Method patterns: "package.(*Type).Method" or "package.Type.Method"
// Function patterns: "package.FunctionName" or "package/subpackage.FunctionName"
func isMethodPattern(fullName string) bool {
	// Find the last package separator
	lastSlash := strings.LastIndex(fullName, "/")
	remainder := fullName
	if lastSlash != -1 {
		remainder = fullName[lastSlash+1:]
	}

	// Skip the package name (first component before dot)
	firstDot := strings.Index(remainder, ".")
	if firstDot == -1 {
		return false // No dot means it's not even a valid function name
	}
	remainder = remainder[firstDot+1:]

	// Check for method patterns:
	// - "(*Type).Method" (pointer receiver)
	// - "Type.Method" (value receiver, has another dot)
	if strings.HasPrefix(remainder, "(*") {
		return true // Pointer receiver method pattern
	}

	// Check if there's another dot (indicates Type.Method pattern)
	return strings.Contains(remainder, ".")
}

// extractReceiverTypeNameFromFuncName extracts the receiver type name from a function name.
// Examples:
//   - "github.com/user/pkg.(*TypeName).Method-fm" -> "TypeName"
//   - "github.com/user/pkg.TypeName.Method-fm" -> "TypeName"
//   - "main.(*MyType).Method-fm" -> "MyType"
func extractReceiverTypeNameFromFuncName(fullName string) string {
	// Find the last package separator
	lastSlash := strings.LastIndex(fullName, "/")
	remainder := fullName
	if lastSlash != -1 {
		remainder = fullName[lastSlash+1:]
	}

	// Look for the pattern: "package.(*Type).Method" or "package.Type.Method"
	// First, find the package name and skip it
	dotIdx := strings.Index(remainder, ".")
	if dotIdx == -1 {
		return ""
	}
	remainder = remainder[dotIdx+1:]

	// Check for pointer receiver: (*Type)
	if strings.HasPrefix(remainder, "(*") {
		closeParenIdx := strings.Index(remainder, ")")
		if closeParenIdx == -1 {
			return ""
		}
		return remainder[2:closeParenIdx]
	}

	// Value receiver: Type.Method
	dotIdx = strings.Index(remainder, ".")
	if dotIdx == -1 {
		return ""
	}

	return remainder[:dotIdx]
}

// pointerType normalizes a type to its pointer variant.
// If the type is already a pointer, returns it as-is.
// Otherwise, returns a pointer to the type.
func pointerType(v reflect.Type) reflect.Type {
	if v.Kind() == reflect.Pointer {
		return v
	}
	return reflect.PointerTo(v)
}

// component interface represents a renderable component
type component interface {
	Render(context.Context, io.Writer) error
}

// isComponent checks if a method returns a component.
func isComponent(t *reflect.Method) bool {
	typ := reflect.TypeOf((*component)(nil)).Elem()
	if t.Type.NumOut() != 1 {
		return false
	}
	return t.Type.Out(0).Implements(typ)
}

// isPromotedMethod checks if a method is promoted from an embedded type.
// Promoted methods have an autogenerated wrapper that can be detected.
// See: https://github.com/golang/go/issues/73883
func isPromotedMethod(method *reflect.Method) bool {
	wPC := method.Func.Pointer()
	wFunc := runtime.FuncForPC(wPC)
	wFile, wLine := wFunc.FileLine(wPC)
	return wFile == "<autogenerated>" && wLine == 1
}

// extractError extracts an error from the last return value if it implements error.
// Returns the remaining values and the extracted error (or nil if no error).
func extractError(args []reflect.Value) ([]reflect.Value, error) {
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if len(args) >= 1 && args[len(args)-1].Type().AssignableTo(errorType) {
		i := args[len(args)-1].Interface()
		args = args[:len(args)-1]
		if i == nil {
			return args, nil
		}
		return args, i.(error)
	}
	return args, nil
}

// formatMethod formats a method for display as "ReceiverType.MethodName".
func formatMethod(method *reflect.Method) string {
	if method == nil || !method.Func.IsValid() {
		return "<nil>"
	}
	receiver := method.Type.In(0)
	if receiver.Kind() == reflect.Pointer {
		receiver = receiver.Elem()
	}
	return fmt.Sprintf("%s.%s", receiver.String(), method.Name)
}
