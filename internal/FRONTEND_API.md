# 前端接口契约

> 状态：Draft  
> 面向阶段：浏览器端优先，后续接入 Wails  
> 当前实现进度：见文末 API Checklist。

## 1. 目标

前端只依赖本文定义的数据结构和 service 方法，不直接依赖 Gin 或 Wails。

```text
Vue 页面
  ↓
Frontend Service
  ├─ Web 阶段：HTTP /api/v1
  └─ Wails 阶段：Wails binding
  ↓
相同的业务请求与响应模型
```

第一阶段只需要支持本地网页开发。清理接口只重写本地仓库历史，不自动修改远端或执行 force push。

## 2. HTTP 约定

### 2.1 Base URL

```text
http://127.0.0.1:{port}/api/v1
```

- 后端只监听 `127.0.0.1`，不要监听 `0.0.0.0`。
- 前端通过环境变量或 Vite proxy 配置地址，不在组件中硬编码端口。
- 开发阶段只允许明确配置的前端 Origin。

### 2.2 数据格式

- Content-Type：`application/json; charset=utf-8`
- 时间：RFC 3339，例如 `2026-10-06T14:30:00+08:00`
- 字节数：JSON number，字段名统一使用 `Bytes` 后缀
- Git SHA / Object ID：接口返回完整值，前端自行截断显示
- 可空字段：使用 `null`，不要用空字符串代替
- 列表分页：使用不透明 cursor，前端不得解析 cursor

### 2.3 成功响应

所有响应同时返回 `X-Request-ID` Header，其值必须与 `meta.requestId` 一致。

```json
{
  "data": {},
  "meta": {
    "requestId": "req_01J..."
  }
}
```

列表响应：

```json
{
  "data": [],
  "meta": {
    "requestId": "req_01J...",
    "nextCursor": "opaque-cursor-or-null",
    "hasMore": false
  }
}
```

### 2.4 错误响应

```json
{
  "error": {
    "code": "REPOSITORY_NOT_FOUND",
    "message": "The selected path does not exist.",
    "details": {
      "path": "D:\\repo"
    },
    "retryable": false
  },
  "meta": {
    "requestId": "req_01J..."
  }
}
```

前端展示 `message`，业务分支只判断稳定的 `code`。

### 2.5 HTTP 状态码

| 状态码 | 使用场景 |
|---:|---|
| `200` | 查询、更新成功 |
| `201` | 创建扫描任务成功 |
| `202` | 请求已接受，后台处理中 |
| `204` | 关闭仓库、取消任务等无响应体操作成功 |
| `400` | 请求格式或参数错误 |
| `404` | 仓库、提交或任务不存在 |
| `409` | 当前状态冲突，例如已有扫描正在运行 |
| `422` | 路径存在但不是有效 Git 仓库等业务校验失败 |
| `500` | 未预期内部错误 |
| `503` | Git CLI 不可用或依赖暂时不可用 |

## 3. 接口总览

### P0：页面壳与仓库选择

| Method | Path | 用途 |
|---|---|---|
| `GET` | `/health` | 服务与 Git CLI 状态 |
| `GET` | `/github/connection` | 验证 GitHub token 与连通性 |
| `GET` | `/repositories/current` | 当前打开的仓库 |
| `POST` | `/repositories/open` | 通过本地路径打开仓库 |
| `DELETE` | `/repositories/current` | 关闭当前仓库 |
| `GET` | `/repositories/recent` | 最近打开的仓库 |
| `DELETE` | `/repositories/recent/{repositoryId}` | 删除一条最近记录 |

### P1：提交历史分析

| Method | Path | 用途 |
|---|---|---|
| `POST` | `/repositories/current/scans` | 启动仓库扫描 |
| `GET` | `/tasks/{taskId}` | 获取扫描进度 |
| `DELETE` | `/tasks/{taskId}` | 取消任务 |
| `GET` | `/repositories/current/commits` | 分页获取提交历史 |
| `GET` | `/repositories/current/commits/{sha}` | 获取提交详情 |
| `GET` | `/repositories/current/commits/{sha}/files` | 获取提交文件变化 |

