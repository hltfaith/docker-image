package service

type ImageInterface interface {
	// dockerfile中每条指令的存储位置
	ImageFileStorageLocation()

	// 根据none标记的镜像, 比较镜像层数最贴近的 "镜像名称:TAG"
	ContainsNoneLayerImage()

	// 包含匹配文件的镜像
	ContainsBinaryfile()

	// 根据镜像层id, 找出所关联的镜像
	ContainsImageLayerID()

	// 镜像层内容
	ImageLayerContent()
}

type ImageRelation struct {
	ImageId   string `json:"image_id"`   // 镜像层ID, 镜像ID, 镜像TAG
	ImageFile string `json:"image_file"` // 镜像文件
}
