package preprocessor

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
)

// TestExtractStructLiteral tests parsing expr? Type{...} patterns
func TestExtractStructLiteral(t *testing.T) {
	proc := NewErrorPropProcessor()

	tests := []struct {
		name         string
		input        string
		wantExpr     string
		wantType     ErrorExprType
		wantStruct   string
		wantFields   string
		wantInferred string
	}{
		{
			name:         "simple struct literal",
			input:        `expr? ServiceError{Code: "DB_ERROR"}`,
			wantExpr:     "expr?",
			wantType:     ErrorExprStructLit,
			wantStruct:   "ServiceError",
			wantFields:   `Code: "DB_ERROR"`,
			wantInferred: "ServiceError",
		},
		{
			name:         "struct with auto-bound err",
			input:        `db.Query(...)? ServiceError{Code: "DB_ERROR", Message: err.Error()}`,
			wantExpr:     "db.Query(...)?",
			wantType:     ErrorExprStructLit,
			wantStruct:   "ServiceError",
			wantFields:   `Code: "DB_ERROR", Message: err.Error()`,
			wantInferred: "ServiceError",
		},
		{
			name:         "struct with nested braces",
			input:        `expr? ServiceError{Code: "ERR", Meta: map[string]string{"key": "value"}}`,
			wantExpr:     "expr?",
			wantType:     ErrorExprStructLit,
			wantStruct:   "ServiceError",
			wantFields:   `Code: "ERR", Meta: map[string]string{"key": "value"}`,
			wantInferred: "ServiceError",
		},
		{
			name:         "struct with multiline fields",
			input:        `expr? ServiceError{Code: "DB_ERROR", Message: "long message"}`,
			wantExpr:     "expr?",
			wantType:     ErrorExprStructLit,
			wantStruct:   "ServiceError",
			wantFields:   `Code: "DB_ERROR", Message: "long message"`,
			wantInferred: "ServiceError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, errExpr := proc.extractExpressionAndErrorExpr(tt.input)

			if expr != tt.wantExpr {
				t.Errorf("expr = %q, want %q", expr, tt.wantExpr)
			}
			if errExpr.ExprType != tt.wantType {
				t.Errorf("type = %v, want %v", errExpr.ExprType, tt.wantType)
			}
			if errExpr.StructType != tt.wantStruct {
				t.Errorf("struct type = %q, want %q", errExpr.StructType, tt.wantStruct)
			}
			if errExpr.StructFields != tt.wantFields {
				t.Errorf("fields = %q, want %q", errExpr.StructFields, tt.wantFields)
			}
			if errExpr.InferredType != tt.wantInferred {
				t.Errorf("inferred type = %q, want %q", errExpr.InferredType, tt.wantInferred)
			}
		})
	}
}

// TestExtractMethodCall tests parsing expr? Type.Method(...) patterns
func TestExtractMethodCall(t *testing.T) {
	proc := NewErrorPropProcessor()

	tests := []struct {
		name         string
		input        string
		wantExpr     string
		wantType     ErrorExprType
		wantReceiver string
		wantMethod   string
		wantArgs     string
		wantInferred string
	}{
		{
			name:         "simple method call",
			input:        `expr? ServiceError.NewDBError(err)`,
			wantExpr:     "expr?",
			wantType:     ErrorExprMethodCall,
			wantReceiver: "ServiceError",
			wantMethod:   "NewDBError",
			wantArgs:     "err",
			wantInferred: "ServiceError",
		},
		{
			name:         "method call with multiple args",
			input:        `db.Query(...)? ServiceError.New("DB_ERROR", err)`,
			wantExpr:     "db.Query(...)?",
			wantType:     ErrorExprMethodCall,
			wantReceiver: "ServiceError",
			wantMethod:   "New",
			wantArgs:     `"DB_ERROR", err`,
			wantInferred: "ServiceError",
		},
		{
			name:         "method call with nested parens",
			input:        `expr? ServiceError.Wrap(fmt.Sprintf("failed: %v", err))`,
			wantExpr:     "expr?",
			wantType:     ErrorExprMethodCall,
			wantReceiver: "ServiceError",
			wantMethod:   "Wrap",
			wantArgs:     `fmt.Sprintf("failed: %v", err)`,
			wantInferred: "ServiceError",
		},
		{
			name:         "method call no args",
			input:        `expr? ServiceError.Default()`,
			wantExpr:     "expr?",
			wantType:     ErrorExprMethodCall,
			wantReceiver: "ServiceError",
			wantMethod:   "Default",
			wantArgs:     "",
			wantInferred: "ServiceError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, errExpr := proc.extractExpressionAndErrorExpr(tt.input)

			if expr != tt.wantExpr {
				t.Errorf("expr = %q, want %q", expr, tt.wantExpr)
			}
			if errExpr.ExprType != tt.wantType {
				t.Errorf("type = %v, want %v", errExpr.ExprType, tt.wantType)
			}
			if errExpr.ReceiverType != tt.wantReceiver {
				t.Errorf("receiver = %q, want %q", errExpr.ReceiverType, tt.wantReceiver)
			}
			if errExpr.MethodName != tt.wantMethod {
				t.Errorf("method = %q, want %q", errExpr.MethodName, tt.wantMethod)
			}
			if errExpr.MethodArgs != tt.wantArgs {
				t.Errorf("args = %q, want %q", errExpr.MethodArgs, tt.wantArgs)
			}
			if errExpr.InferredType != tt.wantInferred {
				t.Errorf("inferred type = %q, want %q", errExpr.InferredType, tt.wantInferred)
			}
		})
	}
}

