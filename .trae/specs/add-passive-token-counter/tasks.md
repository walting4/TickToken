# Tasks

- [ ] Task 1: 搭建项目骨架与配置入口
  - [ ] SubTask 1.1: 初始化模块目录结构（proxy/ tokenizers/ adapters/ cache/ dashboard/ storage/ cmd/）
  - [ ] SubTask 1.2: 实现配置加载，支持"无 API key 被动观测模式"启动（BREAKING：移除强制 API key 校验）
  - [ ] SubTask 1.3: 实现日志与可观测性基础（结构化日志、运行模式标记）

- [ ] Task 2: 实现本地 HTTPS MITM 代理
  - [ ] SubTask 2.1: 实现自签名根 CA 证书生成与首次启动引导提示
  - [ ] SubTask 2.2: 实现基于根 CA 的目标域名证书动态签发与 TLS 拦截
  - [ ] SubTask 2.3: 实现代理监听 127.0.0.1:8899，支持 HTTP_PROXY/HTTPS_PROXY 接入
  - [ ] SubTask 2.4: 实现请求/响应 payload 解密捕获（不修改原文，透传转发）

- [ ] Task 3: 实现多模型 Tokenizer 引擎
  - [ ] SubTask 3.1: 集成 tiktoken，支持 cl100k_base（兜底）与 o200k_base（gpt-4o 系列）
  - [ ] SubTask 3.2: 集成 Anthropic 官方 tokenizer，支持 claude-3-5/3-7 系列
  - [ ] SubTask 3.3: 集成 Gemini tokenizer，支持 gemini-1.5/2.0 系列
  - [ ] SubTask 3.4: 实现 DeepSeek/Qwen/Llama 的 HuggingFace tokenizers 映射
  - [ ] SubTask 3.5: 实现模型路由表与未知模型兜底逻辑（标记 tokenizer=fallback）

- [ ] Task 4: 实现 IDE/CLI 适配器层
  - [ ] SubTask 4.1: 实现代理类适配器基类（基于 User-Agent / 目标域名识别工具来源）
  - [ ] SubTask 4.2: 实现 VS Code / Cursor / Windsurf / JetBrains 适配器（流量打标签）
  - [ ] SubTask 4.3: 实现 Claude Code 日志适配器（解析 ~/.claude/ 会话日志）
  - [ ] SubTask 4.4: 实现 Codex CLI 日志适配器（解析 ~/.codex/log/）
  - [ ] SubTask 4.5: 实现 Aider 日志适配器（解析 .aider.chat.history.md 与运行时输出）

- [ ] Task 5: 实现缓存命中/未命中解析器
  - [ ] SubTask 5.1: 实现 Anthropic 缓存字段解析（cache_creation_input_tokens / cache_read_input_tokens）
  - [ ] SubTask 5.2: 实现 OpenAI 缓存字段解析（prompt_tokens_details.cached_tokens）
  - [ ] SubTask 5.3: 实现 Gemini 缓存字段解析（usageMetadata.cachedContentTokenCount）
  - [ ] SubTask 5.4: 实现 DeepSeek 缓存字段解析（prompt_cache_hit_tokens / prompt_cache_miss_tokens）
  - [ ] SubTask 5.5: 实现无缓存字段的兜底处理（全部记入 cache_miss，标记 cache=unknown）

- [ ] Task 6: 实现本地 SQLite 存储层
  - [ ] SubTask 6.1: 设计 token_events 表结构（时间戳、工具、模型、prompt/completion/cache_hit/cache_miss/cache_creation token 数）
  - [ ] SubTask 6.2: 实现事件写入接口
  - [ ] SubTask 6.3: 实现按时间范围/工具/模型过滤查询接口

- [ ] Task 7: 实现 Web 可视化仪表盘
  - [ ] SubTask 7.1: 内嵌静态资源，提供仪表盘 HTML/CSS/JS（折线图 + 堆叠柱状图）
  - [ ] SubTask 7.2: 实现时间序列接口（默认 24 小时窗口，可切换）
  - [ ] SubTask 7.3: 实现按模型/工具/缓存状态维度的聚合接口
  - [ ] SubTask 7.4: 实现 WebSocket 实时推送（2 秒内同步新事件）

- [ ] Task 8: 端到端集成与验证
  - [ ] SubTask 8.1: 编写代理拦截 + tokenizer + 缓存解析的集成测试（mock 上游）
  - [ ] SubTask 8.2: 编写日志适配器的解析单元测试（使用样例日志）
  - [ ] SubTask 8.3: 验证仪表盘数据与存储层一致性
  - [ ] SubTask 8.4: 编写用户使用文档：CA 安装、代理配置、各工具接入步骤

# Task Dependencies
- Task 2 依赖 Task 1（配置与日志基础）
- Task 3 依赖 Task 1（配置）
- Task 4 依赖 Task 2（代理类适配器）与 Task 1（日志类适配器基类）
- Task 5 依赖 Task 3（缓存字段需结合 tokenizer 结果分类）
- Task 6 依赖 Task 1
- Task 7 依赖 Task 6
- Task 8 依赖 Task 2/3/4/5/6/7
- Task 3 与 Task 6 可并行
- Task 4（日志类部分）与 Task 2 可并行