### P2：提交历史清理

| Method | Path | 用途 |
|---|---|---|
| `POST` | `/repositories/current/cleanup/preview` | 检查选中 commit 的文件路径是否仍存在于当前 HEAD |
| `POST` | `/repositories/current/cleanup` | 删除选中的 commit 并重写当前分支历史 |

## 4. P0 接口

### 4.1 服务状态

```http
GET /api/v1/health
```

响应：

```json
{
  "data": {
    "status": "ok",
    "version": "0.1.0",
    "git": {
      "available": true,
      "version": "2.51.0"
    }
  },
  "meta": {
    "requestId": "req_01J..."
  }
}
```

`status`：`ok | degraded`

该接口只检查本地服务和 Git CLI，不检查 GitHub API。GitHub 连通性由独立的 `/api/v1/github/connection` 接口负责。

#### 4.1.1 GitHub 连通性

```http
GET /api/v1/github/connection
Authorization: Bearer <token>
```

成功返回：

```json
{
  "data": true,
  "meta": {
    "requestId": "req_01J..."
  }
}
```

token 仅用于当前请求，Web 前端不得将其写入 localStorage、日志或构建产物。

可能错误：

| code | HTTP | 说明 |
|---|---:|---|
| `INVALID_REQUEST` | 400 | Authorization Header 缺失或格式错误 |
| `GITHUB_UNAUTHORIZED` | 401 | token 无效 |
| `GITHUB_FORBIDDEN` | 403 | token 没有访问权限或 GitHub 拒绝访问 |
| `GITHUB_CONNECTION_FAILED` | 503 | 网络或 GitHub 服务不可用 |

### 4.2 获取当前仓库

```http
GET /api/v1/repositories/current
```

- 已打开：`200`，返回 `RepositorySummary`
- 未打开：`404`，错误码 `NO_REPOSITORY_OPEN`

### 4.3 打开仓库

```http
POST /api/v1/repositories/open
```

网页无法可靠获得本地目录的绝对路径，因此 Web 阶段由用户输入或粘贴路径。接入 Wails 后，原生目录选择器只负责产生同一个 `path` 请求字段。

请求：

```json
{
  "path": "E:\\projects\\example"
}
```

响应：

```json
{
  "data": {
    "id": "repo_01J...",
    "name": "example",
    "path": "E:\\projects\\example",
    "branch": "main",
    "detachedHead": false,
    "head": "8b1a9953c4611296a827abf8c47804d7f8e4f2d1",
    "commitCount": 326,
    "workingTreeStatus": "clean",
    "gitDirectoryBytes": 1954210112,
    "analysisStatus": "not_scanned",
    "openedAt": "2026-10-06T14:30:00+08:00"
  },
  "meta": {
    "requestId": "req_01J..."
  }
}
```

`RepositorySummary`：

| 字段 | 类型 | 必需 | 说明 |
|---|---|---:|---|
| `id` | `string` | 是 | 当前进程内稳定的仓库 ID |
| `name` | `string` | 是 | 仓库目录名 |
| `path` | `string` | 是 | 规范化后的绝对路径 |
| `branch` | `string \| null` | 是 | 当前分支；detached HEAD 或空仓库时为 null |
| `detachedHead` | `boolean` | 是 | 是否为 detached HEAD |
| `head` | `string \| null` | 是 | 完整 HEAD SHA；空仓库为 null |
| `commitCount` | `number \| null` | 是 | 可达提交数量；未计算时为 null |
| `workingTreeStatus` | `clean \| dirty \| unborn \| unavailable` | 是 | 工作区状态 |
| `gitDirectoryBytes` | `number \| null` | 是 | `.git` 实际占用；未计算时为 null |
| `analysisStatus` | `not_scanned \| scanning \| ready \| failed` | 是 | 完整分析状态 |
| `openedAt` | `string` | 是 | 本次打开时间 |

