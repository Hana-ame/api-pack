---
name: v2-v3-migration
description: v2→v3 迁移策略：shijima 分支为 dev，v3 应独立于 v2，正式发布需完整 DB 迁移方案
metadata:
  type: project
---

shijima 是 dev 分支。v2 API 最终会被废弃。

**Why:** v3 是新架构，不应依赖 v2 代码（handler 封装等）。数据库定义在 dev 阶段可以变更，但正式发布时必须提供从 v2 到 v3 的完整数据库迁移方案。

**How to apply:**
- 写 v3 handler 时不要包裹/调用 v2 函数，直接实现
- 数据库 DDL 变更需要记录，最终产出迁移脚本
- dev 阶段可以打破兼容，release 阶段需要迁移方案
