# 被动式 Token 计数器 Spec

## Why
当前 TickToken 作为 API 中继引擎依赖用户配置 API key 才能进行 token 计量。用户希望在不提供 API key 的前提下，覆盖任意 IDE（VS Code、Cursor、JetBrains、TRAE、WorkBuddy 等）、CLI 编程工具（Claude Code、Codex CLI、Aider、Cline 等）和任意 AI 大模型（OpenAI、Anthropic、Google Gemini、DeepSeek 等），同时保证 token 计数精确度、区分缓存命中/未命中，并以可视化图表呈现。

核心设计理念：**不内置任何模型和平台列表**，通过本地 HTTPS MITM 代理动态抓取流量，从请求 payload 中自动发现模型名称，从响应中直接提取 token usage（精度 100%），仅在响应不含 usage 时回退到本地 tokenizer 估算。此架构实现对任意平台和模型的零维护自动适配。

## What Changes
- 新增本地 HTTPS MITM 代理模块（默认监听 127.0.0.1:8899），通过用户安装的根 CA 证书透明拦截 HTTPS 流量，无需任何 API key
- 新增**动态 Token 计数引擎**（替代原硬编码多模型 Tokenizer）：
  - **优先策略**：从 API 响应中直接提取 usage 字段（通用 JSON 字段探测，非硬编码字段路径），精度 100%
  - **兜底策略**：响应无 usage 时（如部分 streaming 场景），基于模型名模式匹配选择 tokenizer（gpt-* → o200k_base, claude-* → 官方 tokenizer），仍无匹配 → cl100k_base 兜底，标记 fallback
- 新增**动态 IDE/CLI 适配器层**（替代原硬编码平台列表）：
  - 代理类适配器：任意工具通过 HTTP_PROXY/HTTPS_PROXY 接入，基于 User-Agent / 目标域名**动态推断**工具来源（内置常见工具指纹库，支持扩展，未知工具自动标记为 unknown 并记录原始 UA）
  - 日志类适配器：自动扫描常见日志目录（`~/.claude/`、`~/.codex/log/`、`.aider.chat.history.md` 等），解析会话记录并提取 token 使用信息
- 新增**动态缓存命中/未命中解析器**：通用 JSON 字段探测，自动识别各 provider 的缓存字段（cache_creation_input_tokens、prompt_tokens_details.cached_tokens、cachedContentTokenCount、prompt_cache_hit_tokens 等），无缓存字段时全部记入 cache_miss
- 新增可视化仪表盘：基于 Web（内嵌静态资源），提供时间序列折线图、按模型/工具/缓存状态分组的堆叠柱状图、实时面板
- 新增存储层：本地 SQLite 存储 token 事件（时间戳、工具、模型、prompt/completion/cache 分类 token 数）
- **BREAKING**：移除"必须配置 API key 才能启动"的硬性约束，改为可选配置（仅当用户希望同时作为中继使用时）

## Impact
- Affected specs: 无（项目首个正式 spec）
- Affected code:
  - 新增 `proxy/` —— MITM HTTPS 代理实现
  - 新增 `counter/` —— 动态 token 计数引擎（响应 usage 提取 + 本地 tokenizer 兜底）
  - 新增 `adapters/` —— 动态 IDE/CLI 适配器（指纹库 + 日志扫描）
  - 新增 `cache/` —— 动态缓存命中解析（通用 JSON 探测）
  - 新增 `dashboard/` —— Web 可视化
  - 新增 `storage/` —— SQLite 持久化
  - 修改入口 `cmd/` 启动逻辑以支持无 API key 模式

## ADDED Requirements

### Requirement: 无 API key 流量捕获
系统 SHALL 在用户未提供任何 API key 的情况下，通过本地 HTTPS MITM 代理捕获任意 IDE/CLI 与 AI 服务之间的请求与响应 payload，用于后续 token 计数。

