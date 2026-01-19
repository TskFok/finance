# 记账系统

一个使用 Go 语言开发的记账系统，包含 API 接口和后台管理系统。

**✨ 单文件部署**：前端页面和默认配置都嵌入到二进制文件中，只需一个可执行文件即可启动，无需任何外部依赖。

## ✨ 功能特性

### API 接口（供 App/H5 使用）

#### 用户认证
- ✅ 用户注册（支持邮箱验证）
- ✅ 用户登录（JWT 鉴权）
- ✅ 获取用户信息
- ✅ 修改密码
- ✅ 邮箱验证码发送与验证
- ✅ 密码重置（邮箱验证码方式）

#### 消费记录管理
- ✅ 消费记录 CRUD 操作
- ✅ 按时间/类别筛选记录
- ✅ 分页查询
- ✅ 消费统计功能
- ✅ 动态消费类别管理（从数据库获取）

#### 收入管理
- ✅ 收入记录 CRUD 操作
- ✅ 按时间/类型筛选记录
- ✅ 分页查询
- ✅ 收入统计功能

#### 数据导出
- ✅ 导出 CSV 文件
- ✅ 导出 JSON 数据

#### 其他
- ✅ Swagger API 文档
- ✅ CORS 支持

### 后台管理系统

#### 认证与安全
- ✅ 管理员登录/退出
- ✅ Cookie 会话管理
- ✅ 密码重置（邮件链接方式）
- ✅ 管理员直接重置用户密码
- ✅ 邮件配置管理

#### 数据管理
- ✅ 数据概览仪表盘（包含收入和支出统计）
- ✅ 消费记录管理（查看、添加、编辑、删除、筛选）
- ✅ 收入记录管理（查看、添加、编辑、删除、筛选）
- ✅ 消费类别管理（增删改查，支持排序）
- ✅ 用户管理（查看所有用户）

#### AI 功能
- ✅ **AI 模型管理**：配置多个 AI 模型（名称、API 地址、API Key）
- ✅ **AI 账单分析**：选择时间范围和 AI 模型，流式输出账单总结和意见
- ✅ **AI 分析历史**：查看历史分析记录，支持分页和软删除
- ✅ **AI 聊天**：与 AI 模型进行对话，流式输出响应
- ✅ **AI 聊天历史**：查看历史对话记录，支持软删除
- ✅ **Markdown 渲染**：AI 响应自动格式化为 Markdown

#### 数据导出
- ✅ 导出 Excel 文件（支持筛选条件）

#### 其他
- ✅ 前端资源嵌入二进制
- ✅ 响应式设计，支持移动端访问

## 🛠 技术栈

- **Web 框架**: Gin
- **ORM**: GORM
- **数据库**: MySQL
- **认证**: JWT (App端) + Cookie (后台管理)
- **文档**: Swagger
- **Excel**: excelize
- **邮件**: gomail
- **配置**: viper
- **嵌入**: Go embed
- **AI 集成**: 支持 OpenAI 兼容 API（流式响应）

## 🚀 快速开始

### 1. 准备数据库

```sql
CREATE DATABASE finance CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. 直接运行（使用内置默认配置）

```bash
# 安装依赖
go mod tidy

# 直接运行（使用内置默认配置）
go run main.go
```

**内置默认配置**：
- 数据库：`root@127.0.0.1:3306/finance`
- 服务端口：`:8811`
- JWT 过期：24 小时

### 3. 使用外部配置（可选）

如需自定义配置，创建 `config.yaml` 文件：

```yaml
# 只需要填写需要修改的配置项，其他使用内置默认值
database:
  password: "your_password"  # 数据库密码

jwt:
  secret: "your-production-secret"  # 生产环境密钥

email:
  enabled: true
  host: "smtp.qq.com"
  port: 465
  username: "your_email@qq.com"
  password: "your_authorization_code"
```

然后指定配置文件启动：

```bash
go run main.go -c config.yaml
```

### 4. 访问系统

- **后台管理**: http://localhost:8811/
- **API 文档**: http://localhost:8811/swagger/index.html
- **健康检查**: http://localhost:8811/health

## 📦 打包部署

### 构建二进制文件

```bash
# 构建当前平台
make build

# 构建 Linux
make build-linux

# 构建 Windows
make build-windows

# 构建 macOS
make build-mac

# 构建所有平台
make build-all
```

### 部署说明

**单文件部署** - 只需一个二进制文件即可运行：

```bash
# Linux/macOS - 直接运行
./finance-linux-amd64

# Windows - 直接运行
finance-windows-amd64.exe

# 查看帮助
./finance-linux-amd64 -h

# 查看版本
./finance-linux-amd64 -v
```

**可选：使用外部配置覆盖**

```bash
# 方式1: 指定配置文件
./finance-linux-amd64 -c /path/to/config.yaml

