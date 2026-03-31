# MavenRSS (fork from [MrRSS](https://github.com/WCY-dt/MrRSS))

<p>
   <a href="README.md">English</a> | <strong>简体中文</strong>
</p>

[!\[Version\](https://img.shields.io/badge/version-1.3.20-blue.svg null)](https://github.com/WCY-dt/MavenRSS/releases)
[!\[License\](https://img.shields.io/badge/license-GPLv3-green.svg null)](LICENSE)
[!\[Go\](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go null)](https://go.dev/)
[!\[Wails\](https://img.shields.io/badge/Wails-v3%20alpha-red null)](https://wails.io/)
[!\[Vue.js\](https://img.shields.io/badge/Vue.js-3.5+-4FC08D?logo=vue.js null)](https://vuejs.org/)

## ✨ 功能特性

- 🌐 **网页与桌面端部署**：可选择原生桌面应用（Windows/macOS/Linux）或支持多用户访问的自托管网页服务器
- 🔐 **用户身份认证**：安全的登录/注册系统，基于 JWT 身份认证，支持多租户
- 🌍 **自动翻译与摘要**：自动翻译文章标题与正文，并生成简洁的内容摘要，助你快速获取信息
- 🤖 **AI 增强功能**：集成先进 AI 技术，赋能翻译、摘要、推荐等多种功能，让阅读更智能
- 🔌 **丰富的插件生态**：支持 Obsidian、Notion、FreshRSS、RSSHub 等主流工具集成，轻松扩展功能
- 📡 **多样化订阅方式**：支持 URL、XPath、脚本、Newsletter 等多种订阅源类型，满足不同需求
- 🏭 **自定义脚本与自动化**：内置过滤器与脚本系统，支持高度自定义的自动化流程
- 📱 **移动端友好**：响应式设计，针对移动设备优化，加载更快，用户体验更流畅

## 🧠 AI 增强模式

AI 增强模式会把 MavenRSS 从“订阅阅读器”升级成“AI 阅读助手”。文章抓取后，系统可以合并相似内容为文章簇、随着阅读行为持续优化推荐结果，并且每日根据大模型多维度打分生成 Top-10 推荐文章。最终效果是：更快读懂文章、更少被重复信息打扰，也更容易看到真正符合自己兴趣的内容。

### 核心 AI 特性

- **文章去重与聚类**：借助 **SimHash** 和 **sqlite-vec**，系统会把相似文章归并到同一个 **Cluster** 中，减少重复阅读，让持续追踪同一话题更轻松。
- **基于用户兴趣的文章排序**：系统会根据点击、深度阅读与收藏行为持续更新用户兴趣向量。
- **AI 每日推荐**：每日推荐并不只依赖向量相似度，而是先对候选摘要进行分组锦标赛筛选，再对全文做多维度评分，最终选出Top 10。
- **自动化 AI 工作流**：启用后，新内容可以自动经过摘要、翻译、embedding、聚类、融合、重排序、每日推荐调度等环节，尽量减少手动操作。

### 架构流程

```mermaid
flowchart TD
    A["新文章接入"] --> B["缓存文章正文"]
    B --> C["执行规则引擎"]
    C --> D{"满足 AI 增强模式 \n 前置条件?"}
    D -->|否| E["跳过 AI 流程"]
    D -->|是| F{"文章已有摘要?"}
    F -->|否| G["生成 AI 摘要"]
    F -->|是| H["保留现有摘要"]
    G --> I{"订阅源启用 \n translate_articles?"}
    H --> I
    I -->|是| J["翻译文章正文"]
    I -->|否| K["跳过翻译"]
    J --> L["生成文章 Embedding"]
    K --> L
    L --> M["阶段 1: SimHash \n 摘要文本, 海明距离 ≤ 3"]
    M -->|匹配| N["并入现有 Cluster"]
    M -->|不匹配| O["阶段 2: sqlite-vec ANN \n 余弦距离 ≤ 0.15"]
    O -->|匹配| N
    O -->|不匹配| P["创建新 Cluster"]
    N --> Q["簇状态标记为 pending_merge"]
    P --> Q
    Q --> R["执行融合 / 回退拷贝"]
    R --> S["簇状态标记为 pending_embed"]
    S --> T["生成簇级 Embedding"]
    T --> U["簇状态标记为 complete"]
    U --> V["收集用户反馈 \n 点击 / 深读 / 收藏"]
    V --> W["更新用户兴趣向量"]
    W --> X["按向量相似度召回 \n 最近完成的 Cluster"]
    X --> Y["结合时间衰减重排序"]
    Y --> Z["个性化 Cluster Feed"]
    U --> AA["等待异步 AI 工作排空"]
    AA --> AB{"需要补齐缺失日期 \n 或到达定时执行窗口?"}
    AB -->|否| AC["结束当前 AI 周期"]
    AB -->|是| AD["排队生成 AI 每日推荐"]
    AD --> AE["按兴趣向量或时间顺序 \n 召回候选 Cluster"]
    AE --> AF{"推荐专用 AI 配置可用?"}
    AF -->|否| AG["规则重排 \n 召回分 + 新鲜度"]
    AF -->|是| AH["阶段 1：分组锦标赛 \n 从摘要筛出候选"]
    AH --> AI["阶段 2：全文多维打分 \n 信息密度 / 实用性 / 趣味性 / 时效性"]
    AI --> AJ["确定 Top 10 推荐"]
    AG --> AJ
    AJ --> AK["写入每日推荐结果 \n 与推荐分数"]
    AK --> AL["通过日期/列表 API 对外提供"]
    AL --> AM["每日推荐视图展示"]
```

## 🚀 快速开始

### 部署选项

MavenRSS 提供三种部署方式：

#### 选项 1：桌面应用（个人使用推荐）

从上游仓库的 [Releases](https://github.com/WCY-dt/MrRSS/releases/latest) 页面下载适合您平台的最新安装包。

#### 选项 2：网页服务器（团队/共享使用推荐）

将 MavenRSS 部署为网页服务器，支持多用户访问。

##### 使用 Docker（推荐）

```bash
# 使用 Docker Compose 启动
docker-compose up -d

# 或直接使用 Docker
docker run -d -p 1234:1234 \
  -v mavenrss-data:/app/data \
  --name mavenrss-server \
  ghcr.io/tdroseval/mavenrss:latest
```

在 `http://localhost:1234` 访问网页界面

##### 配置

以下环境变量可用于配置：

- `MRRSS_JWT_SECRET`：JWT 令牌的密钥（生产环境必需）
- `MRRSS_ADMIN_USERNAME`：管理员用户名
- `MRRSS_ADMIN_EMAIL`：管理员邮箱
- `MRRSS_ADMIN_PASSWORD`：管理员密码
- `MRRSS_TEMPLATE_USERNAME`：模板用户用户名
- `MRRSS_TEMPLATE_EMAIL`：模板用户邮箱
- `MRRSS_TEMPLATE_PASSWORD`：模板用户密码

#### 选项 3：从源码构建（桌面版）

<details>

<summary>点击展开源码构建指南</summary>

<div markdown="1">

##### 前置要求

在开始之前，请确保已安装以下环境：

- [Go](https://go.dev/) (1.25 或更高版本)
- [Node.js](https://nodejs.org/) (20 LTS 或更高版本，带 npm)
- [Wails v3](https://v3alpha.wails.io/getting-started/installation/) CLI

**平台特定要求：**

- **Linux**: GTK3、WebKit2GTK 4.1、libsoup 3.0、GCC、pkg-config
- **Windows**: MinGW-w64（用于 CGO 支持）、NSIS（用于安装包）
- **macOS**: Xcode 命令行工具

详细安装说明请参见[构建要求](docs/BUILD_REQUIREMENTS.md)

```bash
# Linux 快速设置（Ubuntu 24.04+）：
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev libsoup-3.0-dev gcc pkg-config
```

##### 安装步骤

1. **克隆仓库**
   ```bash
   git clone https://github.com/TdRoseval/MavenRSS.git
   cd MavenRSS
   ```
2. **安装前端依赖**
   ```bash
   cd frontend
   npm install
   cd ..
   ```
3. **安装 Wails v3 CLI**
   ```bash
   go install github.com/wailsapp/wails/v3/cmd/wails3@latest
   ```
4. **构建应用**
   ```bash
   # 使用 Task（推荐）
   task build

   # 或使用 Makefile
   make build

   # 或直接使用 wails3
   wails3 build
   ```
   可执行文件将在 `build/bin` 目录下生成。
5. **运行应用**
   - Windows: `build/bin/MavenRSS.exe`
   - macOS: `build/bin/MavenRSS.app`
   - Linux: `build/bin/MavenRSS`

</div>

</details>

### 数据存储

<details>

<summary>点击展开数据存储说明</summary>

<div markdown="1">

**桌面应用：**

- **正常模式**（默认）：
  - **Windows:** `%APPDATA%\MavenRSS\` (例如 `C:\Users\YourName\AppData\Roaming\MavenRSS\`)
  - **macOS:** `~/Library/Application Support/MavenRSS/`
  - **Linux:** `~/.local/share/MavenRSS/`
- **便携模式**（当 `portable.txt` 文件存在时）：
  - 所有数据存储在 `data/` 文件夹中

**网页服务器：**

- 所有数据存储在 Docker 卷或配置的数据目录中

这确保了您的数据在应用更新和重新安装时得以保留。

</div>

</details>

## 🛠️ 开发指南

<details>

<summary>点击展开开发指南</summary>

<div markdown="1">

### 开发模式运行

启动带有热重载的应用：

```bash
# 使用 Wails v3
wails3 dev

# 或使用 Task
task dev
```

### 代码质量工具

#### 使用 Make

我们提供了 `Makefile` 来处理常见的开发任务（在 Linux/macOS/Windows 上都可用）：

```bash
# 显示所有可用命令
make help

# 运行完整检查（lint + 测试 + 构建）
make check

# 清理构建产物
make clean

# 设置开发环境
make setup
```

### Pre-commit Hooks

本项目使用 pre-commit hooks 来确保代码质量：

```bash
# 安装 hooks
pre-commit install

# 在所有文件上运行
pre-commit run --all-files
```

### 运行测试

```bash
make test
```

</div>

</details>

## 📝 许可证

本项目采用 GPL-3.0 许可证 - 详情请参阅 [LICENSE](LICENSE) 文件。

***

<div align="center">
  <p>Made by AI</p>
  <p>⭐ 如果您觉得这个项目有用，请在 GitHub 上给我们点星！</p>
</div>