可能错误：

| code | 说明 |
|---|---|
| `PATH_REQUIRED` | 路径为空 |
| `REPOSITORY_NOT_FOUND` | 路径不存在 |
| `PATH_NOT_READABLE` | 无读取权限 |
| `NOT_A_GIT_REPOSITORY` | 不是 Git 仓库 |
| `GIT_NOT_AVAILABLE` | 系统 Git CLI 不可用 |

### 4.4 关闭当前仓库

```http
DELETE /api/v1/repositories/current
```

成功返回 `204`。如果扫描任务仍在运行，后端先取消属于该仓库的任务，再释放当前仓库状态。

### 4.5 最近仓库

```http
GET /api/v1/repositories/recent
```

响应项：

```json
{
  "id": "repo_01J...",
  "name": "example",
  "path": "E:\\projects\\example",
  "lastOpenedAt": "2026-10-06T14:30:00+08:00",
  "available": true
}
```

默认按 `lastOpenedAt` 倒序，最多返回 10 条，不分页。

## 5. P1 接口

### 5.1 启动扫描

```http
POST /api/v1/repositories/current/scans
```

请求：

```json
{
  "force": false
}
```

响应 `202`：

```json
{
  "data": {
    "taskId": "task_01J...",
    "type": "repository_scan",
    "status": "queued",
    "progress": {
      "phase": "queued",
      "current": 0,
      "total": null,
      "percent": null
    },
    "createdAt": "2026-10-06T14:31:00+08:00",
    "startedAt": null,
    "finishedAt": null
  },
  "meta": {
    "requestId": "req_01J..."
  }
}
```

同一仓库已有扫描运行时返回 `409 / SCAN_ALREADY_RUNNING`，并在 `details.taskId` 返回现有任务 ID。

### 5.2 查询任务

```http
GET /api/v1/tasks/{taskId}
```

`Task` 字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `taskId` | `string` | 任务 ID |
| `type` | `repository_scan` | 任务类型 |
| `status` | `queued \| running \| completed \| failed \| cancelled` | 任务状态 |
| `progress.phase` | `string` | 当前阶段，例如 `commits` |
| `progress.current` | `number` | 当前完成量 |
| `progress.total` | `number \| null` | 总量未知时为 null |
| `progress.percent` | `number \| null` | `0..100`，无法估算时为 null |
| `error` | `ApiError \| null` | 失败原因 |
| `createdAt` | `string` | 创建时间 |
| `startedAt` | `string \| null` | 开始时间 |
| `finishedAt` | `string \| null` | 结束时间 |

第一阶段前端每 800–1500ms 轮询一次；页面隐藏后降低频率。后续如有需要再增加 SSE，不作为 MVP 前置条件。

### 5.3 提交列表

```http
GET /api/v1/repositories/current/commits
```

Query：

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---:|---|
| `cursor` | string | — | 不透明分页 cursor |
| `limit` | number | `50` | `1..100` |
| `query` | string | — | 匹配提交信息或 SHA |
| `author` | string | — | 作者名称或邮箱 |
| `since` | RFC3339 | — | 起始时间 |
| `until` | RFC3339 | — | 结束时间 |
| `ref` | string | `HEAD` | 分支、标签或 SHA |
| `minIntroducedBytes` | number | `0` | 仅返回 `stats.introducedBytes` 大于或等于该字节数的提交 |

`minIntroducedBytes` 必须是大于或等于 0 的整数。前端改变任一筛选条件后必须清空旧 cursor，并从第一页重新请求。

响应项 `CommitSummary`：

