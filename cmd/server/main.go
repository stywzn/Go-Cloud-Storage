package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stywzn/Go-Cloud-Storage/internal/handler"
	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/service"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
	"github.com/stywzn/Go-Cloud-Storage/pkg/config"
	"github.com/stywzn/Go-Cloud-Storage/pkg/db"
	"github.com/stywzn/Go-Cloud-Storage/pkg/logger"
)

func main() {
	// 1. 初始化日志
	logger.Init()
	defer logger.Log.Sync()

	// 2. 加载配置
	config.LoadConfig()
	conf := config.GlobalConfig

	// 3. 初始化数据库
	if err := db.Init(conf.Database.DSN); err != nil {
		logger.Log.Fatal("Failed to initialize database: " + err.Error())
	}
	logger.Log.Info("Database initialized successfully")

	// ==========================================
	// 👇👇👇 这是你要加上的代码：唤醒 Redis！👇👇👇
	// 假设你刚才 Docker 跑的映射端口是 6379，没设密码，用 0 号库
	// ==========================================
	db.InitRedis("127.0.0.1:6379", "", 0)
	logger.Log.Info("Redis initialized successfully")

	// 4. 自动迁移（创建表）
	autoMigrate()

	// 5. 初始化存储引擎（支持local和minio）
	var storageEngine storage.StorageEngine
	storageType := conf.Storage.Type
	if storageType == "minio" {
		minioEngine, err := storage.NewMinIOStorage(
			conf.MinIO.Endpoint,
			conf.MinIO.AccessKey,
			conf.MinIO.SecretKey,
			conf.MinIO.Bucket,
			conf.MinIO.UseSSL,
		)
		if err != nil {
			logger.Log.Fatal("Failed to initialize MinIO: " + err.Error())
		}
		storageEngine = minioEngine
		logger.Log.Info("MinIO storage initialized: " + conf.MinIO.Endpoint)
	} else {
		storageEngine = storage.NewLocalStorage(conf.Server.StoragePath)
		logger.Log.Info("Local storage initialized: " + conf.Server.StoragePath)
	}

	// 6. 初始化仓库层
	fileRepo := repository.NewFileRepository(db.DB)
	userRepo := repository.NewUserRepository(db.DB)
	taskRepo := repository.NewUploadTaskRepository(db.DB)

	// 7. 初始化服务层
	fileService := service.NewFileService(fileRepo, userRepo, taskRepo, storageEngine)

	// 8. 初始化处理器层
	fileHandler := handler.NewFileHandler(fileService)

	// 9. 初始化 Gin 引擎
	r := gin.Default()

	// 10. 注册路由
	setupRoutes(r, fileHandler)

	// 11. 启动服务器（优雅关闭）
	srv := &http.Server{
		Addr:    ":" + conf.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Log.Info("Server starting on port " + conf.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal("Server startup failed: " + err.Error())
		}
	}()

	// 12. 监听系统信号，优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Fatal("Server forced to shutdown: " + err.Error())
	}

	logger.Log.Info("Server exited")
}

// setupRoutes 注册所有路由
func setupRoutes(r *gin.Engine, fileHandler *handler.FileHandler) {
	// API v1
	v1 := r.Group("/api/v1")
	{
		// 基础上传接口（单文件，无分片）
		v1.POST("/upload", fileHandler.UploadHandler)

		// 分片上传接口
		uploadGroup := v1.Group("/upload")
		//uploadGroup.Use(middleware.JWTAuth()) // 应用JWT中间件
		{
			uploadGroup.POST("/init", fileHandler.InitUpload)
			uploadGroup.PUT("/:upload_id/part/:part_number", fileHandler.UploadPart)
			uploadGroup.POST("/:upload_id/complete", fileHandler.CompleteUpload)
			uploadGroup.GET("/:upload_id/status", fileHandler.GetUploadStatus)
		}
	}

	// 健康检查接口
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Prometheus metrics endpoint
	r.GET("/metrics", gin.WrapF(promhttp.Handler().ServeHTTP))
}

// autoMigrate 自动创建表
func autoMigrate() {
	if err := db.DB.AutoMigrate(
		&model.User{},
		&model.File{},
		&model.UploadTask{},
	); err != nil {
		logger.Log.Fatal("Failed to run migrations: " + err.Error())
	}
	logger.Log.Info("Database migrations completed")
}
