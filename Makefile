.PHONY: build run clean build-linux build-windows build-mac build-all docker-build-linux-amd64 docker-build-linux-arm64 docker-build-all docker-up docker-down docker-logs

# 应用名称
APP_NAME=finance

# Docker 镜像
IMAGE_NAME=finance
IMAGE_TAG=latest

UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),x86_64)
HOST_ARCH := amd64
else ifeq ($(UNAME_M),aarch64)
HOST_ARCH := arm64
else
HOST_ARCH := $(UNAME_M)
endif

# 默认构建当前平台
build:
	@echo "构建当前平台二进制文件..."
	go build -ldflags="-s -w" -o $(APP_NAME) .
	@echo "构建完成: $(APP_NAME)"

# 运行应用
run:
	go run main.go

# 构建 Linux amd64
build-linux:
	@echo "构建 Linux amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(APP_NAME)-linux-amd64 .
	@echo "构建完成: $(APP_NAME)-linux-amd64"

# 构建 Linux arm64
build-linux-arm:
	@echo "构建 Linux arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(APP_NAME)-linux-arm64 .
	@echo "构建完成: $(APP_NAME)-linux-arm64"

# 构建 Windows
build-windows:
	@echo "构建 Windows amd64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(APP_NAME)-windows-amd64.exe .
	@echo "构建完成: $(APP_NAME)-windows-amd64.exe"

# 构建 macOS Intel
build-mac:
	@echo "构建 macOS amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(APP_NAME)-darwin-amd64 .
	@echo "构建完成: $(APP_NAME)-darwin-amd64"

# 构建 macOS Apple Silicon
build-mac-arm:
	@echo "构建 macOS arm64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(APP_NAME)-darwin-arm64 .
	@echo "构建完成: $(APP_NAME)-darwin-arm64"

# 构建所有平台
build-all: build-linux build-linux-arm build-windows build-mac build-mac-arm
	@echo "所有平台构建完成!"

# 清理构建文件
clean:
	@echo "清理构建文件..."
	rm -f $(APP_NAME) $(APP_NAME)-*
	@echo "清理完成"

# 下载依赖
deps:
	go mod tidy
	go mod download

# 打包 Linux amd64 Docker 镜像
docker-build-linux-amd64:
	@echo "构建 Docker 镜像 $(IMAGE_NAME):$(IMAGE_TAG)-linux-amd64..."
	docker buildx build \
		--platform linux/amd64 \
		-t $(IMAGE_NAME):$(IMAGE_TAG)-linux-amd64 \
		--load \
		.
	@echo "构建完成: $(IMAGE_NAME):$(IMAGE_TAG)-linux-amd64"

# 打包 Linux arm64 Docker 镜像
docker-build-linux-arm64:
	@echo "构建 Docker 镜像 $(IMAGE_NAME):$(IMAGE_TAG)-linux-arm64..."
	docker buildx build \
		--platform linux/arm64 \
		-t $(IMAGE_NAME):$(IMAGE_TAG)-linux-arm64 \
		--load \
		.
	@echo "构建完成: $(IMAGE_NAME):$(IMAGE_TAG)-linux-arm64"

# 打包全部 Docker 平台
docker-build-all: docker-build-linux-amd64 docker-build-linux-arm64
	@echo "所有平台 Docker 镜像构建完成!"

# 启动应用与 MySQL。缺少当前架构镜像时先打包对应平台
docker-up:
	@docker image inspect $(IMAGE_NAME):$(IMAGE_TAG)-linux-$(HOST_ARCH) >/dev/null 2>&1 || $(MAKE) docker-build-linux-$(HOST_ARCH)
	FINANCE_IMAGE=$(IMAGE_NAME):$(IMAGE_TAG)-linux-$(HOST_ARCH) docker compose up -d

# 停止并移除 compose 容器（保留数据卷）
docker-down:
	docker compose down

# 查看 compose 日志
docker-logs:
	docker compose logs -f

# 帮助信息
help:
	@echo "可用命令:"
	@echo "  make build          - 构建当前平台二进制文件"
	@echo "  make run            - 运行应用"
	@echo "  make build-linux    - 构建 Linux amd64 版本"
	@echo "  make build-linux-arm - 构建 Linux arm64 版本"
	@echo "  make build-windows  - 构建 Windows amd64 版本"
	@echo "  make build-mac      - 构建 macOS Intel 版本"
	@echo "  make build-mac-arm  - 构建 macOS Apple Silicon 版本"
	@echo "  make build-all      - 构建所有平台版本"
	@echo "  make clean          - 清理构建文件"
	@echo "  make deps           - 下载依赖"
	@echo "  make docker-build-linux-amd64 - 打包 Linux amd64 Docker 镜像"
	@echo "  make docker-build-linux-arm64 - 打包 Linux arm64 Docker 镜像"
	@echo "  make docker-build-all         - 打包全部 Docker 平台"
	@echo "  make docker-up                - 启动应用与 MySQL（缺少当前架构镜像时先打包）"
	@echo "  make docker-down              - 停止 compose 服务"
	@echo "  make docker-logs              - 查看 compose 日志"

