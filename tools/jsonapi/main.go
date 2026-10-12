// Command jsonapi rejects first-party encoding/json operations while allowing
// the Number and RawMessage types required by existing interoperability contracts.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

func inspectSource(filename string, source any) ([]string, error) {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, filename, source, parser.AllErrors)
	if err != nil {
		return nil, err
	}
	aliases := map[string]bool{}
	var failures []string
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != "encoding/json" {
			continue
		}
		alias := "json"
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if alias == "." {
			failures = append(failures, fmt.Sprintf("%s: dot import of encoding/json hides API usage", positions.Position(imported.Pos())))
		}
		aliases[alias] = true
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selection, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name, ok := selection.X.(*ast.Ident)
		// Parsed local bindings have an Obj; imported package names do not.
		if !ok || name.Obj != nil || !aliases[name.Name] {
			return true
		}
		if selection.Sel.Name != "Number" && selection.Sel.Name != "RawMessage" {
			failures = append(failures, fmt.Sprintf("%s: encoding/json.%s must use encoding/json/v2 or jsontext", positions.Position(selection.Pos()), selection.Sel.Name))
		}
		return true
	})
	return failures, nil
}

func inspectTree(root string) ([]string, error) {
	var failures []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "dist", "node_modules", ".uv-cache", ".venv", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		found, err := inspectSource(path, nil)
		if err != nil {
			return err
		}
		failures = append(failures, found...)
		return nil
	})
	return failures, err
}

func main() {
	root := flag.String("root", ".", "first-party repository root to inspect")
	flag.Parse()
	failures, err := inspectTree(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, failure := range failures {
		fmt.Fprintln(os.Stderr, failure)
	}
	if len(failures) != 0 {
		os.Exit(1)
	}
	fmt.Println("First-party JSON operations use v2/jsontext; only Number/RawMessage interoperability types are permitted from v1.")
}