#### Scenario: 首次启动
- **WHEN** 用户首次启动计数器且未配置任何 API key
- **THEN** 系统生成自签名根 CA 证书并提示用户安装到系统信任存储
- **AND** 启动本地 HTTPS 代理监听 127.0.0.1:8899

#### Scenario: 透明拦截
- **WHEN** IDE/CLI 通过 HTTP_PROXY/HTTPS_PROXY 环境变量指向本地代理发起 HTTPS 请求
- **THEN** 系统使用已安装的根 CA 动态签发目标域名证书完成 TLS 握手
- **AND** 解密请求体与响应体后转发，不修改 payload 内容

### Requirement: 动态 Token 计数（响应优先 + 本地兜底）
系统 SHALL 优先从 API 响应中直接提取 token usage 字段（精度 100%），仅在响应不含 usage 时回退到本地 tokenizer 估算。不内置任何模型或平台列表，模型名称从请求 payload 动态提取。

#### Scenario: 响应含 usage 字段（优先策略）
- **WHEN** API 响应体包含 token usage 字段（如 `usage.prompt_tokens`、`usage.input_tokens`、`usageMetadata.promptTokenCount` 等）
- **THEN** 直接使用响应中的 token 计数，精度 100%，无需本地 tokenizer
- **AND** 标记 `source=response`

#### Scenario: 响应不含 usage（兜底策略）
- **WHEN** 响应体未包含 token usage 字段（如部分 streaming 场景或非标准 API）
- **THEN** 从请求体提取 model 名称，基于模型名模式匹配选择 tokenizer
- **AND** 模型名匹配 `gpt-*` → tiktoken `o200k_base`，匹配 `claude-*` → Anthropic 官方 tokenizer
- **AND** 无匹配 → tiktoken `cl100k_base` 兜底，标记 `tokenizer=fallback`
- **AND** 标记 `source=local_tokenizer`

#### Scenario: 任意新模型自动支持
- **WHEN** 请求使用系统从未见过的模型名称
- **THEN** 若响应包含 usage 字段则直接提取（零配置支持）
- **AND** 若响应不含 usage 则兜底到 cl100k_base 估算

### Requirement: 动态 IDE/CLI 适配器
系统 SHALL 不硬编码任何平台列表，通过动态指纹匹配识别流量来源工具，任意新工具自动支持。

#### Scenario: 代理类适配器（动态指纹）
- **WHEN** 任意工具通过代理环境变量发起请求
- **THEN** 系统基于 User-Agent / 目标域名匹配内置指纹库识别工具来源
- **AND** 内置指纹库覆盖常见工具（VS Code、Cursor、JetBrains、Windsurf、TRAE、WorkBuddy 等）
- **AND** 未知工具自动标记为 `unknown` 并记录原始 User-Agent，供后续扩展指纹库

#### Scenario: 日志类适配器（自动扫描）
- **WHEN** 用户使用 Claude Code / Codex CLI / Aider 等无法配置代理的工具
- **THEN** 系统自动扫描常见日志目录（`~/.claude/`、`~/.codex/log/`、`.aider.chat.history.md` 等）
- **AND** 解析会话记录并提取 token 使用信息

### Requirement: 动态缓存命中/未命中解析
系统 SHALL 通过通用 JSON 字段探测自动识别各 provider 的缓存字段，无需硬编码字段路径。

#### Scenario: 已知缓存字段
- **WHEN** 响应包含 `cache_creation_input_tokens`、`cache_read_input_tokens`、`prompt_tokens_details.cached_tokens`、`cachedContentTokenCount`、`prompt_cache_hit_tokens`、`prompt_cache_miss_tokens` 等任一字段
- **THEN** 自动识别并分别记录缓存命中 token 数与未命中 token 数

#### Scenario: 无缓存字段
- **WHEN** 响应未包含任何已知缓存字段
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
- **THEN** 写入一条事件记录（时间戳、工具、模型、各分类 token 数、计数来源）

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
