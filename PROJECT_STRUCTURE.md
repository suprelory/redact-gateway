# Redact Gateway - 项目骨架说明

项目骨架已创建完成！以下是项目结构和关键文件说明：

## 📁 项目结构

```
redact-gateway/
├── backend/                    # 后端服务（Node.js + TypeScript）
│   ├── src/
│   │   ├── engine/            # 脱敏引擎模块（待实现）
│   │   ├── storage/           # 加密存储模块（待实现）
│   │   ├── api/               # 管理 API 模块（待实现）
│   │   ├── main.ts            # 入口文件
│   │   ├── config.ts          # 配置管理
│   │   ├── logger.ts          # 日志模块
│   │   └── types.ts           # 类型定义
│   ├── package.json
│   ├── tsconfig.json
│   └── .env.example           # 环境变量示例
│
├── frontend/                   # 前端界面（React + TypeScript）
│   ├── src/
│   │   ├── pages/             # 页面组件
│   │   │   ├── Dashboard.tsx  # 仪表盘
│   │   │   ├── Rules.tsx      # 规则管理
│   │   │   ├── Traffic.tsx    # 实时流量
│   │   │   ├── Upstream.tsx   # 上游配置
│   │   │   └── Settings.tsx   # 设置
│   │   ├── components/
│   │   │   ├── layout/        # 布局组件
│   │   │   ├── ui/            # UI 组件（待添加）
│   │   │   └── common/        # 通用组件（待添加）
│   │   ├── lib/
│   │   │   ├── api.ts         # API 客户端
│   │   │   ├── types.ts       # 类型定义
│   │   │   └── utils.ts       # 工具函数
│   │   ├── styles/
│   │   │   └── globals.css    # 全局样式（深色主题）
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   └── index.html
│
├── docker-compose.yml          # Docker Compose 配置
├── Dockerfile                  # 多阶段构建 Dockerfile
├── .dockerignore
├── .gitignore
├── LICENSE                     # MIT 许可证
└── README.md                   # 项目说明

```

## 🎨 已完成的部分

### 1. 配置文件
- ✅ TypeScript 配置（backend + frontend）
- ✅ Vite 构建配置
- ✅ Tailwind CSS 配置（深色主题）
- ✅ ESLint 配置准备
- ✅ Docker 配置

### 2. 后端基础
- ✅ 配置管理模块（config.ts）
- ✅ 日志系统（logger.ts with Pino）
- ✅ 完整类型定义（types.ts）
- ✅ 主入口文件框架（main.ts）

### 3. 前端界面
- ✅ 路由配置
- ✅ 应用布局（侧边栏导航）
- ✅ 5 个主要页面（含模拟数据）:
  - 仪表盘：统计卡片 + 图表占位
  - 规则管理：规则列表 + 优先级显示
  - 实时流量：请求日志表格
  - 上游配置：上游端点卡片
  - 设置：系统参数配置
- ✅ 深色主题样式（基于设计规范）
- ✅ API 客户端封装
- ✅ 工具函数库

### 4. 部署配置
- ✅ Docker 多阶段构建
- ✅ Docker Compose 配置
- ✅ 环境变量示例

## 🚀 下一步开发计划

### Phase 1: 核心引擎（优先）
1. **脱敏引擎** (`backend/src/engine/redactor.ts`)
   - 规则匹配器
   - 占位符生成
   - 文本扫描和替换

2. **还原引擎** (`backend/src/engine/restorer.ts`)
   - SSE 流式还原
   - 滑动窗口算法
   - JSON 响应还原

3. **规则系统** (`backend/src/engine/rules.ts`)
   - 内置规则集
   - 自定义规则管理
   - 规则优先级排序

### Phase 2: 存储层
1. **映射存储** (`backend/src/storage/mappings.ts`)
   - SQLite 数据库
   - 加密/解密实现
   - CRUD 操作

2. **加密模块** (`backend/src/storage/crypto.ts`)
   - AES-256-GCM 加密
   - HMAC-SHA256 索引
   - 密钥派生（HKDF）

### Phase 3: 代理服务
1. **HTTP 代理服务器**
   - 请求拦截
   - Body 解析
   - 协议识别

2. **响应处理**
   - SSE 流式代理
   - JSON 响应处理
   - 错误处理

### Phase 4: 管理 API
1. **REST API** (`backend/src/api/`)
   - 规则管理接口
   - 统计查询接口
   - 配置管理接口

2. **前端集成**
   - 替换模拟数据为真实 API
   - 实时数据更新
   - 错误处理

## 📝 快速开始

### 安装依赖

```bash
# 后端
cd backend
npm install

# 前端
cd frontend
npm install
```

### 开发模式

```bash
# 后端（终端 1）
cd backend
npm run dev

# 前端（终端 2）
cd frontend
npm run dev
```

### 配置环境变量

```bash
cd backend
cp .env.example .env
# 编辑 .env 文件，设置 MASTER_KEY 等参数
```

## 🎯 技术亮点

1. **深色主题设计** - 基于你的 design_sense，使用青色系强调色，专业数据安全感
2. **类型安全** - 完整的 TypeScript 类型定义
3. **模块化架构** - 清晰的职责分离
4. **生产就绪** - Docker 部署配置完备
5. **开发体验** - Vite 热重载 + Pino 美化日志

## 📚 参考资料

项目核心逻辑参考了：
- **CosyRedactGateway** - 单文件 Worker 设计，SSE 流式还原
- **Veil** - Rust 架构，加密存储方案
- **Maskit** - Web 界面设计，功能布局

---

**下一步建议**: 从 Phase 1 核心引擎开始实现，我可以立即编写脱敏引擎和规则系统的代码。
