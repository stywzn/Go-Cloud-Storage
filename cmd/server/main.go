package main

import (
	"github.com/gin-gonic/gin"

	"github.com/stywzn/Go-Cloud-Storage/internal/handler"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/service"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
	"github.com/stywzn/Go-Cloud-Storage/pkg/config"
	"github.com/stywzn/Go-Cloud-Storage/pkg/db"
	"github.com/stywzn/Go-Cloud-Storage/pkg/logger"
)

func main() {
	// 1. 加载配置 (pkg/config)
	// 它会自动读取 config/config.yaml
	config.Init()

	// 2. 初始化日志 (pkg/logger)
	logger.Init()
	logger.Log.Info("系统启动中...")

	// 3. 初始化数据库 (pkg/db)
	// 使用配置里的参数连接 MySQL
	db.Init(config.GlobalConfig.Database.DSN)

	// 底层依赖
	store := storage.NewLocalStorage(config.GlobalConfig.Server.StoragePath)
	fileRepo := repository.NewFileRepository(db.DB)

	// Service 初始化
	fileService := service.NewFileService(fileRepo, store)
	// Handler 初始化
	fileHandler := handler.NewFileHandler(fileService)
	//路由
	r := gin.Default()
	r.POST("/upload", fileHandelr.UploadHandler)

	// 启动
	addr := ":" + config.GlobalConfig.Server.Port
	logger.Log.Info("Server starting", "addr", addr)

	if err := r.Run(addr); err != nil {
		logger.Log.Error("Server start failed", "err", err)
	}
}
