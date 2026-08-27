# 前端开发约定

本文件适用于 `frontend/` 目录，并继承根目录 `AGENTS.md` 的项目级约定。

## 运行与验证

前端要求 Node.js 24.0.0 或更高版本。命令仍从仓库根目录执行。

```bash
# 桌面端开发
wails3 task dev

# 移动端依赖与运行
wails3 task ios:install:deps
wails3 task android:install:deps
wails3 task ios:run
wails3 task android:run
wails3 task android:run:device

# 绑定与前端生产构建
wails3 generate bindings -clean=true -ts -i
wails3 task common:build:frontend
```

桌面端启动后，桌面 MCP 按 Wails3 默认配置可用。

## 代码组织

```text
frontend/
├── bindings/                       # Wails 自动生成的 TypeScript 绑定
└── src/
    ├── api/                        # 认证与按业务域拆分的绑定调用和边界归一化
    ├── apps/
    │   ├── shared-app-routes.tsx   # Web 与桌面端共用的业务路由
    │   ├── web/                    # Web 应用入口和路由
    │   ├── desktop/                # 桌面端应用入口和路由
    │   └── mobile/                 # 移动端独立入口、路由和页面
    ├── components/                 # 跨 feature 共享的展示组件
    │   ├── form/                   # 通用表单展示组件
    │   └── ui/                     # 基础 UI 组件
    ├── contexts/                   # 跨 feature 共享的 React 上下文
    ├── features/                   # Web 与桌面端共用的业务功能，按业务域分目录
    ├── hooks/                      # 通用 hooks，含统一数据读取 useResource
    ├── i18n/                       # 国际化资源，按语言目录和 namespace 分文件
    ├── lib/                        # 通用纯函数工具
    └── platform/                   # 运行平台识别
```

- `src/api` 按业务域一个文件组织，页面统一从 `@/api` 聚合入口导入；共享归一化工具放 `api/normalize.ts`。
- 页面归其路由所属的 feature；被多个 feature 使用的展示组件放 `src/components`，被多个 feature 使用的上下文放 `src/contexts`，feature 私有的上下文和 hooks 留在各自目录内。features 之间不得形成循环依赖；`features/workspace` 作为路由中枢可以引用各 feature 的页面，其余 feature 不得反向引用 workspace。
- 一个例外：角色域的页面在 `features/roles`，由设置页外壳（`features/settings`）按路由引用；该依赖保持 settings → roles 单向。

## 业务契约

- `appservice` 契约是前端业务 DTO 的唯一来源。前端不得重复声明渠道、联系人、用户、收件箱、设置等业务模型和枚举，也不要提交 Wails `$zero`。
- `frontend/bindings` 使用 `wails3 generate bindings -clean=true -ts -i` 生成，禁止手工修改，也不得用不同格式覆盖。
- 页面只通过 `src/api` 调用绑定：`client` 注入认证与错误，`service` 绑定方法并归一化可空切片。页面不直接引用 `frontend/bindings`。
- 前端只保留表单值、组件 Props、页面状态、查询参数派生类型，以及对生成类型中可空切片的边界归一化类型。
- 页面卸载时忽略过期结果，不要取消 Wails 绑定调用。

## 数据读取

- 页面数据读取统一使用 `src/hooks/use-resource.ts` 的 `useResource`（基于 TanStack Query），不再手写 `useEffect` 加过期标志的取数样板。查询 key 统一在 `src/hooks/resource-keys.ts` 的 `resourceKeys` 工厂中定义，页面不得手写 key 数组；同一份后端数据在不同页面使用相同 key 以共享缓存，查询参数变化必须体现在 key 中。
- 读取错误由 `useResource` 统一做会话入口恢复；变更操作直接调用 `@/api`，成功后通过 `refresh` 或 `useResourceInvalidator` 失效相关 key，不手工修补缓存数据。
- 会话引导流程（启动探测、身份加载）保持独立实现，不强制走 `useResource`。

## 路由

- `react-router` 在 `package.json` 中锁定精确版本。其 `UNSAFE_` 内部 API 只允许出现在 `src/features/workspace/tab-scoped-router.tsx`；升级 react-router 前必须先验证该模块行为未变化。

## 表单

- 表单使用 React Hook Form 和 Zod，并统一启用 `shouldUseNativeValidation`。客户端字段校验由浏览器显示在校验失败的输入控件上，不在字段下方渲染 `FieldError`，也不同时弹出 Toast；服务端业务错误通过 Toast 展示，不使用 `setError` 回写字段。
- 桌面端 WebView 中，带 `legend` 的原生 `fieldset`（包括 `FieldSet`）不得作为 flex 容器的直接子项，避免 WebKit 首次布局保留额外高度。此类表单组使用单列 grid，或在 `fieldset` 外增加普通块级容器；不得依赖点击、窗口缩放等重绘行为恢复布局。
- 输入框不使用 placeholder；字段含义由标签表达，必要说明用字段帮助文案。
- 页面表单将字段区与底部操作区分为同级布局区域；保存、取消、测试等操作所在区域与最后一个表单项的垂直间距固定为 36px，统一使用 `space-y-9`。操作区不得放入 `FieldGroup`，也不得叠加额外外边距改变该间距。

## 注释

- `src` 业务代码的文件头说明职责，具名函数、组件和导出函数各使用一行简洁、直述型中文注释。
- `bindings` 禁止手工添加注释；`components/ui` 只保留文件头。

## 界面控制

- 当前任务涉及桌面端时，必要时应主动使用 Wails MCP 获取页面信息并完成相关验证，无需另行请求授权。
- 除桌面端任务中的 Wails MCP 验证外，未经用户当次明确授权，不得控制浏览器、桌面应用或系统界面。
- 默认使用命令行验证；需要界面验证时，由用户操作并反馈结果，授权不得跨任务沿用。

## 管理界面设计

- 管理页面采用左对齐的可用宽度布局并保持统一留白。工作台一级导航为固定窄轨，二级栏显示模块标题（消息页除外）。不使用面包屑；需要扩展的设置页和渠道编辑页使用与 URL 同步的页签，不展示空页签。
- 新增、编辑和设置等表单页的标题栏不展示「返回」或其他返回操作；用户通过底部「取消」、页签、二级导航或导航历史离开。非表单详情子页确需显式返回列表时，在标题行右上角放置文字「返回」。
- 设置页同一分组内的字段保持统一的表单行样式；权限状态等字段不得单独使用带边框、圆角和内边距的卡片包裹，应与相邻字段的标签、帮助文案和控件对齐。
- 数据列表使用带表头的表格，字段独立成列且只展示有管理价值的信息，避免卡片式字段聚合。
- 操作按主次排序；主要操作直接展示，低频或危险操作收进三点菜单，危险操作执行前确认。
- 列表操作列中直接展示的详情、编辑、恢复等文字操作统一使用小尺寸描边按钮。
- 操作按钮以文字为主，不添加装饰性图标；图标仅用于导航、类型、状态和三点菜单。
- 界面文案保持简洁，面向最终使用者描述操作、结果和影响；只保留必要的标题、标签、校验、状态和风险提示，不用解释数据结构或实现方式的说明式文案。
