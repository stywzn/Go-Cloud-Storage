# ==============================================================================
# 🛠️ 变量定义 (Variables)
# ==============================================================================
BINARY_NAME=server
MAIN_FILE=cmd/server/main.go
DOCKER_IMAGE=go-cloud-storage
DOCKER_CONTAINER=gcs-container

#以此确保在 Windows/Mac/Linux 下都能获取到正确的路径
GO_FILES=$(shell find . -name "*.go" -type f)

# ==============================================================================
# 📝 核心命令 (Targets)
# ==============================================================================

# 伪目标：防止文件夹里有同名文件导致命令无法执行
.PHONY: all build run clean lint test docker-build docker-run docker-stop help

# 默认动作：输入 `make` 没参数时，默认执行 help
all: help

# ------------------------------------------------------------------------------
# 💻 本地开发 (Local Development)
# ------------------------------------------------------------------------------

## run: 🚀 直接运行项目 (go run)
run:
	@echo " > Running application..."
	@go run $(MAIN_FILE)

## build: 🔨 编译二进制文件到 bin/ 目录
build:
	@echo " > Building binary..."
	@mkdir -p bin
	@go build -o bin/$(BINARY_NAME) $(MAIN_FILE)
	@echo " > Binary built at bin/$(BINARY_NAME)"

## tidy: 🧹 整理依赖 (go mod tidy)
tidy:
	@echo " > Tidying go modules..."
	@go mod tidy

## fmt: 🎨 格式化代码 (go fmt)
fmt:
	@echo " > Formatting code..."
	@go fmt ./...

# ------------------------------------------------------------------------------
# 🛡️ 代码质量与测试 (Quality Assurance)
# ------------------------------------------------------------------------------

## lint: 🔍 静态代码检查 (需要安装 golangci-lint)
# 这是工程化最重要的一步！它能帮你发现潜在 Bug 和不规范的代码
lint:
	@echo " > Running linter..."
	@hash golangci-lint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		echo "Downloading golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2; \
	fi
	@golangci-lint run

## test: 🧪 运行单元测试
test:
	@echo " > Running tests..."
	@go test -v ./...

# ------------------------------------------------------------------------------
# 🐳 Docker 流程 (Docker Integration)
# ------------------------------------------------------------------------------

## docker-build: 📦 构建 Docker 镜像
docker-build:
	@echo " > Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):latest .

## docker-run: ▶️ 启动 Docker 容器 (单机模式)
docker-run:
	@echo " > Running Docker container..."
	@docker run -d \
		--name $(DOCKER_CONTAINER) \
		-p 8080:8080 \
		-e GCS_SERVER_PORT=8080 \
		-v $(PWD)/uploads:/app/uploads \
		$(DOCKER_IMAGE):latest
	@echo " > Container $(DOCKER_CONTAINER) started on port 8080"

## docker-stop: ⏹️ 停止并删除容器
docker-stop:
	@echo " > Stopping container..."
	@-docker stop $(DOCKER_CONTAINER)
	@echo " > Removing container..."
	@-docker rm $(DOCKER_CONTAINER)

## compose-up: 🆙 使用 Docker Compose 启动 (含数据库)
compose-up:
	@echo " > Starting services via Docker Compose..."
	@docker-compose up -d

## compose-down: ⬇️ 停止 Docker Compose
compose-down:
	@echo " > Stopping services..."
	@docker-compose down

# ------------------------------------------------------------------------------
# 🧹 清理 (Cleanup)
# ------------------------------------------------------------------------------

## clean: 🗑️ 清理二进制文件和临时文件
clean:
	@echo " > Cleaning up..."
	@rm -rf bin
	@go clean

# ------------------------------------------------------------------------------
# ℹ️ 帮助 (Help)
# ------------------------------------------------------------------------------

## help: ❓ 显示帮助信息
help:
	@echo "Choose a command run in "$(APP_NAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'