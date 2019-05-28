package main

import (
	"time"
	"strings"
	"log"
	"sync"
	"os"
	"os/exec"
	"bufio"
	"github.com/fsouza/go-dockerclient"
	"flag"
	"fmt"
)

const marker = "#### DOCKER HOSTS UPDATER ####"

var (
	path string
	client *docker.Client
	hosts []string
	mutex sync.Mutex
)

func init() {
	flag.StringVar(&path, "path", "/opt/hosts", "path to hosts")
	flag.Parse()

	dockerClient, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		panic(err)
	}
	client = dockerClient
}

func main() {
	update()

	listen()
}

func update() {
	hosts = make([]string, 0)

	containers, err := client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		panic(err)
	}

	added := false
	for _, container := range containers {
		container, err := client.InspectContainer(container.ID)
		if err != nil {
			log.Fatal(err)

			continue
		}

		fmt.Println(container.Name)
		add(container)
		added = true
	}

	if added {
		updateFile()
	}
}

func listen() {
	listener := make(chan *docker.APIEvents)
	err := client.AddEventListener(listener)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err = client.RemoveEventListener(listener)
		if err != nil {
			log.Fatal(err)
		}
	}()

	timeout := time.After(1 * time.Second)

	for {
		select {
		case event := <-listener:
			if "container" != event.Type {
				continue
			}

			if "start" == event.Action || "stop" == event.Action || "die" == event.Action {
				update()
			}
		case <-timeout:
			continue
		}
	}
}

func add(container *docker.Container) {
	containerHosts, err := getHosts(container)
	if err != nil || 0 == len(containerHosts) {
		return
	}

	hosts = append(hosts, containerHosts)
}

func getHosts(container *docker.Container) (string, error) {
	var hosts string

	rules := container.Config.Labels["traefik.frontend.rule"]
	if 0 < len(rules) {
		rules = rules[5:]
		for _, host := range strings.Split(rules, ",") {
			hosts += " " + host
		}
	} else {
		hosts += " " + container.Name[1:] + ".docker"
	}

	return hosts, nil
}

func updateFile() {
	mutex.Lock()
	defer mutex.Unlock()

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	lines := make([]string, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.Contains(text, marker) {
			break
		}

		lines = append(lines, text)
	}

	if err := scanner.Err(); err != nil {
	     log.Fatal("Scan file", err)
	}

	lines = append(lines, marker)

	for _, host := range hosts {
		lines = append(lines, "127.0.0.1 " + host)
	}

	err = exec.Command("sh", "-c", "echo -e \"" + strings.Join(lines, "\\n") + "\" > " + path).Run()
	if err != nil {
		log.Fatal("Write file: ", err)
	}
}