// TestExtractFuncCall tests parsing expr? Func(...) patterns
func TestExtractFuncCall(t *testing.T) {
	proc := NewErrorPropProcessor()

	tests := []struct {
		name     string
		input    string
		wantExpr string
		wantType ErrorExprType
		wantFunc string
		wantArgs string
	}{
		{
			name:     "simple function call",
			input:    `expr? WrapDBError(err)`,
			wantExpr: "expr?",
			wantType: ErrorExprFuncCall,
			wantFunc: "WrapDBError",
			wantArgs: "err",
		},
		{
			name:     "function call with multiple args",
			input:    `db.Query(...)? NewServiceError("DB_ERROR", err)`,
			wantExpr: "db.Query(...)?",
			wantType: ErrorExprFuncCall,
			wantFunc: "NewServiceError",
			wantArgs: `"DB_ERROR", err`,
		},
		{
			name:     "function call with nested parens",
			input:    `expr? WrapError(fmt.Errorf("failed: %w", err))`,
			wantExpr: "expr?",
			wantType: ErrorExprFuncCall,
			wantFunc: "WrapError",
			wantArgs: `fmt.Errorf("failed: %w", err)`,
		},
		{
			name:     "function call no args",
			input:    `expr? DefaultError()`,
			wantExpr: "expr?",
			wantType: ErrorExprFuncCall,
			wantFunc: "DefaultError",
			wantArgs: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, errExpr := proc.extractExpressionAndErrorExpr(tt.input)

			if expr != tt.wantExpr {
				t.Errorf("expr = %q, want %q", expr, tt.wantExpr)
			}
			if errExpr.ExprType != tt.wantType {
				t.Errorf("type = %v, want %v", errExpr.ExprType, tt.wantType)
			}
			if errExpr.FuncName != tt.wantFunc {
				t.Errorf("func name = %q, want %q", errExpr.FuncName, tt.wantFunc)
			}
			if errExpr.FuncArgs != tt.wantArgs {
				t.Errorf("args = %q, want %q", errExpr.FuncArgs, tt.wantArgs)
			}
			// Note: InferredType is empty for function calls (cannot infer)
			if errExpr.InferredType != "" {
				t.Errorf("inferred type = %q, want empty", errExpr.InferredType)
			}
		})
	}
}

// TestResultTypeDetection tests detecting Result<T, E> and extracting E
func TestResultTypeDetection(t *testing.T) {
	proc := NewErrorPropProcessor()

	tests := []struct {
		name         string
		returnType   string
		wantIsResult bool
		wantOkType   string
		wantErrType  string
		wantTypeName string
	}{
		{
			name:         "Result with angle brackets",
			returnType:   "Result<[]Order, ServiceError>",
			wantIsResult: true,
			wantOkType:   "[]Order",
			wantErrType:  "ServiceError",
			wantTypeName: "ResultSliceOrderServiceError",
		},
		{
			name:         "Result with square brackets",
			returnType:   "Result[string, error]",
			wantIsResult: true,
			wantOkType:   "string",
			wantErrType:  "error",
			wantTypeName: "ResultStringError",
		},
		{
			name:         "Result with pointer type",
			returnType:   "Result<*User, ServiceError>",
			wantIsResult: true,
			wantOkType:   "*User",
			wantErrType:  "ServiceError",
			wantTypeName: "ResultPtrUserServiceError",
		},
		{
			name:         "Result with map type",
			returnType:   "Result<map[string]int, ServiceError>",
			wantIsResult: true,
			wantOkType:   "map[string]int",
			wantErrType:  "ServiceError",
			wantTypeName: "ResultMapServiceError",  // CRITICAL FIX C2: builtin.SanitizeTypeName simplifies maps to "Map"
		},
		{
			name:         "Not a Result type",
			returnType:   "([]Order, error)",
			wantIsResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proc.parseResultType(tt.returnType)

			if tt.wantIsResult {
				if result == nil {
					t.Fatal("expected Result type to be detected, got nil")
				}
				if result.okType != tt.wantOkType {
					t.Errorf("ok type = %q, want %q", result.okType, tt.wantOkType)
				}
				if result.errType != tt.wantErrType {
					t.Errorf("err type = %q, want %q", result.errType, tt.wantErrType)
				}
				if result.typeName != tt.wantTypeName {
					t.Errorf("type name = %q, want %q", result.typeName, tt.wantTypeName)
				}
			} else {
				if result != nil {
					t.Errorf("expected non-Result type, got Result detection: %+v", result)
				}
			}
		})
	}
}

