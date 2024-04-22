package service

import (
	"crypto/md5"
	"docker-image/util"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type HistoryImageStorageData struct {
	Image       string `json:"image"`        // 镜像ID
	Created     string `json:"created"`      // 创建时间
	CreatedBy   string `json:"created_by"`   // 创建的来源
	Size        string `json:"size"`         // 层大小
	IsLayer     string `json:"is_layer"`     // 是否是镜像层 image layer | empty layer
	StoragePath string `json:"storage_path"` // 存储路径
}

type ContainsNoneLayerData struct {
	ImageNameData
	ImageID string `json:"image_id"` // 镜像ID
	Layers  int    `json:"layers"`   // 镜像层数
}

type ContainsBinaryData struct {
	ImageNameData
	ImageID  string `json:"image_id"`  // 镜像ID
	FilePath string `json:"file_path"` // 文件绝对路径
}

type ContainsImageData struct {
	ImageNameData
	ImageID string `json:"image_id"` // 镜像ID
}

type ImageLayerContentData struct {
	ImageLayerID
	Content string `json:"content"` // 显示目录及文件名
	Size    string `json:"size"`    // 单位bit
}

type ImageNameData struct {
	ImageName string `json:"image_name"` // 镜像名称
	ImageTag  string `json:"image_tag"`  // 镜像tag
}

func (i ImageRelation) ImageFileStorageLocation() {
	// docker history image = /var/lib/docker/image/overlay2/imagedb/content/sha256
	// docker history image 为镜像id, 默认是本地已有镜像的最后一层
	// 反查匹配关系
	// 倒数第一层反向查找 <missing> 标记的层, 及本层与 diff_ids 进行匹配
	// 除<missing>外的层, 进行查找已有镜像层查找与 diff_ids 进行匹配
	imageContentPath := "/var/lib/docker/overlay2/"
	imageInfo := GetAllImagesInstance().ImageInfoFromImageId(i.ImageId)
	historyImageStorageDatas := make([]*HistoryImageStorageData, 0)
	missingNum := 0
	imageLayer, emptyLayer := "image layer", "empty layer"
	missing := "<missing>"
	createdBySize := 0
	// 倒序
	for i := len(imageInfo.ImagesHistory) - 1; i >= 0; i-- {
		image := imageInfo.ImagesHistory[i]
		if image.ID == missing {
			isLayer := imageLayer
			if image.Size == 0 { // empty layer
				isLayer = emptyLayer
			} else {
				missingNum += 1
			}
			historyImageStorageDatas = append(historyImageStorageDatas, &HistoryImageStorageData{
				Image:     image.ID,
				Created:   util.CreatedSince(image.Created),
				CreatedBy: image.CreatedBy,
				Size:      util.ImageSize(image.Size),
				IsLayer:   isLayer,
			})
			if len(image.CreatedBy) > createdBySize {
				createdBySize = len(image.CreatedBy)
			}
			continue
		}
		if !strings.ContainsAny(image.ID, ":") {
			continue
		}
		v := GetAllImagesInstance().ImageInfoFromImageId(image.ID)
		iLayer := v.ImageLayerIDS[len(v.ImageLayerIDS)-1]
		storagePath := filepath.Join(imageContentPath, iLayer.CacheID)
		isLayer := imageLayer
		if image.Size == 0 { // empty layer
			storagePath = ""
			isLayer = emptyLayer
		}
		if i == 0 { // 最后一层
			// 补充<missing>层storagePath
			// TODO 需要对比<missing> 与 rootfs 的差异, 哪些层是空层
			missingLayers := make([]string, 0)
			missingImageInfo := GetAllImagesInstance().ImageInfoFromImageId(image.ID)
			for missingLayer := 0; missingLayer <= (missingNum - 1); missingLayer++ {
				iLayer := missingImageInfo.ImageLayerIDS[missingLayer]
				missingLayers = append(missingLayers, filepath.Join(imageContentPath, iLayer.CacheID))
			}
			index := 0
			for _, v := range historyImageStorageDatas {
				if v.Image == missing && v.IsLayer == imageLayer {
					if index >= len(historyImageStorageDatas) {
						log.Panicln("<missing> index out of range")
					}
					v.StoragePath = missingLayers[index]
					index += 1
				}
				if v.Image == missing && v.IsLayer == emptyLayer {
					v.StoragePath = ""
				}
			}
		}
		historyImageStorageDatas = append(historyImageStorageDatas, &HistoryImageStorageData{
			Image:       strings.ReplaceAll(image.ID, "sha256:", "")[:12],
			Created:     util.CreatedSince(image.Created),
			CreatedBy:   image.CreatedBy,
			Size:        util.ImageSize(image.Size),
			IsLayer:     isLayer,
			StoragePath: storagePath,
		})
		if len(image.CreatedBy) > createdBySize {
			createdBySize = len(image.CreatedBy)
		}
	}
	outputImageStorageLocation(historyImageStorageDatas, strconv.Itoa(createdBySize))
}

func (i ImageRelation) ContainsNoneLayerImage() {
	imageInfo := GetAllImagesInstance().ImageInfoFromNoneImageId(i.ImageId)
	var layerIds []string
	for _, id := range imageInfo.ImageLayerIDS {
		layerIds = append(layerIds, id.DiffID)
	}
	maxLayers := 0
	noneLayersData := make([]*ContainsNoneLayerData, 0)
	for _, image := range GetAllImagesInstance().Load() {
		if image.ImageName == "<none>" || image.ImageTag == "<none>" {
			continue
		}
		var iDiffids []string
		for _, id := range image.ImageLayerIDS {
			iDiffids = append(iDiffids, id.DiffID)
		}
		iDiffids = append(iDiffids, layerIds...)
		layer := util.FindDuplicates(iDiffids)
		if layer > maxLayers {
			maxLayers = layer
		}
		noneLayersData = append(noneLayersData, &ContainsNoneLayerData{
			ImageNameData: ImageNameData{
				ImageName: image.ImageName,
				ImageTag:  image.ImageTag,
			},
			ImageID: image.ImageID,
			Layers:  layer, // 重复率
		})
	}
	// 最接近镜像层数
	repoSize := 0
	tagSize := 0
	temp := make([]*ContainsNoneLayerData, 0)
	for _, data := range noneLayersData {
		if data.Layers == maxLayers {
			data.ImageID = strings.ReplaceAll(data.ImageID, "sha256:", "")[:12]
			temp = append(temp, data)
			if len(data.ImageName) > repoSize {
				repoSize = len(data.ImageName)
			}
			if len(data.ImageTag) > tagSize {
				tagSize = len(data.ImageTag)
			}
		}
	}
	noneLayersData = temp
	outputContainsNoneLayer(noneLayersData, strconv.Itoa(repoSize), strconv.Itoa(tagSize))
}

func (i ImageRelation) ContainsBinaryfile() {
	fileStat, err := os.Stat(i.ImageFile)
	if err != nil {
		log.Panicln(err)
	}
	body, err := os.ReadFile(i.ImageFile)
	if err != nil {
		log.Panicln(err)
	}
	filemd5 := fmt.Sprintf("%x", md5.Sum(body))
	// TODO: 遍历优化
	var paths []string
	dirsEntry, _ := os.ReadDir("/var/lib/docker/overlay2/")
	dirsNameChan := make(chan string, len(dirsEntry))
	for _, fd := range dirsEntry {
		if fd.IsDir() && fd.Name() != "l" {
			dirsNameChan <- fd.Name()
		}
	}
	close(dirsNameChan)
	tmppathsChan := make(chan string, len(dirsEntry))
	// 协程遍历处理
	var w sync.WaitGroup
	w.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer w.Done()
			for path := range dirsNameChan {
				err := filepath.Walk("/var/lib/docker/overlay2/"+path, func(subPath string, info os.FileInfo, err error) error {
					if err != nil {
						panic(err)
					}
					if !info.IsDir() {
						if info.Size() == fileStat.Size() {
							tmppathsChan <- subPath
						}
					}
					return err
				})
				if err != nil {
					panic(err)
				}
			}
		}()
	}
	w.Wait()
	close(tmppathsChan)
	for path := range tmppathsChan {
		paths = append(paths, path)
	}

	containsBinaryDatas := make([]*ContainsBinaryData, 0)
	repoSize := 0
	tagSize := 0

	var wg sync.WaitGroup
	var m sync.Mutex
	for _, path := range paths {
		wg.Add(1)
		go func(path string) {
			defer func() {
				wg.Done()
			}()
			body, err := os.ReadFile(path)
			if err != nil {
				log.Println(err)
				return
			}
			if fmt.Sprintf("%x", md5.Sum(body)) != filemd5 {
				return
			}
			pathSplit := strings.Split(path, "/")
			if len(pathSplit) <= 7 {
				return
			}
			for _, info := range GetAllImagesInstance().ImageInfoFromLayerId(pathSplit[5]) {
				containsBinaryDatas = append(containsBinaryDatas, &ContainsBinaryData{
					ImageNameData: ImageNameData{
						ImageName: info.ImageName,
						ImageTag:  info.ImageTag,
					},
					ImageID:  strings.ReplaceAll(info.ImageID, "sha256:", "")[:12],
					FilePath: path,
				})
				m.Lock()
				if len(info.ImageName) > repoSize {
					repoSize = len(info.ImageName)
				}
				if len(info.ImageTag) > tagSize {
					tagSize = len(info.ImageTag)
				}
				m.Unlock()
			}
		}(path)
	}
	wg.Wait()
	outputContainsBinary(containsBinaryDatas, strconv.Itoa(repoSize), strconv.Itoa(tagSize))
}

