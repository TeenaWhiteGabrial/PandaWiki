# 等级权限开关

管理员在「系统配置 → 权限功能」中设置“启用等级权限”。

- 关闭：不执行最低权限等级校验，隐藏等级管理、用户等级和文档/栏目最低等级设置。
- 部门管理、部门授权、个人授权、认证和 API Token 范围检查始终生效。
- 修改部门授权时保留原有用户、文档和栏目等级数据；开启后恢复等级校验。
- GET/PUT /api/v1/system/permissions 仅供系统管理员使用，enabled 只表示等级权限。
- 数据库存储键为 permission_levels；旧 fine_grained_permissions 总开关不再读取。
- backend/config.yml 的 permissions.levels_disabled 与 DISABLE_PERMISSION_LEVELS 仅作为首次初始化默认值。
- 保存后当前服务立即生效，其他进程约 1 秒内同步，已打开页面需刷新。读取失败时保留等级校验（安全优先）。

本次默认关闭等级权限，不删除部门或等级数据。