```json
{
  "sha": "8b1a9953c4611296a827abf8c47804d7f8e4f2d1",
  "shortSha": "8b1a995",
  "subject": "Remove generated archive",
  "author": {
    "name": "Example User",
    "email": "user@example.com"
  },
  "authoredAt": "2026-10-05T10:20:00+08:00",
  "committedAt": "2026-10-05T10:21:00+08:00",
  "parents": ["f6ff93b7080b50aa86061d173e2a0c0f7f20b28d"],
  "refs": {
    "branches": ["main"],
    "tags": []
  },
  "stats": {
    "introducedBytes": 1048576,
    "snapshotBytes": 125829120,
    "addedFiles": 1,
    "modifiedFiles": 2,
    "deletedFiles": 0
  },
  "analysisStatus": "ready"
}
```

`stats` 在尚未分析时为 `null`；`analysisStatus` 为 `pending | ready | failed`。

### 5.4 提交详情

```http
GET /api/v1/repositories/current/commits/{sha}
```

在 `CommitSummary` 基础上增加：

```json
{
  "body": "Optional full commit message",
  "committer": {
    "name": "Example User",
    "email": "user@example.com"
  }
}
```

### 5.5 提交文件变化

返回某一个提交相对其第一个父提交修改的文件列表。根提交没有父提交，按“空文件树 → 根提交文件树”比较，因此其中的文件均为 `added`。merge commit 暂不返回 combined diff，只与第一个父提交比较。

请求：

```http
GET /api/v1/repositories/current/commits/8b1a9953c4611296a827abf8c47804d7f8e4f2d1/files?limit=100&sort=introducedBytes&order=desc HTTP/1.1
Accept: application/json
```

此接口没有请求体。`sha` 必须使用提交列表返回的完整 40 位 commit SHA。

Query：

| 参数 | 类型 | 默认值 | 约束与说明 |
|---|---|---:|---|
| `cursor` | string | — | 不透明分页 cursor；第一页省略，下一页原样传回 `meta.nextCursor` |
| `limit` | number | `100` | `1..100` |
| `sort` | string | `introducedBytes` | `introducedBytes \| path \| newBytes \| additions \| deletions` |
| `order` | string | `desc` | `asc \| desc` |

服务端先按 `sort` 和 `order` 排序，再进行 cursor 分页。排序值相同时按 `path` 升序保证结果稳定；可空的数值字段为 `null` 时始终排在非空值之后。cursor 与 `sha`、`sort`、`order` 绑定，更换任一参数后必须从第一页重新请求。

成功响应：

```json
{
  "data": [
    {
      "path": "internal/handler/health.go",
      "previousPath": null,
      "status": "modified",
      "oldBlob": "a36d52c10c7d9f3c1a0b7cddbf4355f96c601219",
      "newBlob": "8f91d72475003e34139d0c4f2a8357f176665a19",
      "oldBytes": 1250,
      "newBytes": 1380,
      "introducedBytes": 1380,
      "additions": 3,
      "deletions": 3,
      "binary": false
    },
    {
      "path": "assets/archive.zip",
      "previousPath": null,
      "status": "added",
      "oldBlob": null,
      "newBlob": "32c9e5d8bc45c1f1576c2d34feaa263405cf31af",
      "oldBytes": null,
      "newBytes": 1048576,
      "introducedBytes": 1048576,
      "additions": null,
      "deletions": null,
      "binary": true
    }
  ],
  "meta": {
    "requestId": "req_01J...",
    "nextCursor": "opaque-cursor-or-null",
    "hasMore": true
  }
}
```

响应项 `ChangedFile`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `path` | `string` | 当前路径；删除文件使用删除前的路径 |
| `previousPath` | `string \| null` | rename/copy 前的路径，其他状态为 null |
| `status` | `added \| modified \| deleted \| renamed \| copied \| type_changed` | 文件状态 |
| `oldBlob` | `string \| null` | 旧 blob OID |
| `newBlob` | `string \| null` | 新 blob OID |
| `oldBytes` | `number \| null` | 旧文件大小 |
| `newBytes` | `number \| null` | 新文件大小 |
| `introducedBytes` | `number` | 此文件变化产生的新 blob 的逻辑大小；删除或仅改名且 blob 未变化时为 0 |
| `additions` | `number \| null` | 文本新增行数，二进制为 null |
| `deletions` | `number \| null` | 文本删除行数，二进制为 null |
| `binary` | `boolean` | 是否为二进制文件 |