func (i ImageRelation) ContainsImageLayerID() {
	layerid := strings.ReplaceAll(i.ImageId, "sha256:", "")
	imageList := GetAllImagesInstance().ImageInfoFromLayerId(layerid)
	containsImagesData := []*ContainsImageData{}
	repoSize := 0
	tagSize := 0
	for _, image := range imageList {
		containsImagesData = append(containsImagesData, &ContainsImageData{
			ImageID: strings.ReplaceAll(image.ImageID, "sha256:", "")[:12],
			ImageNameData: ImageNameData{
				ImageName: image.ImageName,
				ImageTag:  image.ImageTag,
			},
		})
		if len(image.ImageName) > repoSize {
			repoSize = len(image.ImageName)
		}
		if len(image.ImageTag) > tagSize {
			tagSize = len(image.ImageTag)
		}
	}
	outputContainsImage(containsImagesData, strconv.Itoa(repoSize), strconv.Itoa(tagSize))
}

func (i ImageRelation) ImageLayerContent() {
	image := GetAllImagesInstance().ImageInfoFromImageId(i.ImageId)
	if image == nil {
		log.Fatalln("Not found docker image")
	}
	imageLayerContentData := []*ImageLayerContentData{}
	c_size := 0
	for _, id := range image.ImageLayerIDS {
		entries, err := ioutil.ReadDir("/var/lib/docker/overlay2/" + id.CacheID + "/diff")
		if err != nil {
			log.Fatal(err)
		}
		content := ""
		for _, entry := range entries {
			content = content + entry.Name() + " "
		}
		if len(content) > c_size {
			c_size = len(content)
		}
		size, _ := util.DirSize("/var/lib/docker/overlay2/" + id.CacheID + "/diff")
		imageLayerContentData = append(imageLayerContentData, &ImageLayerContentData{
			ImageLayerID: ImageLayerID{
				DiffID:  id.DiffID[:12],
				ChainID: id.ChainID[:12],
				CacheID: id.CacheID[:12],
			},
			Content: content,
			Size:    strconv.FormatInt(size, 10),
		})
	}
	// output
	output(imageLayerContentData, strconv.Itoa(c_size))
}

