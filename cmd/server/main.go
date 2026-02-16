package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stywzn/Go-Cloud-Storage/internal/handler"
	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"github.com/stywzn/Go-Cloud-Storage/internal/repository"
	"github.com/stywzn/Go-Cloud-Storage/internal/service"
	"github.com/stywzn/Go-Cloud-Storage/internal/storage"
	"github.com/stywzn/Go-Cloud-Storage/pkg/config"
	"github.com/stywzn/Go-Cloud-Storage/pkg/db"
	"github.com/stywzn/Go-Cloud-Storage/pkg/logger"
	"go.uber.org/zap"
)

func main() {

	config.LoadConfig()
	// fmt.Printf("正在尝试连接数据库: [%s]\n", config.GlobalConfig.Database.DSN)
	// 初始化日志 (pkg/logger)
	logger.Init()
	logger.Log.Info("系统启动中...")
	// 初始化数据库
	dsn := config.GlobalConfig.Database.DSN
	if err := db.Init(dsn); err != nil {
		logger.Log.Fatal("Database connection failed", zap.Error(err))
	}

	if err := db.DB.AutoMigrate(&model.File{}); err != nil {
		logger.Log.Fatal("Database migration failed", zap.Error(err))
	}

	// 底层依赖
	store := storage.NewLocalStorage(config.GlobalConfig.Server.StoragePath)
	fileRepo := repository.NewFileRepository(db.DB)

	// Service 初始化
	fileService := service.NewFileService(fileRepo, store)
	// Handler 初始化
	fileHandler := handler.NewFileHandler(fileService)
	//路由
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/upload", fileHandler.UploadHandler)

	srv := &http.Server{
		Addr:    ":" + config.GlobalConfig.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Log.Info("Server is running", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("Server start failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Log.Info("Server exited properly")
}
