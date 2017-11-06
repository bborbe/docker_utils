package main

import (
	"fmt"
	docker_utils_factory "github.com/bborbe/docker_utils/factory"
	"github.com/bborbe/docker_utils/model"
	flag "github.com/bborbe/flagenv"
	"github.com/golang/glog"
	"io"
	"os"
	"runtime"
)



var (
	registryPtr     = flag.String(model.ParameterRegistry, "", "Registry")
	usernamePtr     = flag.String(model.ParameterUsername, "", "Username")
	passwordPtr     = flag.String(model.ParameterPassword, "", "Password")
	passwordFilePtr = flag.String(model.ParameterPasswordFile, "", "Password-File")
	credentialsfromfilePtr = flag.Bool(model.ParameterCredentialsFromDockerConfig, false, "Read Username and Password from ~/.docker/config.json")
)

func main() {
	defer glog.Flush()
	glog.CopyStandardLogTo("info")
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	writer := os.Stdout
	if err := do(writer); err != nil {
		glog.Exit(err)
	}
}

func do(writer io.Writer) error {
	var err error
	password := model.RegistryPassword(*passwordPtr)
	if len(*passwordFilePtr) > 0 {
		password, err = model.RegistryPasswordFromFile(*passwordFilePtr)
		if err != nil {
			return err
		}
	}
	registry := model.Registry{
		Name:     model.RegistryName(*registryPtr),
		Username: model.RegistryUsername(*usernamePtr),
		Password: password,
	}
	if *credentialsfromfilePtr {
		if err := registry.ReadCredentialsFromDockerConfig(); err != nil {
			return fmt.Errorf("read credentials failed: %v", err)
		}
	}
	glog.V(2).Infof("use registry %v", registry)
	if err := registry.Validate(); err != nil {
		return fmt.Errorf("validate registry failed: %v", err)
	}
	factory := docker_utils_factory.New()
	repositories, err := factory.Repositories().List(registry)
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		fmt.Fprintf(writer, "%s\n", repository.String())
	}
	return nil
}
