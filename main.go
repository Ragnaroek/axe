package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
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

	svcs, err := findServices(monoRepoPath)
	if err != nil {
		panic(err)
	}

	grpc, err := analyzeGrpc(monoRepoPath, modName, svcs)
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

func findServices(repoPath string) ([]string, error) {
	out, err := exec.Command("find", repoPath, "-name", "Buildfile",
		"-not", "-path", "*/node_modules/*", "-not", "-path",
		"*/infrastructure/*",
		"-not", "-path", "*/.git/*").Output()
	if err != nil {
		return nil, fmt.Errorf("cannot find services to run coverage on: %w", err)
	}
	buildfiles := strings.Split(string(out), "\n")
	services := make([]string, 0, len(buildfiles))
	for _, buildfile := range buildfiles {
		if buildfile != "" {
			svc := strings.TrimPrefix(path.Dir(buildfile), repoPath+"/")
			services = append(services, svc)
		}
	}
	return services, nil
}

func analyzeGrpc(repoPath string, modName string, svcs []string) ([]Dependency, error) {

	grpcDependencies := make([]Dependency, 0)

	for _, svc := range svcs {
		grpcImports := make([]string, 0)
		svcPath := path.Join(repoPath, svc)
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

func checkGrpcImports(filePath, modName string) ([]string, error) {
	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	imports := make([]string, 0)

	for _, imp := range ast.Imports {
		if importHasPrefix(imp, modName) { // only check module internal imports
			split := strings.Split(imp.Path.Value, "/")
			if importHasSuffix(imp, "proto") {
				if len(split) > 3 {
					svcName := split[len(split)-2]
					groupName := split[len(split)-3]
					imports = append(imports, fmt.Sprintf("%s/%s", groupName, svcName))
				}
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
