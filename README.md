# codex-converter

让 Codex 支持所有 Chat Completions 兼容的模型。

Codex 只支持 Responses API，但大多数模型（DeepSeek、MiMo、Qwen、GLM、Ollama、vLLM...）只支持 Chat Completions。这个代理帮你转换格式。

```
Codex ──→ codex-converter ──→ 任意 Chat Completions API
         (Responses → Chat)    (你配置的provider)
```

## 三步使用

### 1. 编辑 config.toml

```toml
default_provider = "你的provider名"

[server]
port = 8080

[[providers]]
name = "你的provider名"
base_url = "provider的API地址"
model = "模型名"
api_key_env = "存放API Key的环境变量名"
```

### 2. 启动

```bash
export 你的环境变量名=sk-xxxx
./codex-converter
```

### 3. 配置 Codex

编辑 `~/.codex/config.toml`，添加：

```toml
[model_providers.my-proxy]
name = "我的模型"
base_url = "http://127.0.0.1:8080"
wire_api = "responses"
```

启动 Codex：

```bash
codex --model 模型名 --model-provider my-proxy
```

## 添加 Provider 示例

### DeepSeek

```toml
[[providers]]
name = "deepseek"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-pro"
api_key_env = "DEEPSEEK_API_KEY"
```

### MiMo

```toml
[[providers]]
name = "mimo"
base_url = "https://api.xiaomimimo.com"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
auth_style = "api_key_header"
```

### Qwen (通义千问)

```toml
[[providers]]
name = "qwen"
base_url = "https://dashscope.aliyuncs.com/compatible-mode"
model = "qwen-max"
api_key_env = "QWEN_API_KEY"
```

### Ollama (本地模型)

```toml
[[providers]]
name = "ollama"
base_url = "http://localhost:11434/v1"
model = "llama3"
api_key_env = ""
```

### 任意 OpenAI 兼容 API

只要你的 provider 支持 `POST /v1/chat/completions`，就能用：

```toml
[[providers]]
name = "任意名字"
base_url = "https://你的API地址"
model = "模型名"
api_key_env = "环境变量名"
```

## 配置说明

| 字段 | 说明 |
|------|------|
| `name` | provider名称，随便起 |
| `base_url` | API地址，不带 `/chat/completions` 后缀 |
| `model` | 模型名，要和provider支持的一致 |
| `api_key_env` | 存API Key的环境变量名，Ollama等本地模型可留空 |
| `auth_style` | 认证方式：`bearer`（默认）或 `api_key_header`（MiMo用这个） |

## 功能

- ✅ Text streaming（流式输出）
- ✅ Tool calls（函数调用，Codex agent必需）
- ✅ Reasoning/Thinking 模型
- ✅ 任何 Chat Completions 兼容的 provider

## 构建

```bash
go build -o codex-converter ./cmd/server
```

## Docker

```bash
docker build -t codex-converter .
export API_KEY=sk-xxx
docker run -e API_KEY -p 8080:8080 codex-converter
```
