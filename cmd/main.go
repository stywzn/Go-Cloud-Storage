package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// 定义文件存储的根目录
const StorageRoot = "./storage"

func main() {
	// 1. 初始化 Gin
	r := gin.Default()

	// 2. 确保存储目录存在
	if err := os.MkdirAll(StorageRoot, 0755); err != nil {
		log.Fatal("无法创建存储目录: ", err)
	}

	// 🛠️ 接口 1: 上传文件
	// curl -X POST -F "file=@/path/to/image.jpg" http://localhost:8080/upload
	r.POST("/upload", func(c *gin.Context) {
		// 从请求中获取文件 (表单 key 必须是 "file")
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(400, gin.H{"error": "请上传文件 (key='file')"})
			return
		}
		defer file.Close()

		// 构造保存路径: ./storage/filename.ext
		// 注意：实际项目中要处理重名问题，这里先偷懒
		filename := filepath.Base(header.Filename)
		dst := filepath.Join(StorageRoot, filename)

		// 创建目标文件
		out, err := os.Create(dst)
		if err != nil {
			c.JSON(500, gin.H{"error": "无法创建文件"})
			return
		}
		defer out.Close()

		// 关键点：流式拷贝 (IO Copy)
		// 这样即使上传 1GB 的视频，内存也不会爆，因为它是边读边写的
		written, err := io.Copy(out, file)
		if err != nil {
			c.JSON(500, gin.H{"error": "文件写入失败"})
			return
		}

		c.JSON(200, gin.H{
			"msg":      "上传成功",
			"filename": filename,
			"size":     written,
		})
	})

	// 🛠️ 接口 2: 下载文件
	// curl http://localhost:8080/download/image.jpg
	r.GET("/download/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		targetPath := filepath.Join(StorageRoot, filename)

		// 检查文件是否存在
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "文件不存在"})
			return
		}

		// 告诉浏览器这是一个附件，触发下载
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Header("Content-Type", "application/octet-stream")

		// 发送文件
		c.File(targetPath)
	})

	// 启动服务
	fmt.Println("🚀 Simple OSS running at :8080")
	r.Run(":8080")
}
