package resolver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apito-io/engine/utility"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
)

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

func printResponse(rd io.Reader) error {
	var lastLine string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(scanner.Text())
	}

	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return errors.New(errLine.Error)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func (s *GraphQLServer) ListDockerImages(ctx context.Context) ([]string, error) {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	dockerImages, err := cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		panic(err)
	}

	var images []string
	for _, image := range dockerImages {
		if len(image.RepoTags) > 0 {
			if strings.Contains(image.RepoTags[0], ":") {
				tag := strings.Split(image.RepoTags[0], ":")
				images = append(images, tag[0])
			}
		}
	}

	return images, nil
}

func (s *GraphQLServer) BuildADockerImage(ctx context.Context, projectId string) error {

	images, err := s.ListDockerImages(ctx)
	if err != nil {
		return err
	}

	imageName := filepath.Join("apito.io/micro", projectId)

	if utility.ArrayContains(images, imageName) {
		fmt.Println("docker image already found, skipping building")
		return nil
	}

	// Getting the current working directory
	_workingDir, _ := syscall.Getwd()
	fmt.Println("Current Dir:", _workingDir)

	_workingDir = filepath.Join(_workingDir)

	tar, err := archive.TarWithOptions(_workingDir, &archive.TarOptions{
		IncludeFiles: []string{
			filepath.Join("cache", "schema", projectId),
			filepath.Join("cache", "server"),
			//"Dockerfile.micro", transferred to template gen
			"go.mod",
			"go.sum",
			//"go.mod",
			filepath.Join("executor", "executor-linux.so"),
		},
	})
	if err != nil {
		return err
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	opts := types.ImageBuildOptions{
		Dockerfile: filepath.Join("cache", "schema", projectId, "Dockerfile"),
		Tags:       []string{imageName},
		Remove:     true,
	}
	res, err := cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	err = printResponse(res.Body)
	if err != nil {
		return err
	}

	return nil

}

func (s *GraphQLServer) BuildAContainer(ctx context.Context, projectId string, port string) error {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	imageName := filepath.Join("apito.io/micro", projectId)

	// Use nat.NewPort to format containerPortStr into nat.Port
	containerPortNat, err := nat.NewPort("tcp", port)
	if err != nil {
		return err
	}

	exposedPorts := port + "/tcp"
	_, err = cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		ExposedPorts: nat.PortSet{
			nat.Port(exposedPorts): struct{}{}, // Expose port 1234 over TCP
		},
	}, &container.HostConfig{
		PortBindings: map[nat.Port][]nat.PortBinding{
			containerPortNat: []nat.PortBinding{
				{
					//HostIP:   "0.0.0.0",
					HostPort: port,
				},
			},
		},
		//NetworkMode: "host",
	}, nil, nil, projectId)
	if err != nil {
		return err
	}

	return nil
}

func (s *GraphQLServer) RunContainerInBackground(ctx context.Context, projectId string) error {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	if err := cli.ContainerStart(ctx, projectId, container.StartOptions{}); err != nil {
		return err
	}

	return nil
}

func (s *GraphQLServer) ListRunningContainers(ctx context.Context, listAll bool, state string) ([]types.Container, error) {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	dockerContainers, err := cli.ContainerList(ctx, container.ListOptions{
		All: listAll,
	})
	if err != nil {
		return nil, err
	}

	return dockerContainers, nil
}

func (s *GraphQLServer) FilterRunningContainers(ctx context.Context, projectId string) (*types.Container, error) {

	dockerContainers, err := s.ListRunningContainers(ctx, true, "running")
	if err != nil {
		return nil, err
	}

	for _, _container := range dockerContainers {
		if len(_container.Names) > 0 {
			if strings.HasPrefix(_container.Names[0], "/") {
				tag := strings.TrimPrefix(_container.Names[0], "/")
				if tag == projectId {
					return &_container, nil
				}
			}
		}
	}

	return nil, errors.New("no running micro service found for this request")
}

func (s *GraphQLServer) RestartContainer(ctx context.Context, containerId string) error {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	//timeout := 1 * time.Second

	err = cli.ContainerRestart(ctx, containerId, container.StopOptions{
		//Signal:  "",
		//Timeout: timeout,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *GraphQLServer) GetMicroServicePort(ctx context.Context, id string) (string, error) {
	key := fmt.Sprintf(`%s_port`, id)
	driver, err := s.GraphQLExecutor.GetSharedDBDriver(ctx)
	if err != nil {
		return "", err
	}
	data, err := driver.Get(key)
	if err != nil {
		if err.Error() == "key not found" { // if port not found
			port := UnixPortGeneration() // generate
			err = driver.Set(key, port)  // save it
			if err != nil {
				return "", err
			}
			return port, nil
		} else {
			return "", err
		}
	}
	return data.(string), nil
}

func (s *GraphQLServer) SavePID(ctx context.Context, projectId, pid string) error {
	key := fmt.Sprintf(`%s_pid`, projectId)
	driver, err := s.GraphQLExecutor.GetSharedDBDriver(ctx)
	if err != nil {
		return err
	}
	err = driver.Set(key, pid)
	if err != nil {
		return err
	}
	return nil
}

func (s *GraphQLServer) GetPID(ctx context.Context, projectId string) (string, error) {
	key := fmt.Sprintf(`%s_pid`, projectId)
	driver, err := s.GraphQLExecutor.GetSharedDBDriver(ctx)
	if err != nil {
		return "", err
	}
	data, err := driver.Get(key)
	if err != nil {
		return "", err
	}
	return data.(string), nil
}

func UnixPortGeneration() string {
	l, err := net.Listen("tcp", "0.0.0.0:0") // :0 means find a port that is available in the system
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return fmt.Sprintf(`%d`, l.Addr().(*net.TCPAddr).Port)
}
