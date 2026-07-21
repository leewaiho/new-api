# 部署与数据库同步

## 环境

| 环境 | HTTP 地址 | 镜像 | 主机 / 目录 | 数据库 |
|---|---|---|---|---|
| 生产（prod） | `http://192.168.200.10:3010` | `ghcr.io/leewaiho/new-api:latest` | homelab `192.168.200.10` | PostgreSQL 16 (`newapi-postgres-1`) |
| 测试（test） | `http://192.168.200.10:3011` | `ghcr.io/leewaiho/new-api:test` | homelab `192.168.200.10` / `/opt/newapi-test` | 独立 PostgreSQL 16 (`new-api-test-pg`) |

3011 是运行在 homelab 上的隔离 test stack，不是本地 WSL。3010 与 3011 使用完全独立的 PostgreSQL、Redis 和应用容器；禁止让 3011 直接连接 3010 生产数据库。

## 创建、重建或刷新 :3011

每次创建、重建或需要重新基线化 3011 时，必须从 3010 PostgreSQL 快照恢复测试数据库；不得用陈旧的测试数据替代该步骤。所有操作在 homelab 的 `/opt/newapi-test` 中执行：

```bash
ssh homelab '
  set -euo pipefail
  cd /opt/newapi-test

  # 启动隔离 test stack，创建空的 PostgreSQL、Redis 和应用容器。
  docker compose -f docker-compose.yml up -d
  docker compose -f docker-compose.yml ps

  # 从同一主机的 :3010 PostgreSQL 导出快照并恢复到 new-api-test-pg。
  ./sync-db-from-prod.sh

  # 使测试应用重新建立数据库连接。
  docker compose -f docker-compose.yml restart new-api-test
  docker compose -f docker-compose.yml ps
'
```

`sync-db-from-prod.sh` 的恢复是幂等的：每次都会完全覆盖 3011 测试数据库（内部使用 `--clean --if-exists`）。仅在确认允许刷新测试数据时执行。

快照恢复后，E2E 所需的临时渠道、测试 Token 或模型策略只能写入 **3011 测试库**，绝不能写入 3010。不要在生产目录或生产容器中执行该脚本。

## 验证

```bash
curl -s http://192.168.200.10:3011/api/status | python3 -m json.tool | head -5
```

## 更新测试镜像

当 `release/test` 有新的 push，GHCR 会自动构建 `:test` 镜像。更新镜像不会替代数据库快照恢复；需要刷新测试数据时，仍按上述隔离快照流程执行。

```bash
ssh homelab '
  set -euo pipefail
  cd /opt/newapi-test
  docker compose -f docker-compose.yml pull
  docker compose -f docker-compose.yml up -d
'
```

## 停止 / 清理

停止 test stack 前，确认不会误操作生产 stack：

```bash
cd /opt/newapi-test

# 停止容器（保留测试数据）
docker compose -f docker-compose.yml down

# 停止并删除测试卷；下次创建后必须重新从 3010 快照恢复测试数据库。
docker compose -f docker-compose.yml down -v
```

## 分支策略

参见仓库根 `CLAUDE.md` 的 Branch Policy 章节。核心规则：**所有进入 `release/prod` 的改动必须先在 `release/test` 上测试通过。**
