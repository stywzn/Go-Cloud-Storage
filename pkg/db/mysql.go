package db

import (
	"log"
	"os"

	"github.com/stywzn/Go-Cloud-Storage/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	// ⚠️ 记得改成你的 MySQL 密码，或者通过环境变量读取
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/cloud_storage?charset=utf8mb4&parseTime=True&loc=Local"
	}

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ 无法连接数据库: %v", err)
	}

	// 自动迁移表结构
	DB.AutoMigrate(&model.FileMeta{})
	log.Println("✅ 数据库连接成功 & 表结构已同步")
}