`introducedBytes` 不是 `.git` 目录在磁盘上的实际增长量。Git pack 压缩、delta 压缩以及相同 blob 复用都会使实际占用不同；该字段只表示新 blob 的未压缩逻辑大小。

各状态的可空字段：

| status | `oldBlob/oldBytes` | `newBlob/newBytes` | `previousPath` |
|---|---|---|---|
| `added` | null | 必需 | null |
| `modified` | 必需 | 必需 | null |
| `deleted` | 必需 | null | null |
| `renamed` | 必需 | 必需 | 必需 |
| `copied` | 必需 | 必需 | 必需 |
| `type_changed` | 视旧对象类型而定 | 视新对象类型而定 | null |

可能错误：

| HTTP | code | 说明 |
|---:|---|---|
| `400` | `INVALID_REQUEST` | SHA 格式、cursor、limit、sort 或 order 无效 |
| `404` | `NO_REPOSITORY_OPEN` | 当前没有打开仓库 |
| `404` | `COMMIT_NOT_FOUND` | SHA 对应的提交不存在于本次扫描结果中 |
| `409` | `REPOSITORY_NOT_SCANNED` | 仓库尚未完成扫描 |
| `500` | `INTERNAL_ERROR` | 无法读取提交 tree、父提交或 blob |

## 6. P2 接口

### 6.1 预览清理影响

```http
POST /api/v1/repositories/current/cleanup/preview
Content-Type: application/json
```

请求体：

```json
{
  "commitShas": [
    "8b1a9953c4611296a827abf8c47804d7f8e4f2d1"
  ],
  "expectedHead": "f6ff93b7080b50aa86061d173e2a0c0f7f20b28d"
}
```

Preview 会检查选中 commit 修改过的路径是否仍存在于 `expectedHead` 对应的 tree 中。它只用于提醒，不保证完整预测 rebase 结果，也不追踪文件后续的重命名。

响应：

```json
{
  "data": {
    "requiresConfirmation": true,
    "affectedFiles": {
      "8b1a9953c4611296a827abf8c47804d7f8e4f2d1": [
        "src/example.go",
        "assets/example.bin"
      ]
    }
  },
  "meta": {
    "requestId": "req_01J..."
  }
}
```

`affectedFiles` 按 commit SHA 分组。没有匹配路径时返回空对象且 `requiresConfirmation` 为 `false`。格式有效但不在当前扫描结果中的 SHA 会被 Preview 跳过；真正执行 Cleanup 时仍会返回 `COMMIT_NOT_FOUND`。

Preview 与 Cleanup 共用仓库状态、HEAD、可达性、根提交、merge commit 和 first-parent 检查。前端第一次点击清理时先调用 Preview：没有提醒则直接执行 Cleanup；有提醒则展示文件列表，并仅在用户点击 `Cleanup anyway` 后执行 Cleanup。

可能错误：

| HTTP | code | 说明 |
|---:|---|---|
| `400` | `INVALID_REQUEST` | SHA 格式无效、列表为空或包含重复值 |
| `404` | `NO_REPOSITORY_OPEN` | 当前没有打开仓库 |
| `409` | `REPOSITORY_NOT_SCANNED` | 仓库尚未完成扫描 |
| `409` | `REPOSITORY_CHANGED` | 当前 HEAD 与 `expectedHead` 不一致 |
| `422` | `CLEANUP_UNSUPPORTED` | detached HEAD、根提交、merge commit 或目标不在当前 first-parent 历史中 |
| `500` | `GIT_COMMAND_FAILED` | 无法读取当前 Git HEAD |
| `500` | `INTERNAL_ERROR` | 无法读取 HEAD tree 或检查文件路径 |

