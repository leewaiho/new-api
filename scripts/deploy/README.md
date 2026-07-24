# 部署与数据库同步

## 环境

| 环境 | HTTP 地址 | 镜像 | 主机 |
|---|---|---|---|
| 生产（prod） | `http://192.168.200.10:3010` | `ghcr.io/leewaiho/new-api:latest` | homelab `192.168.200.10` |
| 测试（test） | `http://192.168.200.10:3011` | `ghcr.io/leewaiho/new-api:test` | homelab `192.168.200.10` |

3011 是运行在 homelab 上的隔离 test stack，不是本地 WSL。3011 必须使用独立测试数据库，禁止直接连接生产数据库。

## 创建或重建 :3011

所有操作在 homelab 的 `/opt/newapi-test` 中执行。**每次创建、重建或需要重新基线化 3011 时，必须先从 3010 PostgreSQL 快照恢复测试数据库。** 不得用陈旧的测试数据替代该步骤。

```bash
ssh homelab '
  set -euo pipefail
  cd /opt/newapi-test

  # 3011 使用独立的 PostgreSQL、Redis 和应用容器。
  docker compose up -d postgres-test redis-test new-api-test

  # 从 :3010 PostgreSQL 导出快照并恢复到 new-api-test-pg。
  ./sync-db-from-prod.sh

  # 使测试应用重新建立数据库连接。
  docker compose restart new-api-test
  docker compose ps
'
```

`sync-db-from-prod.sh` 会完全覆盖 3011 测试数据库；仅在确认允许刷新测试数据时执行。快照恢复后，E2E 所需的临时渠道、测试 Token 或模型策略只能写入 **3011 测试库**，绝不能写入 3010。

不要在 3010 目录或生产容器中执行该脚本，也不要让 3011 直接连接生产数据库。

## 验证

```bash
curl -s http://192.168.200.10:3011/api/status | python3 -m json.tool | head -5
```

## 更新测试镜像

当 `release/test` 有新的构建可用时，在 homelab 的 `/opt/newapi-test` 更新 test stack。更新镜像不会替代数据库快照恢复；需要刷新测试数据时，仍按“创建或重建 :3011”的隔离快照流程执行。

## 停止 / 清理

停止 test stack 前，确认不会误操作生产 stack。删除测试卷会清空 3011 测试数据；重新创建后必须恢复独立测试数据库，不能连接生产数据库。
