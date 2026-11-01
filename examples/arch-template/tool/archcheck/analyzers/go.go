// Official parser adapter: syntax facts, not a second rule engine.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"
)

type InputFile struct {
	Path string `json:"path"`
	Text string `json:"text"`
}
type Input struct {
	Files []InputFile `json:"files"`
}
type Import struct {
	Specifier       string `json:"specifier"`
	Line            int    `json:"line"`
	StandardLibrary bool   `json:"standard_library"`
}
type Symbol struct {
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
}
type File struct {
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Unit     string   `json:"unit"`
	Imports  []Import `json:"imports"`
	Symbols  []Symbol `json:"symbols"`
}

func main() {
	var input Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	files := []File{}
	errors := []string{}
	for _, source := range input.Files {
		positions := token.NewFileSet()
		parsed, err := parser.ParseFile(positions, source.Path, source.Text, parser.AllErrors)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		file := File{Path: source.Path, Language: "go", Unit: "go:" + path.Dir(source.Path), Imports: []Import{}, Symbols: []Symbol{}}
		for _, item := range parsed.Imports {
			spec, _ := strconv.Unquote(item.Path.Value)
			pkg, err := build.Default.Import(spec, "", build.FindOnly)
			standard := err == nil && pkg.Goroot
			file.Imports = append(file.Imports, Import{spec, positions.Position(item.Pos()).Line, standard})
		}
		for _, item := range parsed.Decls {
			if function, ok := item.(*ast.FuncDecl); ok {
				file.Symbols = append(file.Symbols, Symbol{function.Name.Name, positions.Position(function.Pos()).Line, positions.Position(function.End()).Line})
			}
			if declaration, ok := item.(*ast.GenDecl); ok {
				for _, spec := range declaration.Specs {
					if t, ok := spec.(*ast.TypeSpec); ok {
						file.Symbols = append(file.Symbols, Symbol{t.Name.Name, positions.Position(t.Pos()).Line, positions.Position(t.End()).Line})
					}
				}
			}
		}
		files = append(files, file)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"files": files, "errors": errors}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
