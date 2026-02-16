package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

type ServerConfig struct {
	Port        string `mapstructure:"port"`
	StoragePath string `mapstructure:"storage_path"`
	MaxSize     int64  `mapstructure:"max_file_size"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

var GlobalConfig *Config

func LoadConfig() {
	// 默认配置
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.storage_path", "./uploads")
	viper.SetDefault("server.max_file_size", 104857600) // 100MB
	//配置文件设置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("GCS")                              // 设置前缀，防止跟系统变量冲突 (Go Cloud Storage)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // 把 server.port 替换为 SERVER_PORT
	viper.AutomaticEnv()                                   // 开启自动读取

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件未找到，通过日志提醒，但不退出（因为可能有环境变量）
			log.Println("Warning: Config file not found, using default values or environment variables.")
		} else {
			// 配置文件存在但格式不对（比如缩进错了），必须报错退出
			log.Fatalf(" Error reading config file: %s", err)
		}
	}

	// 解析到结构体
	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}

	log.Println("Config loaded successfully")
}
