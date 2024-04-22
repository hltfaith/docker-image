package model

import (
	"context"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	Client *client.Client
}

var DockerInstance *DockerClient

func init() {
	if DockerInstance == nil {
		DockerInstance = &DockerClient{}
		if err := DockerInstance.Start(); err != nil {
			log.Panic(err)
		}
	}
}

func (d *DockerClient) Start() error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	d.Client = cli
	return nil
}

func (d *DockerClient) Close() {
	_ = d.Client.Close()
}

func (d *DockerClient) ImageInspect(imageId string) (*types.ImageInspect, error) {
	resp, _, err := d.Client.ImageInspectWithRaw(context.TODO(), imageId)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *DockerClient) ImageList() ([]image.Summary, error) {
	resp, err := d.Client.ImageList(context.TODO(), types.ImageListOptions{All: true})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d *DockerClient) ImageHistory(imageId string) ([]image.HistoryResponseItem, error) {
	resp, err := d.Client.ImageHistory(context.TODO(), imageId)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
