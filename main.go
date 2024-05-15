package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/common-nighthawk/go-figure"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"gopkg.in/yaml.v3"
)

// cache:
//   paths:
//     - node_modules/
// test:
//   image: node:20-slim
//   before_script:
//     - npm ci --legacy-peer-deps
//   script:
//     - npm run test

type Config struct {
	Cache map[string][]string `yaml:"cache"`
	Test  struct {
		Image        string   `yaml:"image"`
		BeforeScript []string `yaml:"before_script"`
		Script       []string `yaml:"script"`
	} `yaml:"test"`
}

func main() {
	myFigure := figure.NewFigure("GOTEST", "", true)
	myFigure.Print()
	fmt.Println("")
	fmt.Printf("Lightweight Testing Delivery Tools\n\n")
	fmt.Println("----------------------------------------------")
	fmt.Println("Usage: builder <config.yml> <path>")

	configYml := os.Args[1]
	path := os.Args[2]

	t := Config{}

	// open config file
	body, err := os.ReadFile(configYml)
	if err != nil {
		log.Fatalf("unable to read file: %v", err)
	}

	// unmarshal config file
	err = yaml.Unmarshal(body, &t)
	if err != nil {
		log.Fatalf("unable to unmarshal file: %v", err)
	}
	dirname, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	mkdirFolderFailed := os.MkdirAll(dirname+"/builder/cache", 0777)
	if mkdirFolderFailed != nil {
		panic(mkdirFolderFailed)
	}

	cli, err := client.NewClientWithOpts(client.WithVersion("1.43"))
	if err != nil {
		panic(err)
	}

	volumeMounted := []mount.Mount{
		{

			Type:   mount.TypeBind,
			Source: path,
			Target: "/app",
		},
	}

	// map before_script and script
	scripts := []string{}
	scripts = append(scripts, t.Test.BeforeScript...)
	scripts = append(scripts, t.Test.Script...)

	for _, path := range t.Cache["paths"] {

		mkdirFolderFailed := os.MkdirAll(dirname+"/builder/cache"+path, 0777)
		if mkdirFolderFailed != nil {
			panic(mkdirFolderFailed)
		}
		volumeMounted = append(volumeMounted, mount.Mount{
			Type:   mount.TypeBind,
			Source: dirname + "/builder/cache" + path,
			Target: path,
		})
	}

	ctx := context.Background()
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      t.Test.Image,
		Cmd:        []string{"/bin/sh", "-c", strings.Join(scripts, " && ")},
		WorkingDir: "/app",
		Tty:        false,
	}, &container.HostConfig{
		Mounts: volumeMounted,
	}, &network.NetworkingConfig{}, nil, randomString(10))
	if err != nil {
		panic(err)
	}

	fmt.Println("pulling image: ", t.Test.Image)
	if _, err := cli.ImagePull(ctx, t.Test.Image, image.PullOptions{}); err != nil {
		panic(err)
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		panic(err)
	}
	out, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, Follow: true, ShowStderr: true})
	if err != nil {
		panic(err)
	}
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

}
