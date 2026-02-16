package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	
	// 👇 注意这里：引入你自己定义的包
	"github.com/stywzn/Go-Cloud-Storage/internal/handler"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
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
	logger.Log.Info("🚀 系统启动中...")

	// 3. 初始化数据库 (pkg/db)
	// 使用配置里的参数连接 MySQL
	db.Init(config.GlobalConfig.Database.DSN)

	// 4. 初始化组件 (依赖注入)
	// 4.1 创建存储引擎 (存本地磁盘)
	store := storage.NewLocalStorage(config.GlobalConfig.Server.StoragePath)
	
	// 4.2 创建数据库仓库
	fileRepo := repository.NewFileRepository(db.DB)

	// 4.3 创建业务处理器 (Handler)
	// 把上面两个组件塞给 Handler，这样 Handler 既能存文件，也能记数据库
	fileHandler := handler.NewFileHandler(fileRepo, store)

	// 5. 启动 Web 服务
	r := gin.Default()

	// 路由绑定
	r.POST("/upload", fileHandler.UploadHandler)
	r.GET("/file/:hash", fileHandler.DownloadHandler)

	// 启动
	addr := ":" + config.GlobalConfig.Server.Port
	logger.Log.Info("Server starting", "addr", addr)
	
	if err := r.Run(addr); err != nil {
		logger.Log.Error("Server start failed", "err", err)
	}
}