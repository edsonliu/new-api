# JDOTOKEN 首页定制与上游升级

本模块把原 `https://jdotoken.com/portal` 的宣传首页迁入 new-api，沿用蓝白视觉和横幅素材；模型目录、登录、Key、钱包、控制台及体验功能使用 new-api 自身能力。

## 与上游的边界

- 唯一业务代码接入点：`web/src/features/home/index.tsx` 的 lazy import 和默认首页分支（位于 `HomePageContent` 处理之后）。原版首页组件和自定义 HTML / Markdown / URL 的优先级不变。
- 构建接入点：`web/rsbuild.config.ts` 的 `source.define` 仅注入 `VITE_HOME_VARIANT`。Rsbuild 不会自动把所有 `VITE_` 变量暴露给浏览器，因此这一步不能省略。
- 定制组件、API 适配、CSS Modules 和测试集中在本目录。
- 素材位于 `web/public/jdtoken/`，运行时不访问旧门户域名。
- 文案遵守上游 i18n 约定，仅向七种语言 JSON 添加英文键，不覆盖既有键。
- `/portal` 的兼容入口位于 `web/src/routes/portal.tsx`，跳转到 `/` 并保留查询参数与锚点。`routeTree.gen.ts` 由路由插件生成，不手工编辑。
- 不修改 Go 后端、数据库、计费、认证流程、公共组件、全局样式或依赖版本。复用公共布局、按钮、状态提示及代码块；首页使用本模块的品牌页脚，源码版权、许可证和上游公共页脚不变。

## 构建开关

默认构建启用 JDOTOKEN 首页。后台 `HomePageContent` 非空时仍优先展示管理员设置的内容。

```bash
cd web
bun install --frozen-lockfile
bun run build

# 构建上游原版首页，不必撤销定制代码
VITE_HOME_VARIANT=upstream bun run build
```

`VITE_HOME_VARIANT` 是构建时开关，修改已运行容器的环境变量不会改变已有前端产物。Dockerfile 会在镜像内部构建前端；默认直接构建自有镜像即可获得 JDOTOKEN 首页。如果要在 Docker 构建中切回上游，可在构建上下文的 `web/.env.production.local` 设置 `VITE_HOME_VARIANT=upstream`（本地配置不要提交），再构建镜像。

```bash
# 仓库根目录，使用自己的镜像标签；不要覆盖正在运行的生产镜像
# Docker 构建还包含整个 Go 后端，本次改造只验证了前端构建。
docker build -t jdtoken-new-api:<your-version> .
```

## 数据与导航

- `catalog.ts` 是本模块唯一的模型 API 适配点。复用 `/api/pricing` 请求及类型，不复制价格换算或结算逻辑。
- 遵守 `/api/status` 的 `HeaderNavModules.pricing`；禁用时隐藏模型展示，需要登录时不匿名请求、不展示缓存中的目录。缓存按用户 ID 和分组隔离。
- 首页取目录返回顺序中的前八个模型。展示介绍和详情入口；准确价格由原版模型详情页提供。
- 模型详情接 `/pricing/$modelId`，控制台 `/dashboard`，Key `/keys`，订阅/余额 `/wallet`，体验 `/playground`。
- 开始使用按钮遵守 `register_enabled`；已登录时进入控制台，未开放注册时进入登录页。
- 导航栏沿用上游配置及行为。文档地址应在站点设置中配置；旧热点提示词和旧套餐页面没有作为外链带入。
- SDK 示例优先使用站点配置 `server_address`，为空时使用当前域名，加 `/v1`。仅从支持 `openai` 端点的实际目录模型选示例；没有候选时显示明确的 `YOUR_MODEL_ID` 占位提示。密钥始终从本机环境变量读取。
- 原页面未经核实的调用量、SLA、模型数量和折扣宣传已替换为功能介绍。
- 本模块不导入旧模型、渠道、账号、钱包或订单数据。目标后端没有模型时显示空状态，不伪造可调用模型。

## 跟随上游升级

基线：`c6741c36a`。建议维护自己的 Git 分支与镜像仓库，在升级分支合并选定的上游版本，不直接用官方镜像覆盖定制产物。

1. 拉取上游，建立升级分支；合并选定 tag 或提交。
2. 如果 `features/home/index.tsx` 冲突，保留上游完整逻辑，在管理员自定义内容分支之后接入定制首页。
3. 核对本模块的外部依赖：`getPricing` / `PricingModel`、`useStatus`、`parseHeaderNavModulesFromStatus`、`useAuthStore`、公共布局/代码块、用户协议及隐私政策状态开关和现有页面路由。
   同时保留构建配置中的 `VITE_HOME_VARIANT` 定义，并分别验收默认构建和 `upstream` 构建。
4. 重新生成路由树（Rsbuild dev/build 会生成），不要手动合并生成代码。
5. 运行下列检查，并验证生产构建。Git 无冲突并不代表接口与页面兼容。
6. 用自己的 Dockerfile 构建自有镜像，在测试实例验收后发布；保留旧镜像与原数据库版本对应的回滚方案。

