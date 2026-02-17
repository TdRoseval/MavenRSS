# MrRSS 性能优化指南

本文档记录了 MrRSS 项目的性能优化策略和实施细节。

## 已实施的优化

### 1. 前端代码优化

#### 1.1 代码分割 (Code Splitting)
**文件**: `frontend/vite.config.js`

- 将依赖分为三个 chunk：
  - `vendor`: vue, pinia, vue-i18n
  - `ui`: @phosphor-icons/vue, tailwindcss
  - `utils`: highlight.js, katex

- 启用预构建优化 (`optimizeDeps`)
- 降低 chunk 警告阈值到 1000KB

#### 1.2 组件异步加载
**文件**: `frontend/src/App.vue`

将所有模态框组件改为异步加载：
- AddFeedModal
- EditFeedModal
- SettingsModal
- DiscoverFeedsModal
- UpdateAvailableDialog
- ContextMenu
- ConfirmDialog
- InputDialog
- MultiSelectDialog

这些组件只在需要时才会加载，显著减少初始加载体积。

#### 1.3 资源加载优化
**文件**: `frontend/index.html`

- 添加 `dns-prefetch` 预解析 Google Fonts
- 使用 `preload` 预加载字体，避免阻塞渲染
- 添加 `noscript` 降级方案
- 移除阻塞加载的外部脚本

### 2. 服务器端优化

#### 2.1 Gzip/Brotli 压缩
**文件**: `internal/middleware/compress.go`

- 支持 gzip 和 brotli 两种压缩方式
- 自动检测客户端支持的压缩格式
- 只压缩大于 1KB 的文件
- 使用对象池优化性能
- 支持的内容类型：
  - text/html, text/css, text/plain
  - text/javascript, application/javascript
  - application/json, application/xml
  - image/svg+xml

#### 2.2 静态资源缓存策略
**文件**: `main-core.go`

- **index.html**: 不缓存 (`no-cache, no-store, must-revalidate`)
- **/assets/**: 永久缓存 (`max-age=31536000, immutable`)
- **CSS/JS**: 24 小时缓存 (`max-age=86400`)
- **图片**: 7 天缓存 (`max-age=604800`)
- **字体**: 永久缓存 (`max-age=31536000, immutable`)
- **默认**: 1 小时缓存 (`max-age=3600`)

- 添加 ETag 支持（基于文件修改时间和大小）
- 添加 Last-Modified 头部
- 添加安全头部：
  - X-Content-Type-Options: nosniff
  - X-Frame-Options: DENY
  - X-XSS-Protection: 1; mode=block

#### 2.3 路由配置更新
**文件**: `internal/routes/routes.go`

- 添加 `EnableCompression` 配置选项
- 服务器模式默认启用压缩

### 3. Service Worker 离线缓存

#### 3.1 Service Worker 实现
**文件**: `frontend/public/sw.js`

- 预缓存核心资源
- Cache-First 策略：优先从缓存读取，后台更新
- Network-First for API：优先网络请求，失败时回退到缓存
- 离线支持：导航请求失败时显示离线页面

#### 3.2 Service Worker 注册
**文件**: `frontend/src/utils/serviceWorker.ts`

- 自动检测本地环境
- 支持更新通知
- 生产环境自动注册

#### 3.3 集成到主入口
**文件**: `frontend/src/main.ts`

- 非本地环境自动注册 Service Worker
- 提供成功和更新回调

### 4. 图片加载优化

#### 4.1 图片优化 Composable
**文件**: `frontend/src/composables/ui/useImageOptimization.ts`

- 图片懒加载（使用 IntersectionObserver）
- 图片缓存去重
- 预加载支持
- 占位符支持框架
- 100px 预加载距离

## 性能审计指南

### 使用 Lighthouse 进行审计

1. **安装 Lighthouse**
   ```bash
   npm install -g lighthouse
   ```

2. **运行审计**
   ```bash
   lighthouse http://localhost:1234 --view
   ```

3. **关键指标**
   - **First Contentful Paint (FCP)**: < 1.8s
   - **Largest Contentful Paint (LCP)**: < 2.5s
   - **Time to Interactive (TTI)**: < 3.8s
   - **Total Blocking Time (TBT)**: < 200ms
   - **Cumulative Layout Shift (CLS)**: < 0.1

### 使用 Chrome DevTools

1. **Performance 面板**
   - 记录页面加载
   - 分析 Main Thread 活动
   - 识别长任务

2. **Network 面板**
   - 启用 Disable cache
   - 查看资源加载瀑布图
   - 检查压缩和缓存状态

3. **Lighthouse 面板**
   - 直接在 DevTools 中运行
   - 获取详细的优化建议

## 进一步优化建议

### 1. 图片优化
- [ ] 实现 WebP/AVIF 格式支持
- [ ] 自动生成响应式图片（srcset）
- [ ] 添加图片 CDN 支持
- [ ] 实现渐进式图片加载
- [ ] 添加图片占位符（blurhash）

### 2. 代码优化
- [ ] 进一步分析和分割大组件
- [ ] 实现路由级别的代码分割
- [ ] 优化第三方库的引入方式
- [ ] 使用 Tree Shaking 移除未使用代码

### 3. 服务器优化
- [ ] 配置 HTTP/2 或 HTTP/3
- [ ] 实现服务器端渲染 (SSR)
- [ ] 添加边缘缓存 (CDN)
- [ ] 优化数据库查询
- [ ] 添加响应压缩级别配置

### 4. 缓存策略
- [ ] 实现更智能的缓存失效策略
- [ ] 添加版本化的 API 缓存
- [ ] 实现离线优先的数据同步
- [ ] 添加 IndexedDB 本地存储

### 5. 用户体验
- [ ] 添加骨架屏 (Skeleton Screens)
- [ ] 实现渐进式 Web 应用 (PWA)
- [ ] 添加加载状态指示器
- [ ] 优化首屏渲染时间

## 构建和部署最佳实践

### 构建优化
```bash
cd frontend
npm run build
```

### 服务器模式启动
```bash
go build -tags server -o mrrss-server
./mrrss-server --host 0.0.0.0 --port 1234
```

### Docker 部署
使用项目提供的 Dockerfile 进行部署，确保：
- 使用多阶段构建
- 压缩构建产物
- 配置健康检查
- 设置资源限制

## 监控和持续优化

1. **设置性能预算**
   - 定义关键指标的目标值
   - 使用 Lighthouse CI 集成
   - 在 CI/CD 中自动运行性能测试

2. **真实用户监控 (RUM)**
   - 收集真实用户的性能数据
   - 分析不同地区/设备的表现
   - 设置性能告警

3. **A/B 测试**
   - 测试不同优化策略的效果
   - 基于数据做出优化决策

## 参考文献

- [Web Vitals](https://web.dev/vitals/)
- [Lighthouse Documentation](https://developer.chrome.com/docs/lighthouse/overview/)
- [Vue.js Performance Guide](https://vuejs.org/guide/best-practices/performance.html)
- [Vite Build Optimizations](https://vitejs.dev/guide/build.html)
- [Service Worker API](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API)