# 方式2: 环境变量覆盖（仅覆盖需要修改的配置）
FINANCE_DATABASE_PASSWORD=secret ./finance-linux-amd64
```

**所有资源已嵌入**：前端页面、默认配置都打包在二进制文件中。

## 📚 API 接口

详细 API 文档请访问：http://localhost:8811/swagger/index.html

### 认证相关（/api/v1/auth）

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /api/v1/auth/register | 用户注册 | 否 |
| POST | /api/v1/auth/login | 用户登录 | 否 |
| POST | /api/v1/auth/send-code | 发送邮箱验证码 | 否 |
| POST | /api/v1/auth/verify-code | 验证邮箱验证码 | 否 |
| POST | /api/v1/auth/register-verified | 带验证码的用户注册 | 否 |
| GET | /api/v1/auth/profile | 获取用户信息 | JWT |
| PUT | /api/v1/auth/password | 修改密码 | JWT |
| POST | /api/v1/auth/password/request-reset | 请求密码重置（发送验证码） | 否 |
| POST | /api/v1/auth/password/verify-code | 验证重置验证码 | 否 |
| POST | /api/v1/auth/password/reset | 重置密码 | 否 |

### 消费类别（/api/v1/categories）

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /api/v1/categories | 获取消费类别列表 | 否 |

### 消费记录（/api/v1/expenses）

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /api/v1/expenses | 创建消费记录 | JWT |
| GET | /api/v1/expenses | 获取消费记录列表（支持分页、筛选） | JWT |
| GET | /api/v1/expenses/:id | 获取单条消费记录 | JWT |
| PUT | /api/v1/expenses/:id | 更新消费记录 | JWT |
| DELETE | /api/v1/expenses/:id | 删除消费记录 | JWT |
| GET | /api/v1/expenses/statistics | 获取消费统计 | JWT |

**查询参数**：
- `page`: 页码（默认 1）
- `page_size`: 每页数量（默认 10）
- `category`: 类别筛选
- `start_time`: 开始时间（格式：2024-01-01）
- `end_time`: 结束时间（格式：2024-12-31）

### 收入管理（/api/v1/incomes）

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /api/v1/incomes | 创建收入记录 | JWT |
| GET | /api/v1/incomes | 获取收入记录列表（支持分页、筛选） | JWT |
| GET | /api/v1/incomes/:id | 获取单条收入记录 | JWT |
| PUT | /api/v1/incomes/:id | 更新收入记录 | JWT |
| DELETE | /api/v1/incomes/:id | 删除收入记录 | JWT |

**查询参数**：
- `page`: 页码（默认 1）
- `page_size`: 每页数量（默认 10）
- `type`: 收入类型筛选
- `start_time`: 开始时间（格式：2024-01-01）
- `end_time`: 结束时间（格式：2024-12-31）

### 数据导出（/api/v1/export）

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /api/v1/export/csv | 导出 CSV 文件 | JWT |
| GET | /api/v1/export/json | 导出 JSON 数据 | JWT |

**查询参数**：
- `start_time`: 开始时间（必填，格式：2024-01-01）
- `end_time`: 结束时间（必填，格式：2024-12-31）

### 后台管理接口（/admin）

#### 认证相关

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /admin/login | 管理员登录 | 否 |
| POST | /admin/logout | 退出登录 | Cookie |
| POST | /admin/password/request-reset | 请求密码重置邮件 | 否 |
| GET | /admin/password/verify-token | 验证重置令牌 | 否 |
| POST | /admin/password/reset | 使用令牌重置密码 | 否 |
| POST | /admin/password/admin-reset | 管理员直接重置密码 | Cookie |
| POST | /admin/password/send-reset-email | 发送重置邮件 | Cookie |
| GET | /admin/email-config | 获取邮件配置 | Cookie |

#### 数据管理

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /admin/expenses | 获取所有消费记录 | Cookie |
| POST | /admin/expenses | 创建消费记录 | Cookie |
| PUT | /admin/expenses/:id | 更新消费记录 | Cookie |
| DELETE | /admin/expenses/:id | 删除消费记录 | Cookie |
| GET | /admin/incomes | 获取所有收入记录 | Cookie |
| POST | /admin/incomes | 创建收入记录 | Cookie |
| PUT | /admin/incomes/:id | 更新收入记录 | Cookie |
| DELETE | /admin/incomes/:id | 删除收入记录 | Cookie |
| GET | /admin/categories | 获取所有消费类别 | Cookie |
| POST | /admin/categories | 创建消费类别 | Cookie |
| PUT | /admin/categories/:id | 更新消费类别 | Cookie |
| DELETE | /admin/categories/:id | 删除消费类别 | Cookie |
| GET | /admin/users | 获取所有用户 | Cookie |
| GET | /admin/statistics | 获取统计数据（包含收入和支出） | Cookie |
| GET | /admin/export/excel | 导出 Excel 文件 | Cookie |

#### AI 功能

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /admin/ai-models | 获取所有 AI 模型 | Cookie |
| GET | /admin/ai-models/:id | 获取单个 AI 模型 | Cookie |
| POST | /admin/ai-models | 创建 AI 模型 | Cookie |
| PUT | /admin/ai-models/:id | 更新 AI 模型 | Cookie |
| DELETE | /admin/ai-models/:id | 删除 AI 模型 | Cookie |
| POST | /admin/ai-analysis | AI 账单分析（流式输出） | Cookie |
| GET | /admin/ai-analysis/history | 获取分析历史（支持分页） | Cookie |
| DELETE | /admin/ai-analysis/history/:id | 删除分析历史（软删除） | Cookie |
| POST | /admin/ai-chat | AI 聊天（流式输出） | Cookie |
| GET | /admin/ai-chat/history | 获取聊天历史 | Cookie |
| DELETE | /admin/ai-chat/history/:id | 删除聊天历史（软删除） | Cookie |

## 📱 安卓集成示例

```kotlin
// Retrofit 配置
val client = OkHttpClient.Builder()
    .addInterceptor { chain ->
        val request = chain.request().newBuilder()
            .header("Authorization", "Bearer $token")
            .build()
        chain.proceed(request)
    }
    .build()

