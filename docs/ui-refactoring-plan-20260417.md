# 前端UI重构计划 - 登录页重构

## 1. 概述

### 1.1 目标
重构登录页面，采用 Cursor 风格的左右分栏布局，左侧为动态K线可视化区域，右侧为登录表单。

### 1.2 实施范围（分步实施）
**Phase 1 (当前):** 登录页重构
- 左右分栏布局
- 动态K线可视化背景
- 登录/注册表单重设计
- Google登录按钮
- 语言切换（显示中文/English）

**Phase 2 (后续):** 全局多语言支持
- Vue I18n 集成
- 所有页面翻译

**Phase 3 (后续):** 主题系统
- 橙色主题变量化
- 表单组件统一样式

### 1.3 技术栈
- Vue 3 + Composition API
- Vue Router 4
- Pinia (状态管理)
- Element Plus (基础组件)
- vue-i18n (多语言支持，Phase 1 直接集成)
- Canvas 2D API (K线可视化)

---

## 2. 页面布局规格

### 2.1 桌面端布局 (≥1024px)

```
┌─────────────────────────────────────┬────────────────────────────────────┐
│                                     │                                     │
│   左侧品牌视觉区 (60%)               │   右侧登录表单区 (40%)               │
│   深色背景 #0F172A                   │   白色背景 #FFFFFF                  │
│                                     │                                     │
│   ┌─────────────────────────────┐  │   ┌─────────────────────────────┐  │
│   │                             │  │   │                             │  │
│   │   [动态K线可视化动画]         │  │   │   🔥 Starfire              │  │
│   │                             │  │   │                             │  │
│   │   抽象的K线+成交量柱状图      │  │   │   邮箱/用户名                 │  │
│   │   橙色光效点缀               │  │   │   ┌─────────────────────┐   │  │
│   │                             │  │   │   │                     │   │  │
│   │   底部: 星火量化 tagline     │  │   │   └─────────────────────┘   │  │
│   │   "智能量化，稳健收益"        │  │   │                             │  │
│   │                             │  │   │   密码                     │  │
│   └─────────────────────────────┘  │   │   ┌─────────────────────┐   │  │
│                                    │   │   │                     │   │  │
│                                    │   │   └─────────────────────┘   │  │
│                                    │   │                             │  │
│                                    │   │   [🔵 Sign in] (橙色按钮)    │  │
│                                    │   │                             │  │
│                                    │   │   ───── or ─────            │  │
│                                    │   │                             │  │
│                                    │   │   [G] Continue with Google  │  │
│                                    │   │                             │  │
│                                    │   │   ─────────────────────     │  │
│                                    │   │   还没有账号? 注册          │  │
│                                    │   │                             │  │
│                                    │   │   [EN] / [中文]             │  │
│                                    │   └─────────────────────────────┘  │
└─────────────────────────────────────┴────────────────────────────────────┘
```

### 2.2 移动端布局 (<768px)

```
┌─────────────────────────────────────┐
│  🔥 Starfire                        │
├─────────────────────────────────────┤
│                                     │
│  左侧品牌视觉区 (缩短版)             │
│  高度: 160px                        │
│  静态K线图形（非动画）               │
│                                     │
├─────────────────────────────────────┤
│                                     │
│  右侧登录表单（全宽）                │
│                                     │
│  邮箱输入                           │
│  ┌─────────────────────────────┐   │
│  └─────────────────────────────┘   │
│                                     │
│  密码输入                           │
│  ┌─────────────────────────────┐   │
│  └─────────────────────────────┘   │
│                                     │
│  [🔵 Sign in]                       │
│                                     │
│  [G] Google                         │
│                                     │
│  还没有账号? 注册                    │
│                                     │
│  [EN] / [中文]                      │
│                                     │
└─────────────────────────────────────┘
```

### 2.3 平板端 (768px - 1023px)
- 左右布局保持，比例调整为 50/50
- K线动画保持

---

## 3. 视觉设计规格

### 3.1 色彩系统

```scss
// 登录页专用色彩
$login-primary: #FF6B00;           // 主题橙
$login-primary-light: #FF8A3D;     // 浅橙
$login-primary-dark: #E55A00;       // 深橙
$login-bg-dark: #0F172A;           // 左侧深色背景
$login-bg-dark-gradient: #1E293B;  // 渐变
$login-bg-light: #FFFFFF;          // 右侧白色背景

// 文字
$login-text-primary: #1A1A1A;
$login-text-secondary: #6B7280;
$login-text-on-dark: #F8FAFC;

// 边框
$login-border: #E5E7EB;
$login-border-focus: $login-primary;

// 状态
$login-success: #10B981;
$login-error: #EF4444;
```

### 3.2 字体规格

```scss
// 英文
$font-en: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;

// 中文
$font-zh: 'PingFang SC', 'Microsoft YaHei', sans-serif;

// 字号
$text-xs: 12px;
$text-sm: 14px;
$text-base: 16px;
$text-lg: 18px;
$text-xl: 20px;
$text-2xl: 24px;
$text-3xl: 30px;
```

### 3.3 间距系统

