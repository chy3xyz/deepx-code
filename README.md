# deepx

> 核心亮点，增加本地OCR，补齐 DeepSeek 不支持图片识别的最大短板。

AI 代码助手 —— Go 写的 TUI,跑在任何 OpenAI 兼容的 LLM 上,默认对接 DeepSeek。

类似 Claude Code 类工具,定位上更轻:单二进制、纯键盘、聚焦中文用户、强调 token 经济。

![deepx screenshot](assets/screenshot.jpg)

## 特性

- **双模型路由**:`flash` 起手处理机械单步,关键词命中 / 任务超阈值时升 `pro`。本地路由,零 LLM 调用延迟
- **Plan DAG 调度**:模型主动调 `CreatePlan` 拆任务,deepx 按依赖跑并发子 agent,每个节点独立选 flash / pro
- **会话压缩**:长对话自动压缩,分层摘要保留关键决策;重启恢复与原会话完全一致,DeepSeek 硬盘缓存友好
- **审核模式** (默认):Write / Update / Bash 需人工 YES/NO 确认,其余工具自动执行;`/auto` 切全自动
- **会话持久化**:按 workspace 路径哈希分库,对话按天落盘 `~/.deepx/sessions/{sid}/YYYY-MM-DD.jsonl`,重启自动续上
- **`Memory` 工具**:LLM 主动给关键词,内部扫历史 jsonl 命中。不预注入、不占 token,需要时才查
- **`LoadSkill` 工具**:扫 workspace + 全局 skill 目录,LLM 按 description 决定加载哪个;兼容已有 `~/.claude/skills/`
- **`SwitchModel` 工具**:LLM 觉得任务复杂时主动升 pro(单向、不重启本轮)
- **本地 OCR**:Ctrl+V 粘贴图片 → 调 `OCR` 工具(PaddleOCR PP-OCRv5,首次下载 ~37MB 模型)
- **联网**:`Search` 走 Bing 中文站 / Bocha / Tavily;`Fetch` 抓 URL 自动 HTML→正文。中文用户默认友好
- **Markdown 实时渲染**:流式 token 进来时 glamour 每帧渲染;显示仅保留最近 10 轮,长对话不卡顿

## 安装

### 一键安装(macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/itmisx/deepx/main/scripts/install.sh | bash
```

### 一键安装(Windows / PowerShell)

```powershell
irm https://raw.githubusercontent.com/itmisx/deepx/main/scripts/install.ps1 | iex
```

### 从源码构建(全平台)

```bash
git clone https://github.com/itmisx/deepx.git
cd deepx
go build .             # 需要 Go 1.25+,Windows 产物为 deepx.exe
```

### 卸载

- macOS / Linux:`rm -f ~/.local/bin/deepx && rm -rf ~/.deepx`
- Windows:删 `%LOCALAPPDATA%\Programs\deepx` 与 `%USERPROFILE%\.deepx` 两个目录,从 PATH 移除 `deepx` 所在目录

## 配置

首次启动会弹 modal 引导填 API key,自动写到 `~/.deepx/model.yaml`(权限 `0600`)。手动配置示例:

```yaml
flash:
  base_url: https://api.deepseek.com
  model: deepseek-v4-flash
  api_key: sk-xxx
  context_window: 1048576   # 1M tokens,DeepSeek 默认。可选,压缩阈值按此计算
pro:
  base_url: https://api.deepseek.com
  model: deepseek-v4-pro
  api_key: sk-xxx
  context_window: 1048576
