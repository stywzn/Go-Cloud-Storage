# Go Cloud Storage (Base)

基于 Go 语言构建的高性能、云原生文件存储服务后端。
本项目采用 **Clean Architecture** 架构设计，旨在提供一个不仅功能可用，且易于维护、易于扩展的工程化基座。

## 🏗 技术栈 (Tech Stack)

- **核心框架**: Gin (Web Framework)
- **数据库**: MySQL 8.0 + GORM (ORM)
- **配置管理**: Viper (支持 YAML 及 环境变量注入)
- **日志监控**: Zap (高性能结构化日志)
- **容器化**: Docker + Docker Compose
- **构建工具**: Makefile

## ✨ 核心特性 (Features)

这是项目的 **Main (Base)** 分支，实现了云存储最核心的原子能力：

- **标准三层架构**: `Handler` -> `Service` -> `Repository`，彻底解耦业务逻辑。
- **Hash 去重 (秒传)**: 基于 SHA-256 计算文件指纹，实现相同文件“秒传”，极大节省存储空间。
- **工程化规范**: 统一的错误处理、优雅关机 (Graceful Shutdown)、依赖注入。
- **云原生就绪**: 提供极致精简的 Docker 镜像 (基于 Alpine)，支持一键部署。

## 📂 目录结构 (Directory Structure)

遵循 Standard Go Project Layout：

```text
├── cmd/            # 程序入口 (main.go)
├── config/         # 配置文件模板
├── internal/       # 内部业务代码 (不对外暴露)
│   ├── handler/    # HTTP 接口层 (解析请求)
│   ├── service/    # 核心业务逻辑层 (Hash计算、去重判断)
│   ├── repository/ # 数据访问层 (MySQL 操作)
│   ├── model/      # 数据库模型定义
│   └── storage/    # 存储引擎接口 (本地磁盘/OSS)
├── pkg/            # 公共工具包 (Logger, DB, Config)
└── uploads/        # 本地存储目录