---
name: deploy-db-immutable
description: 上传部署时禁止修改数据库内的记录
metadata:
  type: feedback
---

上传部署时不要修改数据库内的记录。

**Why:** 部署应只替换二进制/代码，不应触碰线上数据，避免意外覆盖或损坏生产数据。

**How to apply:** 任何部署、上传、发布流程中，只更新可执行文件，不执行任何 DDL（ALTER/DROP/CREATE）或 DML（INSERT/UPDATE/DELETE）操作。