```scss
$space-1: 4px;
$space-2: 8px;
$space-3: 12px;
$space-4: 16px;
$space-5: 20px;
$space-6: 24px;
$space-8: 32px;
$space-10: 40px;
$space-12: 48px;
```

### 3.4 阴影

```scss
$shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.05);
$shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
$shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
$shadow-input-focus: 0 0 0 3px rgba(255, 107, 0, 0.15); // 橙色光晕
```

### 3.5 动画规格

```scss
$transition-fast: 150ms ease;
$transition-normal: 250ms ease;
$transition-slow: 350ms ease;

// K线动画
$kline-duration: 60s;        // 一个完整的K线周期
$kline-particles: 50;        // 粒子数量
```

---

## 4. 组件规格

### 4.1 K线可视化组件 (KlineVisualization)

**功能:**
- Canvas 绘制的抽象K线图
- 橙色的上涨K线 (#FF6B00)
- 灰红色的下跌K线 (#64748B)
- 底部成交量柱状图
- 微妙的发光效果

**Props:**
```typescript
interface KlineVisualizationProps {
  height?: number;
  darkMode?: boolean;  // 左侧深色模式
}
```

**性能考虑:**
- 移动端 (<768px): 使用静态版本
- requestAnimationFrame 优化
- Canvas 而非 SVG

### 4.2 登录表单组件 (LoginForm)

**功能:**
- 邮箱/用户名输入
- 密码输入 (带显示/隐藏切换)
- 表单验证
- Google登录按钮
- 语言切换

**状态:**
| 状态 | 样式 |
|------|------|
| Default | 灰色边框 #E5E7EB |
| Focus | 橙色边框 + 光晕 |
| Error | 红色边框 + 错误提示 |
| Disabled | 半透明 + 不可点击 |

**验证规则:**
```javascript
email: [
  { required: true, message: '请输入邮箱' },
  { type: 'email', message: '请输入有效邮箱' }
],
password: [
  { required: true, message: '请输入密码' },
  { min: 6, message: '密码至少6位' }
]
```

### 4.3 Google登录按钮

**样式:**
- 白色背景
- 灰色边框
- Google logo 左侧
- hover: 轻微阴影

### 4.4 语言切换器

**位置:** 登录表单底部
**样式:** 文字链接风格 [EN] / [中文]
**功能:** 切换显示语言，不刷新页面

---

## 5. 文件结构

```
starfire-frontend/src/
├── views/auth/
│   ├── Login.vue           # 登录页（重构）
│   └── Register.vue        # 注册页（同步重构）
├── layouts/
│   └── AuthLayout.vue      # 重构为左右分栏
├── components/auth/
│   └── KlineVisualization.vue  # 新增：Canvas K线可视化
├── i18n/
│   ├── index.js            # 新增：i18n 配置
│   ├── en.json             # 新增：英文翻译 (login/register)
│   └── zh.json             # 新增：中文翻译 (login/register)
├── stores/
│   └── auth.js            # 更新：添加 locale 状态
└── assets/styles/
    ├── variables.scss      # 更新：橙色主题变量
    └── auth.scss           # 新增：登录页样式
```

**Note:** Google 登录作为 UI 占位符集成在 Login.vue 中，不单独拆分组件。

---

## 6. 实施步骤

### Phase 1: 登录页重构

**Step 1.0: 安装 vue-i18n**
- [ ] `npm install vue-i18n@^9.0.0`
- [ ] 创建 `src/i18n/index.js` 配置
- [ ] 创建 `src/i18n/zh.json` 登录/注册翻译
- [ ] 创建 `src/i18n/en.json` 登录/注册翻译
- [ ] 在 `main.js` 中注册 i18n 插件

**Step 1.1: 创建K线可视化组件 (Canvas)**
- [ ] `src/components/auth/KlineVisualization.vue` - Canvas 2D 绘制
- [ ] requestAnimationFrame 动画循环
- [ ] 响应式 resize 监听
- [ ] 移动端 (<768px) 静态版本

**Step 1.2: 重构AuthLayout**
- [ ] 左右分栏 CSS (60/40)
- [ ] 响应式断点 (768px, 1024px)
- [ ] 移动端堆叠布局

**Step 1.3: 重构Login.vue**
- [ ] vue-i18n 表单翻译
- [ ] 新登录表单 UI
- [ ] Element Plus 验证
- [ ] Google 按钮占位 (`alert('即将支持')`)

**Step 1.4: 重构Register.vue**
- [ ] 保持一致的设计语言
- [ ] 相同布局结构

**Step 1.5: 测试**
- [ ] Vitest 组件测试
- [ ] Playwright E2E 登录/注册流程
- [ ] Canvas绘制K线
- [ ] 动画循环
- [ ] 响应式适配
- [ ] 移动端静态版本

**Step 1.2: 重构AuthLayout**
- [ ] 左右分栏CSS
- [ ] 响应式断点
- [ ] 移动端堆叠布局

**Step 1.3: 重构Login.vue**
- [ ] 新登录表单UI
- [ ] 表单验证
- [ ] Google登录按钮
- [ ] 语言切换

**Step 1.4: 重构Register.vue**
- [ ] 保持一致的设计语言
- [ ] 相同布局结构

### Phase 2: 多语言支持 (后续)

**Step 2.1: vue-i18n 架构完善**
- [ ] 路由与 locale 同步 (URL 变化支持 `/en/login`)
- [ ] 全局语言切换器组件
- [ ] 翻译覆盖所有页面

**Step 2.2: 全局语言切换**
- [ ] LanguageSwitcher组件
- [ ] Persist选择到localStorage
- [ ] 路由同步

### Phase 3: 主题系统 (后续)

**Step 3.1: 橙色主题变量化**
- [ ] 更新 variables.scss
- [ ] Element Plus 主题覆盖
- [ ] 暗黑模式兼容

---

## 7. 工程评审决策 (2026-04-17)

以下决策已在工程评审中确认：

| 决策 | 选择 | 来源 | 备注 |
|------|------|------|------|
| Google登录按钮 | 预留 UI，暂不绑定行为 | 评审 Issue 1A | 后端完成后启用 |
| K线可视化实现 | 纯 Canvas 手写 | 评审 Issue 1B | 抽象艺术化动画效果 |
| i18n 方案 | Phase 1 直接集成 vue-i18n | 评审 Issue 1C | 一次到位 |
| K线动画移动端 | 移动端静态版本 | 设计规格 | MediaQuery 检测 |

**技术说明:**
- K线可视化使用 Canvas 2D API，配合 requestAnimationFrame 实现流畅动画
- vue-i18n 在 Phase 1 直接安装配置，无需后续重构
- Google 按钮点击显示 `alert('Google 登录即将支持')` 作为占位

**新增依赖 (Phase 1):**
```json
{
  "vue-i18n": "^9.0.0"
}
```

---

## 8. 并行化策略

**Lane A (可并行):**
- Step 1.0: vue-i18n 安装和配置
- Step 1.1: KlineVisualization 组件
- Step 1.2: AuthLayout 重构

**Lane B (依赖 Lane A):**
- Step 1.3: Login.vue 重构 (依赖 1.0, 1.2)
- Step 1.4: Register.vue 重构 (依赖 1.0, 1.2)

**Lane C (最后):**
- Step 1.5: 测试

**Git Worktree 建议:**
- Worktree 1: Lane A (i18n + klineviz + authlayout)
- Worktree 2: Lane B (login + register)

---

## 9. NOT in Scope

以下内容明确不在本次重构范围内：

- 后端 API 修改
- 数据库变更
- 其他页面的样式修改
- 暗黑模式支持
- 移动端原生体验优化
- 性能优化（非关键路径）

---

## 9. 验收标准

登录页重构完成的标准：

1. **视觉验收**
   - [ ] 左右分栏布局正确显示
   - [ ] 动态K线动画流畅运行
   - [ ] 橙色主题色正确应用
   - [ ] 表单样式与设计稿一致

2. **功能验收**
   - [ ] 邮箱/密码登录正常工作
   - [ ] 表单验证提示正常
   - [ ] Google登录按钮显示，点击后 alert('即将支持')
   - [ ] 语言切换正常切换中英文 (vue-i18n)
   - [ ] 移动端布局正确响应

3. **体验验收**
   - [ ] 页面加载 < 2s
   - [ ] 动画不掉帧
   - [ ] 表单输入响应及时

## 10. 测试要求

### 10.1 单元测试 (Vitest)

```javascript
// src/components/auth/KlineVisualization.spec.js
describe('KlineVisualization', () => {
  it('桌面端渲染 Canvas 动画')
  it('移动端渲染静态版本')
  it('window resize 时图表重绘')
})

// src/views/auth/Login.spec.js
describe('Login', () => {
  it('邮箱验证错误提示')
  it('密码为空提示')
  it('登录成功跳转 /')
  it('登录失败显示错误')
  it('切换到 EN 后 UI 文本变化')
  it('Google 按钮点击 alert')
})
```

### 10.2 E2E 测试 (Playwright)

```javascript
// tests/e2e/auth.spec.js
describe('Auth Flow', () => {
  it('注册 → 登录 → 登出')
  it('语言切换: 中文 → English → 中文')
  it('Google 按钮点击提示')
})
```

---

## 11. GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| Eng Review | `/plan-eng-review` | Architecture & tests | 1 | issues_open | 9 issues, 0 critical gaps |
| Design Review | `/plan-design-review` | UI/UX gaps | 1 | issues_open | score: 2/10 → 7/10 |

**UNRESOLVED:** 0 decisions

**Decisions made in this review:**
- Google OAuth → 预留按钮 UI，暂不绑定行为 (Issue 1A-A)
- K线可视化 → 纯 Canvas 手写 (Issue 1B-B)
- i18n 方案 → Phase 1 直接集成 vue-i18n (Issue 1C-B)
- 并行策略 → Lane A (1.0, 1.1, 1.2 并行), Lane B (1.3, 1.4), Lane C (1.5)

**VERDICT:** ENG REVIEW ISSUES OPEN — 修复 Issue 1A 后可开始实施
