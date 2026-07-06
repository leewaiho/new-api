# 部署与数据库同步

## 环境

| 环境 | 端口 | 镜像 | 主机 | 数据库 |
|---|---|---|---|---|
| 生产 (prod) | :3010 | `ghcr.io/leewaiho/new-api:latest` | homelab 192.168.200.10 | PostgreSQL 16 (newapi-postgres-1) |
| 测试 (test) | :3011 | `ghcr.io/leewaiho/new-api:test` | 本地 WSL | PostgreSQL 16 (new-api-test-pg) |

两个数据库完全独立。测试环境数据库是生产数据库的副本，通过 `sync-db-from-prod.sh` 创建/刷新。

## 首次部署 :3011

```bash
cd /home/weihao/projects/new-api

# 1. 启动 test stack（会自动创建空的 PostgreSQL + Redis）
docker compose -f docker-compose.test.yml up -d

# 2. 等待 PostgreSQL healthy
docker compose -f docker-compose.test.yml ps  # postgres-test 应显示 healthy

# 3. 从 :3010 同步数据库
./scripts/deploy/sync-db-from-prod.sh

# 4. 验证
curl -s http://localhost:3011/api/status | python3 -m json.tool | head -5
```

## 刷新测试数据

当需要把最新的生产数据同步到测试环境时：

```bash
./scripts/deploy/sync-db-from-prod.sh
```

脚本行为：
1. 从 homelab 的 `newapi-postgres-1` 容器 pg_dump
2. 拷贝到本地 `new-api-test-pg` 容器
3. psql 恢复（`--clean --if-exists` 保证幂等，可重复执行）
4. 验证表数量

**幂等**：脚本可以重复运行，每次都会完全覆盖测试数据库的内容。

## 更新测试镜像

当 `release/test` 分支有新的 push，GHCR 会自动构建新的 `:test` 镜像：

```bash
docker compose -f docker-compose.test.yml pull
docker compose -f docker-compose.test.yml up -d
```

## 停止 / 清理

```bash
# 停止容器（保留数据）
docker compose -f docker-compose.test.yml down

# 停止并删除数据卷
docker compose -f docker-compose.test.yml down -v
```

## 分支策略

参见仓库根 `CLAUDE.md` 的 Branch Policy 章节。

核心规则：**所有进入 `release/prod` 的改动必须先在 `release/test` 上测试通过。**