// TestTypeValidation tests validating type matches and mismatches
func TestTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		returnType  string
		errorExpr   ErrorExpr
		wantErr     bool
		wantErrText string
	}{
		{
			name:       "struct literal matches Result error type",
			returnType: "Result<[]Order, ServiceError>",
			errorExpr: ErrorExpr{
				ExprType:     ErrorExprStructLit,
				StructType:   "ServiceError",
				InferredType: "ServiceError",
			},
			wantErr: false,
		},
		{
			name:       "struct literal type mismatch",
			returnType: "Result<[]Order, ServiceError>",
			errorExpr: ErrorExpr{
				ExprType:     ErrorExprStructLit,
				StructType:   "OtherError",
				InferredType: "OtherError",
			},
			wantErr:     true,
			wantErrText: `error type "OtherError" doesn't match function return type Result<[]Order, ServiceError>`,
		},
		{
			name:       "method call matches Result error type",
			returnType: "Result<string, ServiceError>",
			errorExpr: ErrorExpr{
				ExprType:     ErrorExprMethodCall,
				ReceiverType: "ServiceError",
				InferredType: "ServiceError",
			},
			wantErr: false,
		},
		{
			name:       "method call type mismatch",
			returnType: "Result<string, ServiceError>",
			errorExpr: ErrorExpr{
				ExprType:     ErrorExprMethodCall,
				ReceiverType: "OtherError",
				InferredType: "OtherError",
			},
			wantErr:     true,
			wantErrText: `error type "OtherError" doesn't match function return type Result<string, ServiceError>`,
		},
		{
			name:       "string message always valid",
			returnType: "Result<int, ServiceError>",
			errorExpr: ErrorExpr{
				ExprType: ErrorExprString,
				Message:  "failed to process",
			},
			wantErr: false,
		},
		{
			name:       "function call skips validation",
			returnType: "Result<int, ServiceError>",
			errorExpr: ErrorExpr{
				ExprType:     ErrorExprFuncCall,
				FuncName:     "SomeFunc",
				InferredType: "", // Cannot infer
			},
			wantErr: false,
		},
		{
			name:       "interface error type accepts any",
			returnType: "Result<int, error>",
			errorExpr: ErrorExpr{
				ExprType:     ErrorExprStructLit,
				StructType:   "CustomError",
				InferredType: "CustomError",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := NewErrorPropProcessor()

			// Parse the Result type to set up function context
			resultInfo := proc.parseResultType(tt.returnType)
			if resultInfo == nil {
				t.Fatalf("failed to parse Result type: %s", tt.returnType)
			}

			proc.currentFunc = &funcContext{
				isResultType:   true,
				resultOkType:   resultInfo.okType,
				resultErrType:  resultInfo.errType,
				resultTypeName: resultInfo.typeName,
			}

			err := proc.validateErrorExprType(tt.errorExpr, 1)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrText)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestCodeGeneration tests verifying generated code for all patterns
func TestCodeGeneration(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput []string // Strings that should appear in output
		wantNotIn  []string // Strings that should NOT appear
	}{
		{
			name: "struct literal in assignment",
			input: `package main

func FindOrders(db *sql.DB) Result<[]Order, ServiceError> {
	rows := db.Query("SELECT...")? ServiceError{Code: "DB_ERROR", Message: err.Error()}
	return rows, nil
}`,
			wantOutput: []string{
				`tmp, err := db.Query("SELECT...")`,
				`// dingo:e:0`,
				`if err != nil {`,
				`return ResultSliceOrderServiceErrorErr(ServiceError{Code: "DB_ERROR", Message: err.Error()})`,
				`var rows = tmp`,
			},
		},
		{
			name: "method call in assignment",
			input: `package main

func FindOrders(db *sql.DB) Result<[]Order, ServiceError> {
	rows := db.Query("SELECT...")? ServiceError.NewDBError(err)
	return rows, nil
}`,
			wantOutput: []string{
				`tmp, err := db.Query("SELECT...")`,
				`// dingo:e:0`,
				`if err != nil {`,
				`return ResultSliceOrderServiceErrorErr(ServiceError.NewDBError(err))`,
				`var rows = tmp`,
			},
		},
		{
			name: "function call in assignment",
			input: `package main

func FindOrders(db *sql.DB) Result<[]Order, error> {
	rows := db.Query("SELECT...")? WrapDBError(err)
	return rows, nil
}`,
			wantOutput: []string{
				`tmp, err := db.Query("SELECT...")`,
				`// dingo:e:0`,
				`if err != nil {`,
				`return ResultSliceOrderErrorErr(WrapDBError(err))`,
				`var rows = tmp`,
			},
		},
		{
			name: "struct literal in return",
			input: `package main

func FindOrders(db *sql.DB) Result<[]Order, ServiceError> {
	return db.Query("SELECT...")? ServiceError{Code: "DB_ERROR", Message: err.Error()}
}`,
			wantOutput: []string{
				`tmp, err := db.Query("SELECT...")`,
				`// dingo:e:0`,
				`if err != nil {`,
				`return ResultSliceOrderServiceErrorErr(ServiceError{Code: "DB_ERROR", Message: err.Error()})`,
				`return ResultSliceOrderServiceErrorOk(tmp)`,
			},
			wantNotIn: []string{
				`var rows = tmp`, // Should not have assignment
			},
		},
		{
			name: "method call in return",
			input: `package main

func GetUser(id int) Result<*User, ServiceError> {
	return db.FindUser(id)? ServiceError.NotFound(id)
}`,
			wantOutput: []string{
				`tmp, err := db.FindUser(id)`,
				`// dingo:e:0`,
				`if err != nil {`,
				`return ResultPtrUserServiceErrorErr(ServiceError.NotFound(id))`,
				`return ResultPtrUserServiceErrorOk(tmp)`,
			},
		},
		{
			name: "string message (legacy)",
			input: `package main

func GetUser(id int) Result<*User, error> {
	return db.FindUser(id)? "user not found"
}`,
			wantOutput: []string{
				`tmp, err := db.FindUser(id)`,
				`// dingo:e:0`,
				`if err != nil {`,
				`return ResultPtrUserErrorErr(fmt.Errorf("user not found: %w", err))`,
				`return ResultPtrUserErrorOk(tmp)`,
			},
		},
		{
			name: "multiple error propagations",
			input: `package main

func ProcessData() Result<string, ServiceError> {
	data1 := ReadFile("a.txt")? ServiceError{Code: "READ_A"}
	data2 := ReadFile("b.txt")? ServiceError{Code: "READ_B"}
	return "ok", nil
}`,
			wantOutput: []string{
				`tmp, err := ReadFile("a.txt")`,
				`// dingo:e:0`,
				`return ResultStringServiceErrorErr(ServiceError{Code: "READ_A"})`,
				`var data1 = tmp`,
				`tmp1, err1 := ReadFile("b.txt")`,
				`// dingo:e:1`,
				`return ResultStringServiceErrorErr(ServiceError{Code: "READ_B"})`,
				`var data2 = tmp1`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First pass: convert Result<T,E> to Result[T,E] (like preprocessor pipeline does)
			genericProc := NewGenericSyntaxProcessor()
			preprocessed, _, err := genericProc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("GenericSyntaxProcessor failed: %v", err)
			}

			// Second pass: error propagation
			proc := NewErrorPropProcessor()
			output, _, err := proc.ProcessInternal(string(preprocessed))
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			// Check all wanted strings
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\n\nGot:\n%s", want, output)
				}
			}

			// Check no unwanted strings
			for _, notWant := range tt.wantNotIn {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q\n\nGot:\n%s", notWant, output)
				}
			}
		})
	}
}

