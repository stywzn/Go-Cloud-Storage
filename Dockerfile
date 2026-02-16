# ==========================================
# 🏗️ 第一阶段：构建 (Builder)
# ==========================================
# 使用官方 Go 镜像，alpine 版本体积小
FROM golang:1.23-alpine AS builder

# 1. 设置必要的环境变量
# CGO_ENABLED=0: 禁用 CGO，实现静态编译 (为了让二进制文件能在空系统里跑)
# GOOS=linux: 目标系统是 Linux
# GOPROXY: 设置国内代理，加速依赖下载 (必备!)
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOPROXY=https://goproxy.cn,direct

# 2. 设置工作目录
WORKDIR /app

# 3. 缓存优化：先拷贝依赖文件
# 只要 go.mod 和 go.sum 没变，Docker 就不会重新下载依赖，秒级构建
COPY go.mod go.sum ./
RUN go mod download

# 4. 拷贝源码
COPY . .

# 5. 编译
# -ldflags="-s -w": 去掉调试信息和符号表，让二进制文件体积减少 30%
# -o server: 输出文件名为 server
# cmd/server/main.go: 你的入口文件路径
RUN go build -ldflags="-s -w" -o server cmd/server/main.go

# ==========================================
# 🚀 第二阶段：运行 (Runner)
# ==========================================
# 使用极简的 alpine 镜像 (只有 5MB 左右)
FROM alpine:latest

# 1. 设置工作目录
WORKDIR /app

# 2. 安装基础依赖 (可选但推荐)
# ca-certificates: 防止 HTTPS 请求报错 (比如你要调微信支付接口)
# tzdata: 设置时区 (不然容器时间会慢 8 小时)
RUN apk --no-cache add ca-certificates tzdata

# 3. 设置时区为上海
ENV TZ=Asia/Shanghai

# 4. 从第一阶段拷贝构建好的二进制文件
COPY --from=builder /app/server .

# 5. 拷贝配置文件 (作为默认配置)
# 虽然我们主要用环境变量，但拷贝进去作为兜底是个好习惯
COPY --from=builder /app/config ./config

# 6. 挂载上传目录 (告诉 Docker 这个目录是存数据的)
# 这样在 docker run 时，如果不挂载宿主机目录，Docker 会自动创建一个匿名卷，防止数据丢失
VOLUME ["/app/uploads"]

# 7. 暴露端口 (声明)
EXPOSE 8080

# 8. 启动命令
CMD ["./server"]