val retrofit = Retrofit.Builder()
    .baseUrl("http://your-server:8811/")
    .client(client)
    .addConverterFactory(GsonConverterFactory.create())
    .build()
```

## 🔧 配置说明

### 配置文件

程序按以下顺序查找配置文件：
1. 命令行指定的路径 (`-c` 或 `--config`)
2. 当前目录 `./config.yaml`
3. config 目录 `./config/config.yaml`
4. 系统目录 `/etc/finance/config.yaml`
5. 用户目录 `~/.finance/config.yaml`

### 环境变量覆盖

所有配置都可以通过环境变量覆盖，格式：`FINANCE_配置路径`（用下划线分隔）

| 环境变量 | 对应配置 | 默认值 |
|----------|----------|--------|
| FINANCE_SERVER_PORT | server.port | :8811 |
| FINANCE_SERVER_MODE | server.mode | release |
| FINANCE_SERVER_BASE_URL | server.base_url | http://localhost:8811 |
| FINANCE_DATABASE_HOST | database.host | 127.0.0.1 |
| FINANCE_DATABASE_PORT | database.port | 3306 |
| FINANCE_DATABASE_USERNAME | database.username | root |
| FINANCE_DATABASE_PASSWORD | database.password | (空) |
| FINANCE_DATABASE_DBNAME | database.dbname | finance |
| FINANCE_JWT_SECRET | jwt.secret | (默认值) |
| FINANCE_JWT_EXPIRE_HOURS | jwt.expire_hours | 24 |
| FINANCE_EMAIL_ENABLED | email.enabled | false |
| FINANCE_EMAIL_HOST | email.host | smtp.qq.com |
| FINANCE_EMAIL_PORT | email.port | 465 |
| FINANCE_EMAIL_USERNAME | email.username | (空) |
| FINANCE_EMAIL_PASSWORD | email.password | (空) |

## 📁 项目结构

```
finance/
├── api/                    # API 处理器
│   ├── admin.go            # 后台管理 API
│   ├── auth.go             # 用户认证（App端）
│   ├── expense.go          # 消费记录
│   ├── income.go           # 收入管理
│   ├── category.go         # 消费类别管理
│   ├── export.go           # 数据导出
│   ├── password_reset.go   # 密码重置（后台）
│   ├── ai_model.go         # AI 模型管理
│   ├── ai_analysis.go      # AI 账单分析
│   ├── ai_chat.go          # AI 聊天
│   └── response.go         # 响应格式
├── config/                 # 配置管理
│   ├── config.go           # Viper 配置加载
│   ├── embed.go            # 配置文件嵌入声明
│   └── default.yaml        # 内置默认配置（嵌入）
├── database/               # 数据库初始化
│   └── database.go         # GORM 自动迁移
├── docs/                   # Swagger 文档
│   ├── docs.go             # 生成的文档代码
│   ├── swagger.json        # JSON 格式文档
│   └── swagger.yaml        # YAML 格式文档
├── middleware/             # 中间件
│   └── jwt.go              # JWT 认证
├── models/                 # 数据模型
│   ├── user.go             # 用户模型
│   ├── expense.go          # 消费记录模型
│   ├── income.go           # 收入模型
│   ├── category.go         # 消费类别模型
│   ├── password_reset.go   # 密码重置令牌模型
│   ├── email_verification.go # 邮箱验证码模型
│   ├── ai_model.go         # AI 模型配置
│   ├── ai_analysis.go      # AI 分析历史
│   └── ai_chat.go          # AI 聊天历史
├── router/                 # 路由配置
│   └── router.go           # 路由设置
├── service/                # 业务服务
│   └── email.go            # 邮件服务
├── web/                    # 前端资源（嵌入）
│   ├── embed.go            # 前端嵌入声明
│   └── index.html          # 后台管理页面
├── config.example.yaml     # 外部配置文件示例
├── main.go                 # 程序入口
├── go.mod                  # 依赖管理
├── Makefile                # 构建脚本
└── README.md               # 说明文档
```

## 📋 数据模型

### 用户（User）
- ID、用户名、邮箱、密码（加密）、创建时间、更新时间

### 消费记录（Expense）
- ID、用户ID、金额、类别、描述、消费时间、创建时间、更新时间

### 收入记录（Income）
- ID、用户ID、金额、类型、收入时间、创建时间、更新时间

### 消费类别（Category）
- ID、名称、排序、创建时间、更新时间、删除时间（软删除）

### AI 模型（AIModel）
- ID、名称、API 地址、API Key、创建时间、更新时间

### AI 分析历史（AIAnalysisHistory）
- ID、AI模型ID、开始时间、结束时间、提示词、分析结果、创建时间、删除时间（软删除）

### AI 聊天历史（AIChatMessage）
- ID、AI模型ID、用户输入、AI响应、创建时间、删除时间（软删除）

## 📧 邮件配置

要启用邮件发送功能（密码重置、邮箱验证），需要配置以下环境变量：

```bash
# 启用邮件服务
export FINANCE_EMAIL_ENABLED=true