// TestErrBinding tests verifying err variable is accessible in custom error expressions
func TestErrBinding(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput []string
	}{
		{
			name: "err in struct literal",
			input: `package main

func Test() Result<int, ServiceError> {
	x := Func()? ServiceError{Message: err.Error()}
	return x, nil
}`,
			wantOutput: []string{
				`tmp, err := Func()`,
				`ServiceError{Message: err.Error()}`,
			},
		},
		{
			name: "err in method call",
			input: `package main

func Test() Result<int, ServiceError> {
	x := Func()? ServiceError.Wrap(err)
	return x, nil
}`,
			wantOutput: []string{
				`tmp, err := Func()`,
				`ServiceError.Wrap(err)`,
			},
		},
		{
			name: "err in function call",
			input: `package main

func Test() Result<int, error> {
	x := Func()? WrapError(err, "context")
	return x, nil
}`,
			wantOutput: []string{
				`tmp, err := Func()`,
				`WrapError(err, "context")`,
			},
		},
		{
			name: "err with numbered suffix",
			input: `package main

func Test() Result<int, ServiceError> {
	x := Func1()? ServiceError{Message: err.Error()}
	y := Func2()? ServiceError{Message: err1.Error()}
	return y, nil
}`,
			wantOutput: []string{
				`tmp, err := Func1()`,
				`ServiceError{Message: err.Error()}`,
				`tmp1, err1 := Func2()`,
				`ServiceError{Message: err1.Error()}`,
			},
		},
		{
			name: "err in nested expression",
			input: `package main

func Test() Result<int, ServiceError> {
	x := Func()? ServiceError{Code: "ERR", Message: fmt.Sprintf("failed: %v", err)}
	return x, nil
}`,
			wantOutput: []string{
				`tmp, err := Func()`,
				`ServiceError{Code: "ERR", Message: fmt.Sprintf("failed: %v", err)}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First pass: convert Result<T,E> to Result[T,E] (like preprocessor pipeline does)
			genericProc := NewGenericSyntaxProcessor()
			preprocessed, _, err := genericProc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("GenericSyntaxProcessor failed: %v", err)
			}

			// Second pass: error propagation
			proc := NewErrorPropProcessor()
			output, _, err := proc.ProcessInternal(string(preprocessed))
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\n\nGot:\n%s", want, output)
				}
			}
		})
	}
}

// TestBalancedBraces tests extractBalancedBraces helper
func TestBalancedBraces(t *testing.T) {
	proc := NewErrorPropProcessor()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple fields",
			input: `Code: "ERR"}`,
			want:  `Code: "ERR"`,
		},
		{
			name:  "nested braces",
			input: `Code: "ERR", Meta: map[string]string{"key": "value"}}`,
			want:  `Code: "ERR", Meta: map[string]string{"key": "value"}`,
		},
		{
			name:  "multiple nested levels",
			input: `A: {B: {C: "value"}}}`,
			want:  `A: {B: {C: "value"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proc.extractBalancedBraces(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBalancedParens tests extractBalancedParens helper
func TestBalancedParens(t *testing.T) {
	proc := NewErrorPropProcessor()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple args",
			input: `err)`,
			want:  `err`,
		},
		{
			name:  "multiple args",
			input: `"code", err)`,
			want:  `"code", err`,
		},
		{
			name:  "nested parens",
			input: `fmt.Sprintf("failed: %v", err))`,
			want:  `fmt.Sprintf("failed: %v", err)`,
		},
		{
			name:  "multiple nested levels",
			input: `f(g(h(x))))`,
			want:  `f(g(h(x)))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proc.extractBalancedParens(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSanitizeTypeComponent tests type name sanitization
// CRITICAL FIX C2: Now uses builtin.SanitizeTypeName (authoritative implementation)
func TestSanitizeTypeComponent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "[]Order",
			want:  "SliceOrder",
		},
		{
			input: "*User",
			want:  "PtrUser",
		},
		{
			input: "map[string]int",
			want:  "Map",  // builtin simplifies map types to just "Map"
		},
		{
			input: "ServiceError",
			want:  "ServiceError",
		},
		{
			input: "error",
			want:  "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Use builtin.SanitizeTypeName (takes variadic args, pass single type)
			got := builtin.SanitizeTypeName(tt.input)
			if got != tt.want {
				t.Errorf("builtin.SanitizeTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestResultErrorPropEdgeCases tests edge cases and error conditions for Result error propagation
func TestResultErrorPropEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errText string
	}{
		{
			name: "empty error expression",
			input: `package main

func Test() Result<int, ServiceError> {
	x := Func()?
	return x, nil
}`,
			wantErr: false, // Should use bare err return
		},
		{
			name: "ternary operator not confused with error prop",
			input: `package main

func Test() int {
	x := condition ? 1 : 0
	return x
}`,
			wantErr: false, // Should not transform ternary
		},
		{
			name: "null coalesce not confused with error prop",
			input: `package main

func Test() int {
	x := value ?? 0
	return x
}`,
			wantErr: false, // Should not transform null coalesce
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := NewErrorPropProcessor()
			_, _, err := proc.ProcessInternal(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errText != "" && !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errText)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
