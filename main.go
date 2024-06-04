package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin"
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

var (
	url         = kingpin.Arg("config", "Test Configuration").Required().String()
	projectPath = kingpin.Flag("project_path", "Project Path").Short('p').Default("").String()
	projectKey  = kingpin.Flag("project_key", "Project key").Short('k').Default("").String()
	scanSonar   = kingpin.Flag("send_sonar", "read only one message").Short('s').Default("false").Bool()
)

func fail(msg string, o ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, o...)
	os.Exit(1)
}

type Config struct {
	Cache map[string][]string `yaml:"cache"`
	Test  struct {
		Image        string   `yaml:"image"`
		BeforeScript []string `yaml:"before_script"`
		Script       []string `yaml:"script"`
	} `yaml:"test"`
}

func main() {

	kingpin.UsageTemplate(kingpin.CompactUsageTemplate).Version("0.1.0")
	kingpin.Parse()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	myFigure := figure.NewFigure("GOTEST", "", true)
	myFigure.Print()
	fmt.Println("")
	fmt.Printf("Lightweight Testing Delivery Tools\n\n")
	fmt.Println("----------------------------------------------")
	fmt.Println("Usage: builder <config.yml> <path>")

	configYml := url
	var path string
	if projectPath == nil || *projectPath == "" {
		// get Current path
		currentPath, _ := os.Getwd()
		path = currentPath
	} else {
		path = *projectPath
	}

	fmt.Println("Config File: ", *configYml)
	fmt.Println("Project Path: ", path)
	fmt.Println("Sonar Scan: ", *scanSonar)
	fmt.Println("----------------------------------------------")
	t := Config{}

	// open config file
	body, err := os.ReadFile(*configYml) // Dereference the configYml pointer
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

	fmt.Println("Starting Job With  Docker Image : ", t.Test.Image)
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

	sonarImage := "sonarsource/sonar-scanner-cli"
	if *scanSonar {

		sonarVolumeMounted := []mount.Mount{
			{

				Type:   mount.TypeBind,
				Source: path,
				Target: "/usr/src",
			},
		}

		fmt.Println("Scanning Sonar")
		// Run Sonar Via docker
		PROJECT_KEY := ""
		if *projectKey == "" {

			// get current folder name
			PROJECT_KEY = filepath.Base(path)
		} else {
			PROJECT_KEY = *projectKey
		}

		sonarOpts := []string{
			"-Dsonar.projectKey=" + PROJECT_KEY + "-new",
			"-Dsonar.javascript.lcov.reportPaths=coverage/lcov.info",
			"-Dsonar.typescript.tsconfigPaths=tsconfig.sonar.json",

			"-Dsonar.java.binaries=**/*",
		}
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image: sonarImage,
			Tty:   false,
			Env: []string{
				"SONAR_HOST_URL=https://sonar.javan.co.id",
				"SONAR_TOKEN=squ_940b44dfc1230d6e687fea00bfac901803e54c4d",
				"SONAR_SCANNER_OPTS=" + strings.Join(sonarOpts, " "),
			},
		}, &container.HostConfig{
			Mounts: sonarVolumeMounted,
		}, &network.NetworkingConfig{}, nil, "sonarscan-"+randomString(10))

		if err != nil {
			panic(err)
		}

		fmt.Println("Starting Job With  Docker Image : ", sonarImage)
		if _, err := cli.ImagePull(ctx, sonarImage, image.PullOptions{}); err != nil {
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

}
