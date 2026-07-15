# 被动式 Token 计数器 Spec

## Why
当前 TickToken 作为 API 中继引擎依赖用户配置 API key 才能进行 token 计量。用户希望在不提供 API key 的前提下，覆盖主流 IDE（VS Code、Cursor、JetBrains）、CLI 编程工具（Claude Code、Codex CLI、Aider、Cline 等）和主流 AI 大模型（OpenAI、Anthropic、Google Gemini、DeepSeek 等），同时保证 token 计数精确度、区分缓存命中/未命中，并以可视化图表呈现。为此需要引入"被动观测"机制——通过本地 HTTPS MITM 代理与日志/进程适配器在不持有 API key 的情况下捕获请求/响应 payload，再用各模型原生 tokenizer 进行精确计数。

## What Changes
- 新增本地 HTTPS MITM 代理模块（默认监听 127.0.0.1:8899），通过用户安装的根 CA 证书透明拦截 HTTPS 流量，无需任何 API key
- 新增多模型 Tokenizer 引擎：OpenAI 系列（tiktoken cl100k_base / o200k_base）、Anthropic Claude（官方 tokenizer）、Google Gemini（gemini-tokenizer）、DeepSeek/Qwen/Llama（基于 HuggingFace tokenizers 兼容映射）
- 新增 IDE/CLI 适配器层：
  - 代理类适配器：VS Code（含 GitHub Copilot、Cline）、Cursor、JetBrains（含 AI Assistant）、Windsurf —— 通过设置 HTTP_PROXY/HTTPS_PROXY 指向本地代理生效
  - 日志类适配器：Claude Code（解析 `~/.claude/` 会话日志）、Codex CLI（解析 `~/.codex/log/`）、Aider（解析 `.aider.chat.history.md` 与运行时输出）
- 新增缓存命中/未命中解析器：
  - Anthropic：解析响应中 `usage.cache_creation_input_tokens` 与 `usage.cache_read_input_tokens`
  - OpenAI：解析 `usage.prompt_tokens_details.cached_tokens`
  - Gemini：解析 `usageMetadata.cachedContentTokenCount`
  - DeepSeek：解析 `usage.prompt_cache_hit_tokens` 与 `usage.prompt_cache_miss_tokens`
- 新增可视化仪表盘：基于 Web（内嵌静态资源），提供时间序列折线图、按模型/工具/缓存状态分组的堆叠柱状图、实时面板
- 新增存储层：本地 SQLite 存储 token 事件（时间戳、工具、模型、prompt/completion/cache 分类 token 数）
- **BREAKING**：移除"必须配置 API key 才能启动"的硬性约束，改为可选配置（仅当用户希望同时作为中继使用时）

## Impact
- Affected specs: 无（项目首个正式 spec）
- Affected code:
  - 新增 `proxy/` —— MITM HTTPS 代理实现
  - 新增 `tokenizers/` —— 各模型 tokenizer 适配
  - 新增 `adapters/` —— IDE/CLI 适配器
  - 新增 `cache/` —— 缓存命中解析
  - 新增 `dashboard/` —— Web 可视化
  - 新增 `storage/` —— SQLite 持久化
  - 修改入口 `cmd/` 启动逻辑以支持无 API key 模式

## ADDED Requirements

### Requirement: 无 API key 流量捕获
系统 SHALL 在用户未提供任何 API key 的情况下，通过本地 HTTPS MITM 代理捕获 IDE/CLI 与 AI 服务之间的请求与响应 payload，用于后续 token 计数。

#### Scenario: 首次启动
- **WHEN** 用户首次启动计数器且未配置任何 API key
- **THEN** 系统生成自签名根 CA 证书并提示用户安装到系统信任存储
- **AND** 启动本地 HTTPS 代理监听 127.0.0.1:8899

#### Scenario: 透明拦截
- **WHEN** IDE/CLI 通过 HTTP_PROXY/HTTPS_PROXY 环境变量指向本地代理发起 HTTPS 请求
- **THEN** 系统使用已安装的根 CA 动态签发目标域名证书完成 TLS 握手
- **AND** 解密请求体与响应体后转发，不修改 payload 内容

