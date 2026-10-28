# JDOTOKEN 应用目录

公开入口 `/agent-market`，顶部导航在“排行榜”之前显示“应用”。样式参考用户提供的 `https://moyu.cn/agent-market` 的卡片目录结构，页面在本站渲染，不加载或嵌入对方站点。沿用 JDOTOKEN 首页配色、Logo、公共导航、主题/语言切换及公司备案页脚。

## 当前范围

本次实现应用展示和入口导航，不包含新的生成工作台后端或多智能体执行服务。用户尚未指定应用清单和第三方地址，因此初版不加入未确认的合作方或外链。

- AI 对话：进入现有 `/playground`，未登录时继续由上游路由引导登录，并保留返回地址。
- 视频创作、创作素材库：操作页面标为“即将上线”，同时提供本站 `/docs/#video-workflow` 和 `/docs/#asset-mode` 接入入口。
- 图片生成、提示词助手、智能体工作台：展示规划入口，按钮禁用且标明“即将上线”。未宣称这些操作页面已经实现。

应用数据位于 `catalog.ts`，卡片在 `index.tsx` 渲染。新增或开放应用时更新文案、目的地址与相应测试；先验证实际工作台可用，不仅替换禁用按钮。未增加用户数据存储或付费请求。

## 组件复用与升级边界

- 复用 `PublicLayout`、`Card` 系列、`Badge`、`Button` 和 `jdtoken-home/HomeFooter`，沿用首页 CSS Module 的配色与品牌资源。
- 现有 `ModelShowcase` 是模型/价格目录，公共 `Search` 为控制台命令面板，不能直接承载应用入口。本模块只新增应用业务内容与局部网格样式，不重复实现导航、主题切换、登录或弹窗。
- 唯一既有业务代码改动：`hooks/use-top-nav-links.ts` 增加 5 行，用现有 `VITE_HOME_VARIANT` 开关在排行榜之前插入入口。`upstream` 构建不添加该入口，直接访问该路径重定向首页。
- 新增 TanStack 文件路由；`routeTree.gen.ts` 由构建插件自动生成。七个语言 JSON 仅追加新键，不修改原有翻译。
- 无 Go、数据库、计费、鉴权实现、渠道配置、全局样式或依赖变更。后续上游合并时检查导航 hook、公共布局/按钮 API、路由生成与首页构建开关；正常执行路由生成、相关测试和构建即可，不需数据库迁移。
- 当前导航没有增加后台开关；如后续需要管理员配置应用或控制可见性，应独立设计配置契约，不把前端按钮禁用当作权限控制。

## 验证

```bash
cd web
bun run test src/features/jdtoken-apps/__tests__/applications.test.tsx src/features/jdtoken-home/__tests__/homepage.test.tsx
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/jdtoken-apps src/routes/agent-market src/hooks/use-top-nav-links.ts
bunx oxfmt --check src/features/jdtoken-apps src/routes/agent-market src/hooks/use-top-nav-links.ts
bun run build
bun run preview --host 127.0.0.1 --port 4190
# 单独终端；使用明确的本地 API fixtures，无生产修改或付费生成
python3 src/features/jdtoken-apps/__tests__/browser-check.py --url http://127.0.0.1:4190 --output /tmp/jdtoken-apps-preview
```

浏览器检查涵盖首页/手机菜单顺序、导航高亮、禁用入口、文档地址、键盘启动、登录返回地址、直接刷新、1440/1024/768/390/320px 栅格与溢出、暗色截图、Logo/备案和无外站加载。工作区截图及证据见 `reports/application-market-20261028/`。新增菜单和页面仍需按项目 Git tag 流程发布后才在生产生效。
