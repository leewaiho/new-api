# 部署与数据库同步

## 环境

| 环境 | 端口 | 镜像 | 主机 | 数据库 |
|---|---|---|---|---|
| 生产 (prod) | :3010 | `ghcr.io/leewaiho/new-api:latest` | homelab 192.168.200.10 | PostgreSQL 16 (newapi-postgres-1) |
| 测试 (test) | :3011 | `ghcr.io/leewaiho/new-api:test` | homelab 192.168.200.10 (`/opt/newapi-test`) | 独立 PostgreSQL 16 (new-api-test-pg) |

两个数据库完全独立。3011 是运行在 homelab 上的隔离 test stack，不是本地 WSL。创建、重建或刷新 3011 时，必须先启动独立 test stack，再从同一台 homelab 主机的 :3010 PostgreSQL 创建一次快照并恢复到 3011 测试数据库。恢复会完全覆盖 3011 测试数据库；禁止让 3011 直接连接 :3010 生产数据库。

## 首次部署 :3011

```bash
cd /opt/newapi-test

# 1. 启动 test stack（会自动创建空的 PostgreSQL + Redis）
docker compose -f docker-compose.yml up -d

# 2. 等待 PostgreSQL healthy
docker compose -f docker-compose.yml ps  # postgres-test 应显示 healthy

# 3. 从同一主机的 :3010 创建快照并恢复到独立 3011 测试数据库（会覆盖测试库）
./sync-db-from-prod.sh

# 4. 验证
curl -s http://192.168.200.10:3011/api/status | python3 -m json.tool | head -5
```

## 刷新测试数据

当需要把最新的生产数据同步到测试环境时，先确保隔离 test stack 已启动：

```bash
cd /opt/newapi-test
docker compose -f docker-compose.yml up -d
./sync-db-from-prod.sh
```

脚本行为：
1. 从同一台 homelab 主机的 :3010 PostgreSQL 创建快照
2. 恢复到独立的 3011 `new-api-test-pg` 容器
3. psql 恢复（`--clean --if-exists` 保证幂等，可重复执行）
4. 验证表数量

**幂等**：脚本可以重复运行，每次都会完全覆盖 3011 测试数据库的内容。不要让 3011 直接连接 :3010 生产数据库。

## 更新测试镜像

当 `release/test` 分支有新的 push，GHCR 会自动构建新的 `:test` 镜像：

```bash
cd /opt/newapi-test
docker compose -f docker-compose.yml pull
docker compose -f docker-compose.yml up -d
```

## 停止 / 清理

```bash
cd /opt/newapi-test

# 停止容器（保留数据）
docker compose -f docker-compose.yml down

# 停止并删除数据卷
docker compose -f docker-compose.yml down -v
```

## 分支策略

参见仓库根 `CLAUDE.md` 的 Branch Policy 章节。

核心规则：**所有进入 `release/prod` 的改动必须先在 `release/test` 上测试通过。**
