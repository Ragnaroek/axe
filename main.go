package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
)

type Arch struct {
	Services []string `json:"services"`
}

func main() {

	monoRepoPath, err := os.Getwd() //TODO allow overwrite via flag
	if err != nil {
		panic(err)
	}

	svcs, err := os.ReadDir(path.Join(monoRepoPath, "services/"))
	if err != nil {
		panic(err)
	}

	arch := Arch{
		Services: []string{},
	}

	for _, svc := range svcs {
		arch.Services = append(arch.Services, svc.Name())
	}

	bytes, err := json.Marshal(arch)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile("arch.json", bytes, os.ModePerm)
	if err != nil {
		panic(err)
	}
	// TODO Extract services from monorepo and write them to a json file
	// TODO Visualise services in archv online tool!
	// TODO Extract grpc relations between services (from import) and write them to a json file
}
