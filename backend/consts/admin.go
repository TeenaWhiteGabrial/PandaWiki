package consts

type UserKBPermission string

const (
	UserKBPermissionNull        UserKBPermission = ""             // 无权限
	UserKBPermissionNotNull     UserKBPermission = "not null"     // 有权限
	UserKBPermissionFullControl UserKBPermission = "full_control" // 完全控制
	UserKBPermissionDocManage   UserKBPermission = "doc_manage"   // 文档管理
	UserKBPermissionDataOperate UserKBPermission = "data_operate" // 数据运营
)

type UserRole string

const (
	UserRoleAdmin   UserRole = "admin"   // 超级管理员
	UserRoleManager UserRole = "manager" // 管理员
	UserRoleUser    UserRole = "user"    // 普通用户
)