# QQ 邮箱示例
export FINANCE_EMAIL_HOST=smtp.qq.com
export FINANCE_EMAIL_PORT=465
export FINANCE_EMAIL_USERNAME=your_qq@qq.com
export FINANCE_EMAIL_PASSWORD=your_authorization_code  # QQ邮箱授权码

# 163 邮箱示例
export FINANCE_EMAIL_HOST=smtp.163.com
export FINANCE_EMAIL_PORT=465
export FINANCE_EMAIL_USERNAME=your_email@163.com
export FINANCE_EMAIL_PASSWORD=your_authorization_code

# 服务器地址（邮件中的重置链接）
export FINANCE_SERVER_BASE_URL=https://your-domain.com
```

### 获取邮箱授权码

**QQ 邮箱**：设置 → 账户 → POP3/SMTP服务 → 开启 → 生成授权码

**163 邮箱**：设置 → POP3/SMTP/IMAP → 开启服务 → 获取授权码

### 密码重置流程

1. **用户自助重置（App端）**：
   - 在登录页点击"忘记密码"
   - 输入注册邮箱
   - 收到验证码邮件
   - 输入验证码和新密码完成重置

2. **用户自助重置（后台）**：
   - 在登录页点击"忘记密码"
   - 输入注册邮箱
   - 收到邮件后点击链接
   - 设置新密码

3. **管理员重置**：
   - 登录后台 → 用户管理
   - 点击"重置密码"直接设置新密码
   - 或点击"发送邮件"发送重置链接

## 🤖 AI 功能使用

### 1. 配置 AI 模型

在后台管理的"AI 模型"页面，添加 AI 模型配置：
- **名称**：模型显示名称（如：OpenAI GPT-4）
- **API 地址**：OpenAI 兼容的 API 地址（如：`https://api.openai.com/v1`）
- **API Key**：对应的 API 密钥

### 2. AI 账单分析

1. 进入"AI 分析"页面
2. 选择 AI 模型
3. 选择时间范围
4. 点击"开始分析"
5. 系统会流式输出分析结果（Markdown 格式）
6. 分析完成后自动保存到历史记录

### 3. AI 聊天

1. 进入"AI 聊天"页面
2. 选择 AI 模型
3. 输入消息并发送
4. 系统会流式输出 AI 响应（Markdown 格式）
5. 对话历史自动保存

### 支持的 AI 服务

- OpenAI API（`https://api.openai.com/v1`）
- 其他 OpenAI 兼容的 API 服务（如：本地部署的模型、第三方代理服务等）

## ⚠️ 注意事项

1. **安全性**: 生产环境请修改 JWT_SECRET 和数据库密码
2. **数据库**: 确保 MySQL 服务已启动并创建数据库
3. **跨平台**: 使用 MySQL 驱动，支持无 CGO 跨平台编译
4. **邮件**: 邮件服务为可选功能，不配置也可使用管理员直接重置密码
5. **AI 功能**: AI 功能需要配置有效的 AI 模型和 API Key，否则无法使用
6. **数据备份**: 建议定期备份数据库，特别是生产环境

## 📄 许可证

MIT License
