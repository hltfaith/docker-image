package service

import (
	"crypto/sha256"
	"docker-image/model"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync/atomic"

	"github.com/docker/docker/api/types/image"
)

var getAllImagesData *GetImagesInfo

type ImagesInfoInterface interface {
	// 从缓存中拿去image信息
	Load() []*ImageInfo

	// 存储到缓存
	store()

	// 初始化时获取所有image信息
	getAllImagesInfo() ([]*ImageInfo, error)

	// 镜像内容寻址
	imageContentAddress(diffIds []string) (*[]*ImageLayerID, error)

	// 根据镜像层id获取镜像信息
	ImageInfoFromLayerId(layerId string) []*ImageInfo

	// 根据镜像id获取镜像信息
	ImageInfoFromImageId(imageId string) *ImageInfo

	// 根据镜像id获取none标记镜像信息
	ImageInfoFromNoneImageId(imageId string) *ImageInfo
}

type GetImagesInfo struct {
	cache atomic.Value
}

type ImageInfo struct {
	ImageNameData
	ImageID       string                      `json:"image_id"`        // 镜像ID
	ImageLayerIDS []*ImageLayerID             `json:"image_layer_ids"` // 镜像内容寻址
	ImagesHistory []image.HistoryResponseItem `json:"images_history"`  // 镜像存储位置信息
}

type ImageLayerID struct {
	DiffID  string `json:"diff_id"`
	ChainID string `json:"chain_id"`
	CacheID string `json:"cache_id"`
}

func GetAllImagesInstance() *GetImagesInfo {
	if getAllImagesData == nil {
		getAllImagesData = &GetImagesInfo{}
		getAllImagesData.store()
	}
	return getAllImagesData
}

func init() {
	GetAllImagesInstance()
}

func (i *GetImagesInfo) Load() []*ImageInfo {
	c := i.cache.Load()
	if c == nil {
		panic("ImageInfo cache is nil")
	}
	dataMap, ok := c.(map[string][]*ImageInfo)
	if !ok || dataMap == nil {
		return nil
	}
	return dataMap["ImagesInfo"]
}

func (i *GetImagesInfo) store() {
	imagesMap := make(map[string][]*ImageInfo, 0)
	imagesInfo, err := i.getAllImagesInfo()
	if err != nil {
		log.Panicln(err)
	}
	imagesMap["ImagesInfo"] = imagesInfo
	i.cache.Store(imagesMap)
}

func (i *GetImagesInfo) getAllImagesInfo() ([]*ImageInfo, error) {
	imageList, err := model.DockerInstance.ImageList()
	if err != nil {
		return nil, err
	}
	imagesInfo := make([]*ImageInfo, 0)
	for _, image := range imageList {
		imagesInspect, err := model.DockerInstance.ImageInspect(image.ID)
		if err != nil {
			return nil, err
		}
		imagesHistory, err := model.DockerInstance.ImageHistory(image.ID)
		if err != nil {
			return nil, err
		}
		imagelayerIds, err := i.imageContentAddress(imagesInspect.RootFS.Layers)
		if err != nil {
			return nil, err
		}
		// fix: <none>:<none>
		if len(image.RepoTags) == 0 && len(image.RepoDigests) == 0 {
			imagesInfo = append(imagesInfo, &ImageInfo{
				ImageNameData: ImageNameData{
					ImageName: "<none>",
					ImageTag:  "<none>",
				},
				ImageLayerIDS: *imagelayerIds,
				ImageID:       image.ID,
				ImagesHistory: imagesHistory,
			})
			continue
		}
		// fix: 镜像:TAG   kubeovn/kube-ovn     <none>
		if len(image.RepoTags) == 0 && len(image.RepoDigests) != 0 {
			imagesInfo = append(imagesInfo, &ImageInfo{
				ImageNameData: ImageNameData{
					ImageName: strings.Split(image.RepoDigests[0], "@")[0],
					ImageTag:  "<none>",
				},
				ImageLayerIDS: *imagelayerIds,
				ImageID:       image.ID,
				ImagesHistory: imagesHistory,
			})
			continue
		}
		for _, imageTag := range image.RepoTags {
			split := strings.Split(imageTag, ":")
			if len(split) < 2 {
				continue
			}
			imagesInfo = append(imagesInfo, &ImageInfo{
				ImageNameData: ImageNameData{
					ImageName: split[0],
					ImageTag:  split[1],
				},
				ImageLayerIDS: *imagelayerIds,
				ImageID:       image.ID,
				ImagesHistory: imagesHistory,
			})
		}
	}
	return imagesInfo, nil
}

func (i *GetImagesInfo) imageContentAddress(diffIds []string) (*[]*ImageLayerID, error) {
	// 内容寻址
	imageLayerID := []*ImageLayerID{}
	firstLayer := true
	chainID := ""
	for _, diffID := range diffIds {
		if firstLayer { // 首层
			b, err := ioutil.ReadFile("/var/lib/docker/image/overlay2/layerdb/" + strings.ReplaceAll(diffID, ":", "/") + "/cache-id")
			if err != nil {
				log.Fatal(err)
				return nil, err
			}
			imageLayerID = append(imageLayerID, &ImageLayerID{
				DiffID:  strings.ReplaceAll(diffID, "sha256:", ""),
				ChainID: strings.ReplaceAll(diffID, "sha256:", ""),
				CacheID: string(b),
			})
			chainID = diffID
			firstLayer = false
			continue
		}
		// 上层
		enc := chainID + " " + diffID
		chain := fmt.Sprintf("%x", sha256.Sum256([]byte(enc)))
		b, err := ioutil.ReadFile("/var/lib/docker/image/overlay2/layerdb/" + "sha256/" + chain + "/cache-id")
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		imageLayerID = append(imageLayerID, &ImageLayerID{
			DiffID:  strings.ReplaceAll(diffID, "sha256:", ""),
			ChainID: chain,
			CacheID: string(b),
		})
		chainID = "sha256:" + chain
	}
	return &imageLayerID, nil
}

func (i *GetImagesInfo) ImageInfoFromLayerId(layerId string) []*ImageInfo {
	imagesInfo := make([]*ImageInfo, 0)
	for _, image := range i.Load() {
		for _, layer := range image.ImageLayerIDS {
			if layer.CacheID == layerId || layer.ChainID == layerId || layer.DiffID == layerId {
				imagesInfo = append(imagesInfo, image)
			}
		}
	}
	return imagesInfo
}

func (i *GetImagesInfo) ImageInfoFromImageId(imageId string) *ImageInfo {
	if imageId == "" {
		return &ImageInfo{}
	}
	imageid := strings.ReplaceAll(imageId, "sha256:", "")
	images := strings.Split(imageid, ":")
	imageTag := false
	if len(images) == 2 {
		imageTag = true
	}
	for _, image := range i.Load() {
		if imageTag {
			if image.ImageName == images[0] && image.ImageTag == images[1] {
				return image
			}
			continue
		}
		if strings.Contains(image.ImageID, imageid) {
			return image
		}
	}
	return &ImageInfo{}
}

func (i *GetImagesInfo) ImageInfoFromNoneImageId(imageId string) *ImageInfo {
	imagesInfo := &ImageInfo{}
	for _, image := range i.Load() {
		if strings.Contains(image.ImageID, imageId) {
			if image.ImageName == "<none>" || image.ImageTag == "<none>" {
				imagesInfo = image
			}
		}
	}
	return imagesInfo
}