```

两个 role 独立 endpoint —— 想 flash 用便宜小模型、pro 用 Claude / GPT 各自填,只要 OpenAI Chat Completions 协议兼容。

`context_window`(tokens)控制会话压缩触发阈值(`窗口 × 70%`),含 `deepseek` 的模型默认 1M,其余 64K。

## 使用

```bash
cd <你的项目目录>
deepx
```

deepx 把 `os.Getwd()` 作 workspace,所有工具调用 / Memory 检索 / 项目级 skill 发现都以这个目录为根。

### Slash 命令

| 命令      | 作用                                          |
| --------- | --------------------------------------------- |
| `/plan`   | 切只读模式(Write / Update / Bash 禁用)         |
| `/auto`   | 切全工具模式(全部工具自动执行)                  |
| `/review` | 切审核模式(Write / Update / Bash 需人工确认)    |
| `/mode`   | 显示当前模式                                   |
| `/config` | 重新配置 API key                               |
| `/skills` | 列出所有命中的 skill + 完整路径                 |
| `/help`   | 帮助                                           |

审核模式下,Write / Update / Bash 调用前弹出确认弹窗,↑/↓ 切换 YES/NO,Enter 确认,Esc 拒绝。

### 快捷键

| 键                                                         | 作用                                             |
| ---------------------------------------------------------- | ------------------------------------------------ |
| `Enter`                                                    | 发送                                             |
| `Ctrl+Shift+A` / `Cmd+Shift+A`(支持 Kitty Protocol 的终端) | 输入框全选                                       |
| `Ctrl+V`                                                   | 粘贴(含图片)                                     |
| `Ctrl+C`                                                   | 流式中:中断;空闲时:退出                          |
| 鼠标拖选                                                   | 终端原生选择(deepx 不接管鼠标)→ 复制走系统剪贴板 |

## 工具

| 工具                                       | 说明                                 | plan | auto | review |
| ------------------------------------------ | ------------------------------------ | ---- | ---- | ------ |
| `Read` / `List` / `Tree` / `Glob` / `Grep` | 文件系统只读                         | ✓    | ✓    | ✓      |
| `Write` / `Update`                         | 写入 / 替换文件内容                  | -    | ✓    | ⏳     |
| `Bash`                                     | 执行 shell 命令                      | -    | ✓    | ⏳     |
| `Search` / `Fetch`                         | Web 搜索 / 抓 URL                    | ✓    | ✓    | ✓      |
| `OCR`                                      | 本地图片识字(PaddleOCR)              | ✓    | ✓    | ✓      |
| `Memory`                                   | 检索本 workspace 历史对话            | ✓    | ✓    | ✓      |
| `LoadSkill`                                | 按名加载用户预定义的 `SKILL.md` 正文 | ✓    | ✓    | ✓      |
| `SwitchModel`                              | LLM 主动升级到 pro 模型              | ✓    | ✓    | ✓      |
| `CreatePlan` / `UpdatePlanStatus`          | 模型拆 DAG + 自维护状态,deepx 调度   | -    | -    | -      |

> ⏳ = 自动执行,但需人工确认。`Update` 支持两种模式:行号替换 (`start_line`/`end_line`) 和字符串匹配 (`old_string`/`new_string`)。

## Skills 多来源发现

| 范围         | 路径(按扫描顺序,workspace 覆盖 global)                         |
| ------------ | -------------------------------------------------------------- |
| workspace 级 | `<wd>/.deepx/skills/`                                          |
| global 级    | `~/.agents/skills/` → `~/.claude/skills/` → `~/.deepx/skills/` |

workspace 级 skill 在项目仓库内,**可以 `git add` 给团队共享**(或 `.gitignore` 当个人本地)。global 兼容生态 —— 已有 `~/.claude/skills/` 的 Claude Code 用户直接复用。

改完 SKILL.md 需重启 deepx(catalog 启动时构造,进程内不刷新)。

## 架构

```
~/.deepx/
├── model.yaml                    flash + pro 双模型配置
├── skills/                       global skill 库
└── sessions/
    └── <sha1(workspace)[:16]>/
        ├── meta.json             工作区元信息
        ├── state.json            压缩状态(summary + total_turns)
        └── YYYY-MM-DD.jsonl      只存 user/assistant 主对话(含模式通知)

<workspace>/.deepx/skills/        项目级 skill 库(可 git 提交)
```

**单轮对话流程**:

```
用户输入
  ↓
RouteByKeyword (本地, 零延迟) ──► flash 或 pro
  ↓
StartStream (主对话循环)
  ├─ 直接答 → 完成
  ├─ 调工具 → review 模式拦截写操作 → Executor → 结果回灌 → 继续
  ├─ SwitchModel → flash 升 pro,本轮剩余用 pro
  └─ CreatePlan → Plan DAG scheduler
                      ├─ subagent A (flash) ┐
                      ├─ subagent B (flash) ├─ 按依赖并发
                      └─ subagent C (pro)   ┘  ← depends_on A,B
                                  ↓
                            汇总回主对话
```

**会话压缩流程**:

```
StreamDoneMsg → estimateTokens ≥ ctxWindow × 70% 触发
  ↓
runCompression (异步 goroutine,非流式 CallOnce)
  ├─ 尾部分层保留 ~20K tokens 上下文
  ├─ 旧摘要注入 [previous summary] → LLM 合并连贯摘要
  └─ 结果回主 goroutine:
       history[0] = 新摘要, state.json 重置 total_turns
```

**Token 经济设计**:

- `Memory` / `LoadSkill` 不预注入,只在 LLM 调时才进 context
- system prompt 极简(只放跨工具规约 + workspace),每个工具的触发条件只写在自己的 description 里,不在 system 重复
- DeepSeek KV cache 友好:modeNotification 措辞统一写入 JSONL,重启 history 字节级一致
- 显示仅保留最近 10 轮,markdown 渲染缓存避免滚动重渲

## 项目结构

```
deepx/
├── main.go
├── config/                  ~/.deepx/model.yaml 读写
├── agent/
│   ├── llm.go               StartStream 工具循环 + SwitchModel/Review 拦截
│   ├── scheduler.go         Plan DAG 并发调度
│   ├── subagent.go          子 agent
│   └── keyword_router.go    本地关键词路由
├── tools/                   全部工具实现
├── session/                 ~/.deepx/sessions/ 持久化+压缩状态
├── skill/                   多路径 skill 发现与加载
├── ocr/                     PaddleOCR 包装
└── tui/                     bubbletea TUI
```

## 发布

推 `vX.Y.Z` tag → GitHub Actions 自动跑 goreleaser → 5 平台二进制 + checksums.txt + Release。

```bash
git tag v0.1.0 && git push origin v0.1.0
```

本地预演(不发布):

```bash
goreleaser release --snapshot --clean --skip=publish
```

产物在 `./dist/`。

## 已知限制

- macOS Terminal.app + Apple Color Emoji 字体:emoji cell 物理宽度 ≠ Unicode 规范的 2 cell,会让 chat 区含 emoji 行 scrollbar / 分隔线偶尔 ±1 cell 偏移。整个 charm 生态在 macOS 上都有这个现象。换 iTerm2 / Ghostty / kitty 消除
- Ctrl+A 全选不可用:macOS 终端在 GUI 层拦截 Cmd+A 不送 PTY,任何 TUI 都不能用 Cmd+A;用 Ctrl+Shift+A 替代

## License

[MIT](LICENSE) © 2026 itmisx

## 致谢

- Charm 团队的 bubbletea / lipgloss / glamour 生态
- DeepSeek 提供低成本高质量的 chat completions
- PaddleOCR 提供离线 OCR