### 6.2 清理选中的提交历史

```http
POST /api/v1/repositories/current/cleanup
Content-Type: application/json
```

该接口从当前分支历史中删除用户选中的 commit，并重新生成这些 commit 之后的历史。它不是 `git rm`，也不是只删除工作区文件。

清理在本地仓库中同步执行，不会自动 push 或 force push。成功后，用户必须先检查重写结果，再自行更新远端。只有远端所有 branch/tag 都不再引用旧历史时，之后的 `git clone` 才不会继续下载旧对象。

请求体：

```json
{
  "commitShas": [
    "8b1a9953c4611296a827abf8c47804d7f8e4f2d1"
  ],
  "expectedHead": "f6ff93b7080b50aa86061d173e2a0c0f7f20b28d",
  "autoStash": false
}
```

| 字段 | 类型 | 必需 | 说明 |
|---|---|---:|---|
| `commitShas` | `string[]` | 是 | 需要从当前分支历史中删除的完整 40 位 SHA；不能为空或重复 |
| `expectedHead` | `string` | 是 | 用户确认时看到的完整 HEAD SHA，用于阻止在仓库已经变化后继续清理 |
| `autoStash` | `boolean` | 否 | 默认 `false`；为 `true` 时清理前临时 stash 工作区和未跟踪文件，结束后恢复 |

MVP 约束：

- 只重写当前本地分支，不处理 detached HEAD。
- 工作区和暂存区必须干净；或者请求显式设置 `autoStash: true`，由服务端临时保存并在清理后恢复。
- 仓库必须已经完成扫描，且 `expectedHead` 必须等于当前 HEAD。
- 目标 commit 必须存在于当前仓库、可从当前 HEAD 到达，并且不能是根提交或 merge commit。
- 只处理当前分支。其他 branch/tag 不会被修改，用户需要自行更新；只要它们仍引用旧历史，旧对象仍可能出现在新的 clone 中。
- 接口不承诺 `.git` 目录立即变小；本地 reflog、未回收对象和 pack 文件仍可能保留旧数据。
- 前端必须先调用 Preview，并在存在受影响路径时要求用户再次确认。

接口同步执行：HTTP 请求会一直等待历史重写和可选 stash 恢复完成，成功返回 `200 OK`，失败直接返回对应错误。

成功响应：

```json
{
  "data": {
    "previousHead": "f6ff93b7080b50aa86061d173e2a0c0f7f20b28d",
    "newHead": "4ad8e85618d744f6ef870dd15ac98e12846f22e1",
    "droppedCommitShas": [
      "8b1a9953c4611296a827abf8c47804d7f8e4f2d1"
    ]
  },
  "meta": {
    "requestId": "req_01J..."
  }
}
```

成功后，服务端必须使原扫描结果失效；前端重新获取当前仓库并提示用户再次扫描。远端更新不属于该请求。

可能错误：

| HTTP | code | 说明 |
|---:|---|---|
| `400` | `INVALID_REQUEST` | SHA 格式无效、列表为空或包含重复值 |
| `404` | `NO_REPOSITORY_OPEN` | 当前没有打开仓库 |
| `404` | `COMMIT_NOT_FOUND` | 某个 SHA 不属于当前仓库 |
| `409` | `REPOSITORY_NOT_SCANNED` | 仓库尚未完成扫描 |
| `409` | `REPOSITORY_CHANGED` | 当前 HEAD 与 `expectedHead` 不一致 |
| `409` | `WORKING_TREE_DIRTY` | 工作区或暂存区存在未提交修改 |
| `422` | `CLEANUP_UNSUPPORTED` | detached HEAD、根提交、merge commit 或目标不在当前分支历史中 |
| `500` | `GIT_COMMAND_FAILED` | Git 历史重写失败；错误响应不得包含 token 等敏感信息 |

