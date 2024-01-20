package main

import (
	"context"
	"fmt"
	"os"

	"dagger.io/dagger"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx := context.Background()
	client, _ := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	defer client.Close()

	// create cache volumes for docker certificates and docker images, containers and local volumes
	certCache := client.CacheVolume("node")
	dockerState := client.CacheVolume("docker-state")

	// create the container with Dind with the docker daemon we will be using
	docker, _ := client.Container().
		From("docker:dind").
		WithExposedPort(2376).
		WithMountedCache("/var/lib/docker", dockerState, dagger.ContainerWithMountedCacheOpts{
			Sharing: dagger.Private,
		}).
		WithMountedCache("/certs", certCache).
		WithExec(nil, dagger.ContainerWithExecOpts{
			InsecureRootCapabilities: true,
		}).
		AsService().
		Start(ctx)

	// Preparing the runner container
	runner := client.Container().
		From("docker:latest").
		// The certificates generated in the DinD container is valid for
		// the hostname "docker", so we will need to use that.
		WithServiceBinding("docker", docker).
		WithMountedCache("/certs", certCache).
		WithEnvVariable("DOCKER_HOST", "tcp://docker:2376").
		WithEnvVariable("DOCKER_TLS_CERTDIR", "/certs").
		WithEnvVariable("DOCKER_CERT_PATH", "/certs/client").
		WithEnvVariable("DOCKER_TLS_VERIFY", "1")

	group := errgroup.Group{}

	// Container example 1
	// Run first container that pulls busybox.
	group.Go(func() error {
		_, _ = runner.
			WithExec([]string{"docker", "pull", "busybox"}).
			Sync(ctx)
		return nil
	})

	// Container example 2
	// Running second container that pulls alpine
	group.Go(func() error {
		_, _ = runner.
			WithExec([]string{"docker", "pull", "alpine"}).
			Sync(ctx)
		return nil
	})

	// Wait for all goroutines to complete.
	// The two first steps run in parallel.
	if err := group.Wait(); err != nil {
		fmt.Printf("errgroup tasks ended up with an error: %v\n", err)
	} else {
		fmt.Println("all works done successfully")
	}

	// Container example 3
	// Running third container that shows alpine and busybox images
	// fetched by the remote docker daemon container
	_, _ = runner.
		WithExec([]string{"docker", "images"}).
		Sync(ctx)
}
