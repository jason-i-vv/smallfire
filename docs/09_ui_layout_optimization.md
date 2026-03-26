# UI布局优化 - 左右布局改造

## 变更日期
2026-03-23

## 变更概述
将前端界面从上下布局改为左右布局，统一将菜单放在左侧，提供更好的用户体验和更高效的空间利用。

## 变更内容

### 1. 新增文件

#### `src/components/common/AppSidebar.vue`
- 左侧导航菜单组件
- 支持可折叠功能（240px → 64px）
- 菜单分组：仪表盘、信号中心、市场分析、交易管理、系统设置
- 使用 Element Plus el-menu 组件
- 包含 logo、菜单导航、折叠按钮

### 2. 修改文件

#### `src/layouts/DefaultLayout.vue`
- 从垂直布局改为水平左右布局
- 集成左侧导航栏（AppSidebar）
- 顶部栏显示：页面标题、系统状态、用户信息
- 移除对 AppHeader 和 AppFooter 的依赖
- 状态信息（系统状态、同步时间）集成到顶部栏右侧

#### `src/assets/styles/variables.scss`
- 新增侧边栏相关样式变量：
  - `$sidebar-width: 240px`
  - `$sidebar-collapsed-width: 64px`
  - `$sidebar-header-height: 64px`
  - `$top-header-height: 64px`

#### `src/assets/styles/global.scss`
- 更新侧边栏菜单样式覆盖
- 保留 app-header 和 app-footer 样式（向后兼容）

#### `docs/04_frontend_dashboard_design.md`
- 更新页面结构说明
- 添加新版左右布局示意图
- 保留旧版上下布局作为参考

### 3. 删除文件

- `src/components/common/AppHeader.vue` - 功能已集成到布局
- `src/components/common/AppFooter.vue` - 功能已集成到布局

## 技术实现

### 布局结构
```
┌─────────┬──────────────────────────────────────┐
│         │  顶部栏（页面标题 + 状态 + 用户信息）     │
│  侧边栏  ├──────────────────────────────────────┤
│ (可折叠) │                                      │
│         │              主内容区                    │
│         │                                      │
│         │                                      │
└─────────┴──────────────────────────────────────┘
```

### 关键代码片段

#### AppSidebar.vue 核心逻辑
```vue
<template>
  <aside class="app-sidebar" :class="{ 'is-collapsed': isCollapsed }">
    <el-menu :collapse="isCollapsed">
      <!-- 菜单项 -->
    </el-menu>
  </aside>
</template>
```

#### DefaultLayout.vue 核心样式
```scss
.default-layout {
  display: flex;
  min-height: 100vh;
}

.main-wrapper {
  flex: 1;
  margin-left: 240px;  // 与侧边栏宽度同步
  transition: margin-left $transition;
}
```

## 测试要点

- [ ] 侧边栏菜单导航功能正常
- [ ] 侧边栏折叠/展开功能正常
- [ ] 菜单高亮显示当前页面
- [ ] 顶部栏显示正确的页面标题
- [ ] 系统状态和同步时间显示正常
- [ ] 用户下拉菜单功能正常
- [ ] 退出登录功能正常
- [ ] 各页面布局适配新布局结构
- [ ] 响应式布局（移动端适配）

## 相关的需求文档
- `docs/04_frontend_dashboard_design.md` - 前端界面设计