```bash
cd web
bun run test src/features/jdtoken-home/__tests__/homepage.test.tsx
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/jdtoken-home src/features/home/index.tsx src/routes/portal.tsx
bunx oxfmt --check src/features/jdtoken-home src/features/home/index.tsx src/routes/portal.tsx
bun run build
```

独立浏览器回归（可选环境依赖：Python 的 playwright 包和 Chromium/Chrome）：

```bash
# 终端一：构建产物预览
bun run preview --host 127.0.0.1 --port 4173
# 终端二：使用固定的接口测试数据，不连接生产，也不调用模型
python src/features/jdtoken-home/__tests__/browser-check.py --url http://127.0.0.1:4173 --output /tmp/jdtoken-preview
```

检查包括桌面/移动端布局、资源完整性、真实路由链接、复制代码、旧首页地址跳转、暗色主题与长模型名称。浏览器截图中的模型属于测试数据，不能用来判断目标实例真实渠道是否可用。

## 已知范围

这是首页迁移，不是旧门户全量重写。视觉内容在 new-api 公共外壳下重建；上游导航、主题、语言仍由公共组件维护，首页品牌页脚在本模块维护。后续升级可能需要少量人工适配，不能承诺零冲突自动升级。

## 首页品牌调整（2026-10-11）

- `home-footer.tsx` 仅用于定制首页，展示 JDOTOKEN 与按后台开关启用的协议入口。公共 `Footer` 无关闭项目归属的 props/插槽，内部法律链接组件也未导出；扩展它会增加上游公共文件合并点。因此采用简单的独立语义化页脚，不修改公共 Footer，不用 CSS 隐藏归属元素。
- 已有构建接入点 `rsbuild.config.ts` 根据 `VITE_HOME_VARIANT` 选用本模块的 `template.html` 与站点图标，设置初始页面标题及 meta title；`upstream` 保持原版行为。
- 运行时站点名使用上游 `SystemName` 配置设为 `JDOTOKEN`；`DocsLink` 设置为 `https://api.jdotoken.com/docs/` 并启用 `HeaderNavModules.docs`。这些站点级配置同时作用于共用它们的其他页面。
- 不改变后端、数据库结构、依赖或认证/计费逻辑。升级时核验原有首页入口、构建品牌分支和法律链接状态字段；上游 HTML 入口增加必要脚本/安全配置时，将相关变化同步到本模块模板。

## 企业与备案展示（2026-10-11）

首页 `home-footer.tsx` 展示用户提供的公司全称 `及第嘉序(北京)智算科技有限公司` 与备案号 `京ICP备2026065372号-1`，备案号链接至 `https://beian.miit.gov.cn/`。公司法定名称和备案编号在各语言下保持原文。仅调整独立首页模块，不新增上游文件接入点，不修改后台、数据库或公共页脚。现有浏览器回归覆盖展示、备案链接与移动端换行。

## 官方品牌与公安备案（2026-10-12）

用户提供的新品牌位于工作区 `heguixinx/`。首页页脚使用横向 Logo 的常规/反白版本，顶部使用紧凑的上下布局；公安备案号 `京公网安备11010602203425号` 与原 ICP 编号同时展示，链接到公安机关备案查询页。浏览器回归覆盖图片加载、备案链接、暗色版本及窄屏布局。

版本化 `mark-square-20261012.png` 避免旧图片一周缓存；复用既有 Rsbuild favicon 接入点与后台 `Logo` 选项，不修改公共 Header/Footer。没有新增上游核心接入点、依赖或数据库变更。升级时保留品牌资源和独立首页模块，回归 favicon、Logo 配置、备案链接及移动端。


## 2026-10-12 品牌名称与 Logo 比例

当前面向用户的英文品牌统一为 `JDOTOKEN`，包括七种语言中的产品名、首页、HTML 标题和独立接入文档；内部目录、构建开关与容器名继续沿用 `jdtoken`，避免破坏部署兼容性。示例客户端自定义变量使用 `JDOTOKEN_API_KEY` / `JDOTOKEN_TASK_ID`，网关接口和鉴权格式不变。

`mark-square-20261012.png` 将原始上下组合 Logo 等比缩放到 416×272，居中放入 512×512 透明画布，为上游圆形、方形图标容器保留安全边距。登录页和控制台的 `object-cover` 因图片本身为正方形不再裁掉左右部分；首页与文档页脚的横版 Logo 仍按原比例显示。后台 `Logo` 与构建 favicon 均指向新资源路径，避免旧缓存。

本次不新增上游核心文件修改；仍依赖现有 Rsbuild 品牌分支、七种语言字典和 `SystemName` / `Logo` 选项。后续升级保留这些接入点，回归登录、注册、控制台和移动端 Logo；无需数据库结构迁移。发布前备份配置和数据库，回滚需同时恢复镜像、文档软链接、`SystemName` 与 `Logo`。
