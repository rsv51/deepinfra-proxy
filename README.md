# deepinfra-proxy

一个极简的 Go 语言 HTTP 代理，专为将来自客户端的请求转发到 https://api.deepinfra.com 设计，支持基于预设 KEY 的简单鉴权、路径重写及常用请求头覆盖，便于在本地或内网环境中快速代理 DeepInfra API 请求。

## 功能

- 基于环境变量 KEYS 的简单 token 验证（支持逗号分隔多个 key）。
- 支持从 Authorization、X-Api-Key、query 参数（key 或 api_key）中提取 token。
- 对常见的 OpenAI 风格路径做替换：
  - /v1/chat/completions -> /v1/openai/chat/completions
  - /v1/completions -> /v1/openai/completions
- 复制并转发请求头（排除 Authorization 和 X-Api-Key），并注入一组与 DeepInfra 兼容的头部。
- 保持原始请求的 HTTP 方法、Body、查询字符串。

## 快速开始

1. 克隆或将此仓库放在某个目录（示例以 Windows 路径为例）：

   n:\GO\deepinfra-proxy

2. 依赖

   只需安装 Go（推荐 1.20+）。

3. 本地运行（开发）

   在项目目录下：

   ```bash
   go run main.go -p 8080
   ```

   或通过环境变量覆盖端口：

   ```bash
   PORT=9090 go run main.go
   ```

4. 设置允许的 keys

   使用环境变量 KEYS，多个 key 用逗号分隔。例如：

   ```bash
   KEYS="linux-do,my-other-key" go run main.go
   ```

   如果未提供 KEYS，程序会默认允许名为 `linux-do` 的 key（仅用于开发/测试）。

5. 示例请求

   假设代理运行在本机 8080：

   - 使用 Authorization 头：

     ```bash
     curl -v -H "Authorization: Bearer <YOUR_KEY>" -H "Content-Type: application/json" \
       -d '{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}' \
       http://localhost:8080/v1/chat/completions
     ```

   - 使用 query 参数：

     ```bash
     curl "http://localhost:8080/v1/chat/completions?key=<YOUR_KEY>" -d '{...}'
     ```

## 构建为二进制

在项目根目录执行：

```bash
go build -o deepinfra
```

在 Windows 下可直接运行生成的可执行文件：

```bash
deepinfra.exe -p 8080
```

项目中还包含用于交叉编译的脚本示例 `build_linux_amd64.ps1`。

## 配置项

- PORT: 可选，覆盖默认监听端口（默认 8080）。
- -p: 命令行参数，指定监听端口（优先级低于 PORT 环境变量）。
- KEYS: 可选，逗号分隔的允许 token 列表。

## 注意事项

- 本代理做了简单的 token 校验，不适合作为公开生产环境下的唯一鉴权手段。将其放在受限网络或与更严格的鉴权层联合使用。
- 转发时会覆盖并注入部分请求头以匹配 DeepInfra 的预期行为，若你依赖特殊头部，请根据需要修改代码。

## 目录结构

- main.go — 程序主文件
- go.mod — Go 模块文件
- build_linux_amd64.ps1 — 示例交叉编译脚本

## 许可

MIT License（若需要其他许可，请自行更改）。

