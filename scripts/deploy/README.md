# 部署与数据库同步

## 环境

| 环境 | 端口 | 镜像 | 主机 |
|---|---|---|---|---|
| 生产（prod） | :3010 | `ghcr.io/leewaiho/new-api:latest` | homelab `192.168.200.10` |
| 测试（test） | :3011 | `ghcr.io/leewaiho/new-api:test` | homelab `192.168.200.10` |

3011 是运行在 homelab 上的隔离 test stack，不是本地 WSL。3011 必须使用独立测试数据库，禁止直接连接生产数据库。

## 创建或重建 :3011

在 homelab 的 `/opt/newapi-test` 中操作：

1. 先启动隔离的 test stack，确保其 PostgreSQL 与 Redis 已就绪。
2. 从同一台 homelab 主机上的 :3010 PostgreSQL 生成一次快照。
3. 将该快照恢复到 3011 的独立测试数据库。
4. 验证 3011 服务状态。

恢复会完全覆盖 3011 测试数据库；仅在确认允许刷新测试数据时执行。不要让 3011 直接连接生产数据库。

## 验证

```bash
curl -s http://192.168.200.10:3011/api/status | python3 -m json.tool | head -5
```

## 更新测试镜像

当 `release/test` 有新的构建可用时，在 homelab 的 `/opt/newapi-test` 更新 test stack。更新镜像不会替代数据库快照恢复；需要刷新测试数据时，仍按“创建或重建 :3011”的隔离快照流程执行。

## 停止 / 清理

停止 test stack 前，确认不会误操作生产 stack。删除测试卷会清空 3011 测试数据；重新创建后必须恢复独立测试数据库，不能连接生产数据库。