func outputImageStorageLocation(data []*HistoryImageStorageData, createdBySize string) {
	format := strings.ReplaceAll("%-12s %-14s %-12s %-1111s %-10s %s\n", "1111", createdBySize)
	fmt.Fprintf(os.Stdout, format, "IMAGE", "CREATED", "LAYER", "CREATED BY", "SIZE", "STORAGE")
	for _, v := range data {
		fmt.Fprintf(os.Stdout, format, v.Image, v.Created, v.IsLayer, v.CreatedBy, v.Size, v.StoragePath)
	}
}

func outputContainsNoneLayer(data []*ContainsNoneLayerData, repoSize, tagSize string) {
	format := strings.ReplaceAll(strings.ReplaceAll("%-1111s %-9999s %-12s %s\n", "1111", repoSize), "9999", tagSize)
	fmt.Fprintf(os.Stdout, format, "REPOSITORY", "TAG", "IMAGE ID", "ROOTFS LAYERS")
	for _, v := range data {
		fmt.Fprintf(os.Stdout, format, v.ImageName, v.ImageTag, v.ImageID, strconv.Itoa(v.Layers))
	}
}

func outputContainsBinary(data []*ContainsBinaryData, repoSize, tagSize string) {
	format := strings.ReplaceAll(strings.ReplaceAll("%-1111s %-9999s %-12s %s\n", "1111", repoSize), "9999", tagSize)
	fmt.Fprintf(os.Stdout, format, "REPOSITORY", "TAG", "IMAGE ID", "FILE PATH")
	for _, v := range data {
		fmt.Fprintf(os.Stdout, format, v.ImageName, v.ImageTag, v.ImageID, v.FilePath)
	}
}

func outputContainsImage(data []*ContainsImageData, repoSize, tagSize string) {
	format := strings.ReplaceAll("%-1111s %-9999s %s\n", "1111", repoSize)
	format = strings.ReplaceAll(format, "9999", tagSize)
	fmt.Printf(format, "REPOSITORY", "TAG", "IMAGE ID")
	for _, v := range data {
		fmt.Printf(format, v.ImageName, v.ImageTag, v.ImageID)
	}
}

func output(content []*ImageLayerContentData, size string) {
	format := strings.ReplaceAll("%-14s %-14s %-14s %-34s %s\n", "34", size)
	fmt.Printf(format, "DIFF ID", "CHAIN ID", "CACHE ID", "CONTENT", "SIZE")
	for _, c := range content {
		fmt.Printf(format, c.DiffID, c.ChainID, c.CacheID, c.Content, c.Size)
	}
}
