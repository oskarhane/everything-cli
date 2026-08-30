package gates

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// snakeRe is the required shape for output field names (JSON/TOON keys and
// table columns). Wire/parse structs, config keys, and enum values are
// exempt by convention; this gate targets the PrintTable fields slices.
var snakeRe = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// fieldViolations returns snake_case violations for a fields literal.
func fieldViolations(context string, fields []string) []string {
	var violations []string
	for _, f := range fields {
		if !snakeRe.MatchString(f) {
			violations = append(violations,
				fmt.Sprintf("%s: output field %q is not snake_case", context, f))
		}
	}
	return violations
}

// stringListLit returns the string elements of a []string composite literal
// whose elements are all string literals; nil if node is anything else.
func stringListLit(node ast.Node) []string {
	lit, ok := node.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	arr, ok := lit.Type.(*ast.ArrayType)
	if !ok {
		return nil
	}
	id, ok := arr.Elt.(*ast.Ident)
	if !ok || id.Name != "string" {
		return nil
	}
	elems := make([]string, 0, len(lit.Elts))
	for _, e := range lit.Elts {
		s, ok := e.(*ast.BasicLit)
		if !ok || s.Kind != token.STRING {
			return nil
		}
		v, err := strconv.Unquote(s.Value)
		if err != nil {
			return nil
		}
		elems = append(elems, v)
	}
	return elems
}

// isFieldsName reports whether an identifier names an output-fields slice
// (calendarFields, busyFields, ...). Flag-name lists like updateFlags do not
// qualify: they are input identifiers, not output fields.
func isFieldsName(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), "fields")
}

// outputCasingViolations scans one non-test Go source for snake_case
// violations among (a) inline []string literals passed straight to
// output.PrintTable and (b) assignments/declarations of *Fields variables.
// A source scan was chosen over reflect-walking view structs because the
// fields slices are the single source of truth for output columns/keys and
// the scan needs no command execution.
func outputCasingViolations(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	pos := func(n ast.Node) string {
		return filepath.ToSlash(fset.Position(n.Pos()).String())
	}

	var violations []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ValueSpec:
			if !namesEndInFields(node.Names) {
				return true
			}
			for _, v := range node.Values {
				if fields := stringListLit(v); fields != nil {
					violations = append(violations, fieldViolations(pos(node), fields)...)
				}
			}
		case *ast.AssignStmt:
			for i, rhs := range node.Rhs {
				if i >= len(node.Lhs) {
					break
				}
				lhs, ok := node.Lhs[i].(*ast.Ident)
				if !ok || !isFieldsName(lhs.Name) {
					continue
				}
				if fields := stringListLit(rhs); fields != nil {
					violations = append(violations, fieldViolations(pos(node), fields)...)
				}
			}
		case *ast.CallExpr:
			call, ok := node.Fun.(*ast.SelectorExpr)
			if !ok || call.Sel.Name != "PrintTable" {
				return true
			}
			for _, arg := range node.Args {
				if fields := stringListLit(arg); fields != nil {
					violations = append(violations, fieldViolations(pos(node), fields)...)
				}
			}
		}
		return true
	})
	return violations, nil
}

func namesEndInFields(names []*ast.Ident) bool {
	for _, n := range names {
		if isFieldsName(n.Name) {
			return true
		}
	}
	return false
}

// TestOutputFields_AreSnakeCase scans every non-test Go source under
// internal/subcommands for PrintTable fields literals and fields variables,
// asserting snake_case. Each failure message carries file:line, the field
// name, and its context.
func TestOutputFields_AreSnakeCase(t *testing.T) {
	var violations []string
	root := subcommandsDir(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		vs, err := outputCasingViolations(path)
		if err != nil {
			return err
		}
		violations = append(violations, vs...)
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	for _, v := range violations {
		t.Error(v)
	}
}
