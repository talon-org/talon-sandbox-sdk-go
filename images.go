package talonsandbox

import (
	"context"
	"fmt"
)

// ImageInfo 描述一个可用的 baseimage。
// 字段与后端 dto.ImageDTO 一一对应（见 dto.go ImageDTO）。
type ImageInfo struct {
	// ID 是 image 的唯一标识符（如 "img_abc123"）。
	ID string `json:"id"`
	// Name 是 image 的友好名称，如 "node:20-bookworm"。
	Name string `json:"name"`
	// URL 是 OCI 镜像拉取地址。
	URL string `json:"url"`
	// SHA256 是镜像层的校验摘要。
	SHA256 string `json:"sha256"`
	// OS 是目标操作系统，通常 "linux"。
	OS string `json:"os"`
	// Arch 是 CPU 架构，通常 "amd64"。
	Arch string `json:"arch"`
	// Source 是来源：内置镜像为 "builtin"，管理员上传为 "admin"。
	Source string `json:"source"`
	// IsDefault 标记是否为平台默认镜像（创建 sandbox 时未指定 image_id 时的 fallback）。
	IsDefault bool `json:"is_default"`
	// Description 是可选的描述文字。
	Description string `json:"description,omitempty"`
	// CreatedAt 是创建时间戳（Unix 秒）。
	CreatedAt int64 `json:"created_at"`
}

// ListImages 查询平台可用的 baseimage 列表（GET /v1/images）。
//
// 该端点是租户级读操作（任何已认证成员均可调用），结果可用于创建 sandbox 时
// 填写 Opts.Image。
//
// 用法示例：
//
//	images, err := talonsandbox.ListImages(ctx)
//	for _, img := range images {
//	    fmt.Println(img.Name, img.ID)
//	}
func ListImages(ctx context.Context, clientOpts ...Option) ([]ImageInfo, error) {
	c := resolveClient(clientOpts)
	var out struct {
		Images []ImageInfo `json:"images"`
	}
	if _, err := c.get(ctx, "/v1/images", &out); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	return out.Images, nil
}
