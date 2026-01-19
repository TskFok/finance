.PHONY: build run clean build-linux build-windows build-mac build-all

# 应用名称
APP_NAME=finance

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

