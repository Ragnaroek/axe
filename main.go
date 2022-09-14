package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Dependency struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Arch struct {
	Services []string     `json:"services"`
	Grpc     []Dependency `json:"grpc"`
}

func main() {

	monoRepoPath, err := os.Getwd() //TODO allow overwrite via flag
	if err != nil {
		panic(err)
	}

	modName, err := readModuleName(monoRepoPath)
	if err != nil {
		panic(err)
	}

	svcPath := path.Join(monoRepoPath, "services/")
	svcs, err := findServices(svcPath)
	if err != nil {
		panic(err)
	}
	grpc, err := analyzeGrpc(svcPath, modName, svcs)
	if err != nil {
		panic(err)
	}

	arch := Arch{
		Services: svcs,
		Grpc:     grpc,
	}

	bytes, err := json.Marshal(arch)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("arch.json", bytes, os.ModePerm)
	if err != nil {
		panic(err)
	}
}

func findServices(path string) ([]string, error) {
	folders, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	svcs := make([]string, 0, len(folders))
	for _, svc := range folders {
		svcs = append(svcs, svc.Name())
	}
	return svcs, nil
}

func analyzeGrpc(svcRootPath, modName string, svcs []string) ([]Dependency, error) {

	grpcDependencies := make([]Dependency, 0)

	for _, svc := range svcs {
		svcPath := path.Join(svcRootPath, svc)
		grpcImports := make([]string, 0)
		err := filepath.WalkDir(svcPath, func(path string, fs fs.DirEntry, err error) error {
			if !fs.IsDir() && strings.HasSuffix(fs.Name(), ".go") {
				fileGrpcImports, err := checkGrpcImports(path, modName)
				if err != nil {
					return err
				}

				for _, grpcImport := range fileGrpcImports {
					if grpcImport != svc && !contains(grpcImports, grpcImport) {
						grpcImports = append(grpcImports, grpcImport)
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}

		for _, grpcImport := range grpcImports {
			grpcDependencies = append(grpcDependencies, Dependency{From: svc, To: grpcImport})
		}
	}
	return grpcDependencies, nil
}

func contains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}

// TODO adapt this to return a list of service-names (just strings) that this service talks to
func checkGrpcImports(path, modName string) ([]string, error) {
	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	imports := make([]string, 0)

	for _, imp := range ast.Imports {
		if importHasPrefix(imp, modName) && importHasSuffix(imp, "pkg/proto") { // only check module internal imports
			split := strings.Split(imp.Path.Value, "/")
			if len(split) > 3 {
				svcName := split[len(split)-3]
				imports = append(imports, svcName)
			}

		}
	}
	return imports, nil
}

func importHasPrefix(imp *ast.ImportSpec, prefix string) bool {
	// The import literal contains the " of the string, that why we have to add " to the prefix for
	// prefix checking
	return strings.HasPrefix(imp.Path.Value, "\""+prefix)
}

func importHasSuffix(imp *ast.ImportSpec, suffix string) bool {
	return strings.HasSuffix(imp.Path.Value, suffix+"\"")
}

func readModuleName(monoRepoPath string) (string, error) {
	bytes, err := os.ReadFile(path.Join(monoRepoPath, "go.mod"))
	if err != nil {
		return "", err
	}

	module, _, found := strings.Cut(string(bytes), "\n")
	if !found {
		return "", fmt.Errorf("invalid go.mod")
	}

	return strings.TrimPrefix(module, "module "), nil
}
