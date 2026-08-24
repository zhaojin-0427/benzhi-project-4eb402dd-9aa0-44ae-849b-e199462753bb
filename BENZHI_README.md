# BENZHI_README

基于 Go 实现的声景证据册 HTTP API 项目，一款后端服务，已完整实现声景样本从批次登记、内容寻址上传、双人盲标、共识争议、专家仲裁到冻结清单和科研发布证书的本地 JSON HTTP 服务，并提供可验证事件链、崩溃恢复和真实网络自检。

## 项目说明
- 项目：benzhi-project-4eb402dd-9aa0-44ae-849b-e199462753bb
- 项目用途：已完整实现声景样本从批次登记、内容寻址上传、双人盲标、共识争议、专家仲裁到冻结清单和科研发布证书的本地 JSON HTTP 服务，并提供可验证事件链、崩溃恢复和真实网络自检。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/soundledger -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-4eb402dd-9aa0-44ae-849b-e199462753bb-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-4eb402dd-9aa0-44ae-849b-e199462753bb-arm64 linux/arm64
docker run -it benzhi-project-4eb402dd-9aa0-44ae-849b-e199462753bb-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/soundledger -selfcheck -addr=127.0.0.1:19081`
