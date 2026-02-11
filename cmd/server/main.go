package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/stywzn/Go-Cloud-Storage/internal/handler"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/pkg/db"
)

func main() {
	// 1. 确保 storage 目录存在
	os.MkdirAll("storage", 0755)

	// 2. 初始化数据库
	db.Init()

	// 3. 依赖注入
	fileRepo := repository.NewFileRepository(db.DB)
	fileHandler := handler.NewFileHandler(fileRepo)

	// 4. 启动 Web 服务
	r := gin.Default()

	// 允许最大上传 100MB
	r.MaxMultipartMemory = 100 << 20

	r.POST("/upload", fileHandler.UploadHandler)
	r.GET("/file/:hash", fileHandler.DownloadHandler)

	fmt.Println("🚀 Go-Cloud-Storage running on :8080")
	r.Run(":8080")
}
