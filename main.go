// author: changhao
// mail: 2083969687@qq.com
package main

import (
	"docker-image/service"
	"flag"
	"fmt"
	"os"
)

var (
	image    = flag.String("i", "", "layer id, image id, image:tag") // 层id, 镜像id, 镜像:tag
	layer    = flag.Bool("layer", false, "docker-image layer")       // 镜像层
	history  = flag.Bool("history", false, "docker-image history")   // 镜像 history记录
	relation = flag.Bool("relation", false, "docker-image relation") // 镜像层关联的镜像
	file     = flag.String("file", "", "docker-image file")          // 镜像文件路径
	none     = flag.Bool("none", false, "docker-image none")         // none标记镜像相似镜像层名称
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", "docker-image")
		flag.PrintDefaults()
		examples := "\nexamples: \n" +
			"   docker-image -layer -i xxxxxxxx \n" +
			"   docker-image -history -i xxxxxxxx \n" +
			"   docker-image -relation -i xxxxxxxx \n" +
			"   docker-image -file /root/file.txt \n" +
			"   docker-image -none -i xxxxxxxx \n"
		fmt.Fprintf(os.Stderr, examples)
	}
	flag.Parse()
	s := service.ImageRelation{}
	if *file != "" {
		s.ImageFile = *file
		s.ContainsBinaryfile()
		os.Exit(0)
	}
	if *image == "" {
		fmt.Fprintf(os.Stderr, "error: docker-image -i parameter is null")
		os.Exit(0)
	}
	s.ImageId = *image
	switch {
	case *layer:
		s.ImageLayerContent()
	case *history:
		s.ImageFileStorageLocation()
	case *relation:
		s.ContainsImageLayerID()
	case *none:
		s.ContainsNoneLayerImage()
	}
}