## 7. 稳定错误码

| code | HTTP | 前端处理 |
|---|---:|---|
| `INVALID_REQUEST` | 400 | 显示字段错误 |
| `PATH_REQUIRED` | 400 | 聚焦路径输入框 |
| `REPOSITORY_NOT_FOUND` | 404 | 显示路径不存在 |
| `NO_REPOSITORY_OPEN` | 404 | 返回欢迎页 |
| `COMMIT_NOT_FOUND` | 404 | 关闭提交详情并刷新 |
| `TASK_NOT_FOUND` | 404 | 停止轮询 |
| `NOT_A_GIT_REPOSITORY` | 422 | 显示无效仓库状态 |
| `PATH_NOT_READABLE` | 422 | 显示权限错误 |
| `SCAN_ALREADY_RUNNING` | 409 | 继续跟踪已有任务 |
| `REPOSITORY_CHANGED` | 409 | 刷新仓库状态并要求用户重新确认 |
| `WORKING_TREE_DIRTY` | 409 | 要求用户先提交或暂存当前修改 |
| `CLEANUP_UNSUPPORTED` | 422 | 提示当前提交结构暂不支持自动清理 |
| `GIT_NOT_AVAILABLE` | 503 | 显示 Git 安装提示 |
| `HEALTH_CHECK_TIMEOUT` | 503 | 提示稍后重试 |
| `HEALTH_CHECK_CANCELLED` | 503 | 停止当前请求，可重新检查 |
| `HEALTH_CHECK_FAILED` | 500 | 通用健康检查错误，提供 requestId |
| `GITHUB_UNAUTHORIZED` | 401 | 提示 token 无效 |
| `GITHUB_FORBIDDEN` | 403 | 提示 token 权限不足 |
| `GITHUB_CONNECTION_FAILED` | 503 | 提示检查网络后重试 |
| `GIT_COMMAND_FAILED` | 500 | 显示 message，提供 requestId |
| `INTERNAL_ERROR` | 500 | 通用错误状态，提供 requestId |

## 8. 前端 Service 边界

建议前端定义以下与传输无关的方法：

```text
SystemService
  getHealth()

RepositoryService
  getCurrent()
  open(path)
  close()
  listRecent()
  removeRecent(repositoryId)
  startScan(force)

TaskService
  get(taskId)
  cancel(taskId)

HistoryService
  listCommits(query)
  getCommit(sha)
  listCommitFiles(sha, query)

CleanupService
  preview(commitShas, expectedHead)
  cleanup(commitShas, expectedHead, autoStash)
```

页面和 store 只调用这些 service。HTTP 阶段由 `Http*Service` 实现；Wails 阶段由 `Wails*Service` 实现，组件和 store 不需要重写。

## 9. Wails 映射原则

接入 Wails 后：

- 保留本文字段名、枚举值、错误码和分页语义。
- Wails 方法名可以使用 Go 导出命名，但返回模型必须能映射到本文 JSON 模型。
- 原生目录选择器只替代 `open(path)` 中获取 path 的方式。
- 业务逻辑放在共享应用服务中，不在 Gin handler 和 Wails binding 中各写一套。
- 长任务仍返回 `taskId`；进度可以继续轮询，也可以在之后改为 Wails Events。

建议映射：

| HTTP | Wails 方法语义 |
|---|---|
| `POST /repositories/open` | `OpenRepository(path)` |
| `GET /repositories/current` | `GetCurrentRepository()` |
| `POST /repositories/current/scans` | `StartRepositoryScan(force)` |
| `GET /tasks/{taskId}` | `GetTask(taskId)` |
| `GET /repositories/current/commits` | `ListCommits(query)` |
| `POST /repositories/current/cleanup` | `CleanupHistory(request)` |

## 10. 验收顺序

