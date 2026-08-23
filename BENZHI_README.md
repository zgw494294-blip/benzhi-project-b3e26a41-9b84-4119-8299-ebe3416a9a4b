# BENZHI_README

## 项目说明
- 项目：benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b
- 项目用途：实验室废弃物合规交接台已实现从批次登记、条目封装、相容性审查、退回整改、清单冻结、双方现场确认到不可变归档凭据的完整流程，提供 SQLite 持久化、乐观并发控制、requestId 幂等和原生响应式浏览器工作台。
- Go 工具链：`golang:1.23.0`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b-arm64 linux/arm64
docker run -it benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