### Requirement: 多模型精确 Tokenizer
系统 SHALL 使用与目标模型匹配的原生 tokenizer 进行 token 计数，确保与官方计费口径一致（误差 ≤ 0.1%）。

#### Scenario: OpenAI 模型
- **WHEN** 请求目标为 `gpt-4o` / `gpt-4o-mini`
- **THEN** 使用 tiktoken `o200k_base` 编码对 prompt 与 completion 计数

#### Scenario: Claude 模型
- **WHEN** 请求目标为 `claude-3-5-sonnet` / `claude-3-7-sonnet`
- **THEN** 使用 Anthropic 官方 tokenizer 计数

#### Scenario: 未知模型兜底
- **WHEN** 目标模型不在已知列表
- **THEN** 使用 tiktoken `cl100k_base` 作为兜底并在日志中标记 `tokenizer=fallback`

### Requirement: IDE/CLI 适配器
系统 SHALL 同时支持代理类与日志类两类适配器，覆盖主流 IDE 与 CLI 工具。

#### Scenario: 代理类适配器
- **WHEN** 用户为 VS Code / Cursor / JetBrains / Windsurf 设置代理环境变量
- **THEN** 系统识别流量来源工具并打标签（基于 User-Agent 或目标域名）

#### Scenario: 日志类适配器
- **WHEN** 用户使用 Claude Code / Codex CLI / Aider 且无法配置代理
- **THEN** 系统监听对应日志目录变化，解析会话记录并提取 token 使用信息

### Requirement: 缓存命中/未命中区分
系统 SHALL 解析响应中的缓存相关字段，分别记录缓存命中 token 数与未命中 token 数。

#### Scenario: Anthropic 缓存
- **WHEN** Claude 响应包含 `cache_creation_input_tokens` 与 `cache_read_input_tokens`
- **THEN** 分别记入 `cache_creation` 与 `cache_hit` 分类

#### Scenario: OpenAI 缓存
- **WHEN** 响应 `prompt_tokens_details.cached_tokens` > 0
- **THEN** 将该数值记入 `cache_hit`，其余 prompt token 记入 `cache_miss`

#### Scenario: 无缓存字段
- **WHEN** 响应未包含任何缓存字段
- **THEN** 全部 prompt token 记入 `cache_miss` 并标记 `cache=unknown`

### Requirement: 可视化图表
系统 SHALL 提供 Web 仪表盘，实时展示 token 使用情况。

#### Scenario: 时间序列
- **WHEN** 用户访问仪表盘
- **THEN** 显示最近 24 小时 token 消耗折线图（可切换时间窗口）

#### Scenario: 分类聚合
- **WHEN** 用户选择"按模型"或"按工具"或"按缓存状态"维度
- **THEN** 显示对应维度的堆叠柱状图，区分 cache_hit / cache_miss / cache_creation / completion

#### Scenario: 实时面板
- **WHEN** 有新请求经过代理
- **THEN** 仪表盘在 2 秒内通过 WebSocket 推送更新

### Requirement: 本地持久化
系统 SHALL 将所有 token 事件持久化到本地 SQLite 数据库，保证进程重启后历史数据可查。

#### Scenario: 写入
- **WHEN** 一次请求/响应完成解析
- **THEN** 写入一条事件记录（时间戳、工具、模型、各分类 token 数）

#### Scenario: 查询
- **WHEN** 仪表盘请求历史数据
- **THEN** 支持按时间范围、工具、模型过滤查询

## MODIFIED Requirements

### Requirement: 启动配置
原 TickToken 启动时强制要求配置至少一个上游 API key。修改为：API key 为可选项；未配置时系统以纯被动观测模式启动，仅做 token 计数与可视化，不提供中继转发能力。

## REMOVED Requirements

### Requirement: 强制 API key 校验
**Reason**: 与"无 API key token 抓取"核心需求冲突
**Migration**: 已配置 API key 的用户不受影响；未配置的用户将自动进入被动观测模式