1. `/health`、统一错误结构。
2. 打开、获取、关闭仓库。
3. 最近仓库。
4. 扫描任务和进度。
5. 提交列表、详情、文件变化。
6. 提交历史清理。
7. 最后增加 Wails adapter。

每个接口至少覆盖：正常响应、空状态、无效参数、Git CLI 不可用和仓库在请求期间发生变化。

## 11. GitHub 官方参考资料

本项目的核心分析对象是本地 Git 仓库，提交遍历、对象大小和历史重写仍应优先使用系统 Git CLI。以下 GitHub 文档主要用于字段语义、远端仓库兼容和安全提示，不代表本地功能必须依赖 GitHub REST API。

### 11.1 REST API 基础

- [GitHub REST API 总览](https://docs.github.com/en/rest)
- [REST API 快速入门](https://docs.github.com/en/rest/quickstart)
- [REST API 版本控制](https://docs.github.com/en/rest/about-the-rest-api/api-versions)
- [REST API 身份验证](https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api)
- [REST API 分页](https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api)
- [REST API 限流](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
- [REST API 最佳实践](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api)

### 11.2 仓库与提交

- [Repositories API](https://docs.github.com/en/rest/repos/repos)
- [Commits API](https://docs.github.com/en/rest/commits/commits)
- [比较两个提交](https://docs.github.com/en/rest/commits/commits#compare-two-commits)
- [Repository Contents API](https://docs.github.com/en/rest/repos/contents)

### 11.3 Git 数据库对象

- [Git Database API 总览](https://docs.github.com/en/rest/git)
- [Git Commit Objects API](https://docs.github.com/en/rest/git/commits)
- [Git Blobs API](https://docs.github.com/en/rest/git/blobs)
- [Git Trees API](https://docs.github.com/en/rest/git/trees)
- [Git References API](https://docs.github.com/en/rest/git/refs)
- [Git Tags API](https://docs.github.com/en/rest/git/tags)
- [使用 REST API 操作 Git 数据库](https://docs.github.com/en/rest/guides/using-the-rest-api-to-interact-with-your-git-database)

### 11.4 历史清理与安全

- [从仓库中删除敏感数据](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository)
- [GitHub 上的大文件说明](https://docs.github.com/en/repositories/working-with-files/managing-large-files/about-large-files-on-github)
- [GitHub 仓库限制](https://docs.github.com/en/repositories/creating-and-managing-repositories/repository-limits)
- [受保护分支说明](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)

实现清理提示时，应特别参考“删除敏感数据”文档中的历史重写副作用、协作者同步、fork/clone 残留和强制推送风险。

## 12. API 实现 Checklist

完成标准：路由、Handler、Service、统一响应结构和基本自动化测试均已具备。当前 `open` 接口按 Web MVP 的标准本地工作区仓库范围验收；bare repository 和 linked worktree 暂不作为本次 PR 的阻塞项。

### P0：页面壳与仓库选择

- [x] `GET /api/v1/health`
- [x] `POST /api/v1/repositories/open`
- [x] `GET /api/v1/repositories/current`
- [x] `DELETE /api/v1/repositories/current`
- [ ] `GET /api/v1/repositories/recent`
- [ ] `DELETE /api/v1/repositories/recent/{repositoryId}`
- [x] `GET /api/v1/github/connection`（独立 GitHub 连通性接口）

### P1：提交历史分析

- [x] `POST /api/v1/repositories/current/scans`
- [x] `GET /api/v1/tasks/{taskId}`
- [x] `DELETE /api/v1/tasks/{taskId}`
- [x] `GET /api/v1/repositories/current/commits`
- [x] `GET /api/v1/repositories/current/commits` 支持 `minIntroducedBytes` 筛选
- [x] `GET /api/v1/repositories/current/commits/{sha}`
- [x] `GET /api/v1/repositories/current/commits/{sha}/files`

### P2：提交历史清理

- [x] `POST /api/v1/repositories/current/cleanup/preview`
- [x] `POST /api/v1/repositories/current/cleanup`
