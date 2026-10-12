package i18n

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Locale string

type MessageKey string

const (
	ZhCN Locale = "zh-CN"
	EnUS Locale = "en-US"
)

const (
	KeyInternal                         MessageKey = "error.internal"
	KeyInvalidRequest                   MessageKey = "error.invalidRequest"
	KeyUnauthorized                     MessageKey = "error.unauthorized"
	KeyForbidden                        MessageKey = "error.forbidden"
	KeyNotFound                         MessageKey = "error.notFound"
	KeyConflict                         MessageKey = "error.conflict"
	KeyDependencyUnavailable            MessageKey = "error.dependencyUnavailable"
	KeyRateLimited                      MessageKey = "error.rateLimited"
	KeyUsernameConflict                 MessageKey = "auth.usernameConflict"
	KeyEmailConflict                    MessageKey = "auth.emailConflict"
	KeySuperAdminExists                 MessageKey = "auth.superAdminExists"
	KeyPermissionDenied                 MessageKey = "permission.permissionDenied"
	KeyPermissionSnapshotInvalid        MessageKey = "permission.snapshotInvalid"
	KeyAccessUpdating                   MessageKey = "permission.updating"
	KeyMenuTreeInvalid                  MessageKey = "menu.treeInvalid"
	KeyMenuNotFound                     MessageKey = "menu.notFound"
	KeyMenuCodeConflict                 MessageKey = "menu.codeConflict"
	KeyMenuPathConflict                 MessageKey = "menu.pathConflict"
	KeyMenuInvalidParent                MessageKey = "menu.invalidParent"
	KeyMenuCycleDetected                MessageKey = "menu.cycleDetected"
	KeyMenuProtected                    MessageKey = "menu.protected"
	KeyMenuParentDisabled               MessageKey = "menu.parentDisabled"
	KeyMenuStructureConflict            MessageKey = "menu.structureConflict"
	KeyMenuInvalidFields                MessageKey = "menu.invalidFields"
	KeyMenuPlatformUnavailable          MessageKey = "menu.platformUnavailable"
	KeyRoleNotFound                     MessageKey = "role.notFound"
	KeyRoleCodeConflict                 MessageKey = "role.codeConflict"
	KeyRoleNameConflict                 MessageKey = "role.nameConflict"
	KeyRoleSystemProtected              MessageKey = "role.systemProtected"
	KeyRoleDefaultProtected             MessageKey = "role.defaultProtected"
	KeyRoleUsersAttached                MessageKey = "role.usersAttached"
	KeyRoleInvalidState                 MessageKey = "role.invalidState"
	KeyRoleInvalidPermission            MessageKey = "role.invalidPermission"
	KeyRoleSuperAdminAuthorization      MessageKey = "role.superAdminAuthorization"
	KeyRoleDataInvalid                  MessageKey = "role.dataInvalid"
	KeyUserNotFound                     MessageKey = "user.notFound"
	KeyUserUsernameConflict             MessageKey = "user.usernameConflict"
	KeyUserPhoneConflict                MessageKey = "user.phoneConflict"
	KeyUserSelfOperation                MessageKey = "user.selfOperation"
	KeyUserSuperAdminProtected          MessageKey = "user.superAdminProtected"
	KeyUserLastSuperAdmin               MessageKey = "user.lastSuperAdmin"
	KeyUserInvalidRoles                 MessageKey = "user.invalidRoles"
	KeyUserRoleNotFound                 MessageKey = "user.roleNotFound"
	KeyUserDataInvalid                  MessageKey = "user.dataInvalid"
	KeyAuthPlatformNotFound             MessageKey = "authPlatform.notFound"
	KeyAuthPlatformCodeConflict         MessageKey = "authPlatform.codeConflict"
	KeyAuthPlatformBuiltinProtected     MessageKey = "authPlatform.builtinProtected"
	KeyAuthPlatformInvalidPolicy        MessageKey = "authPlatform.invalidPolicy"
	KeyAuthPlatformMenusAttached        MessageKey = "authPlatform.menusAttached"
	KeyAuthPlatformDisabled             MessageKey = "authPlatform.disabled"
	KeyAuthSessionUpdating              MessageKey = "auth.sessionUpdating"
	KeyAuthPlatformUnavailable          MessageKey = "authPlatform.unavailable"
	KeySessionInvalidID                 MessageKey = "session.invalidId"
	KeySessionNotFound                  MessageKey = "session.notFound"
	KeySessionCurrentProtected          MessageKey = "session.currentProtected"
	KeySessionQueryFailed               MessageKey = "session.queryFailed"
	KeySessionRevokeFailed              MessageKey = "session.revokeFailed"
	KeyOperationLogQueryFailed          MessageKey = "operationLog.queryFailed"
	KeyMailRecipientDenied              MessageKey = "mail.recipientDenied"
	KeyMailSendFailed                   MessageKey = "mail.sendFailed"
	KeyMailSendTimeout                  MessageKey = "mail.sendTimeout"
	KeyMailRecipientRejected            MessageKey = "mail.recipientRejected"
	KeyMailProviderLimited              MessageKey = "mail.providerLimited"
	KeyMailProviderUnavailable          MessageKey = "mail.providerUnavailable"
	KeyMailRuleImportInvalid            MessageKey = "mail.ruleImportInvalid"
	KeyMailRuleImportConflict           MessageKey = "mail.ruleImportConflict"
	KeyMailRuleExportLimit              MessageKey = "mail.ruleExportLimit"
	KeyMailRateLimitInvalid             MessageKey = "mail.rateLimitInvalid"
	KeyMailRateLimitNotFound            MessageKey = "mail.rateLimitNotFound"
	KeyMailRateLimitUnavailable         MessageKey = "mail.rateLimitUnavailable"
	KeyMailOptionStatusPending          MessageKey = "mail.options.status.pending"
	KeyMailOptionStatusSent             MessageKey = "mail.options.status.sent"
	KeyMailOptionStatusFailed           MessageKey = "mail.options.status.failed"
	KeyMailOptionScopeEmail             MessageKey = "mail.options.scope.email"
	KeyMailOptionScopeDomain            MessageKey = "mail.options.scope.domain"
	KeyMailOptionActionAllow            MessageKey = "mail.options.action.allow"
	KeyMailOptionActionDeny             MessageKey = "mail.options.action.deny"
	KeyMailOptionSceneLogin             MessageKey = "mail.options.scene.login"
	KeyMailOptionSceneForget            MessageKey = "mail.options.scene.forget"
	KeyMailOptionSceneBindEmail         MessageKey = "mail.options.scene.bindEmail"
	KeyMailOptionSceneChangePassword    MessageKey = "mail.options.scene.changePassword"
	KeyMailOptionSubjectLogin           MessageKey = "mail.options.subject.login"
	KeyMailOptionSubjectForget          MessageKey = "mail.options.subject.forget"
	KeyMailOptionSubjectBindEmail       MessageKey = "mail.options.subject.bindEmail"
	KeyMailOptionSubjectChangePassword  MessageKey = "mail.options.subject.changePassword"
	KeyMailOptionRateLimitPolicy        MessageKey = "mail.options.rateLimit.policy"
	KeyMailOptionRateLimitMode          MessageKey = "mail.options.rateLimit.mode"
	KeyMailOptionRateLimitDimension     MessageKey = "mail.options.rateLimit.dimension"
	KeySMSOptionStatusPending           MessageKey = "sms.options.status.pending"
	KeySMSOptionStatusSent              MessageKey = "sms.options.status.sent"
	KeySMSOptionStatusFailed            MessageKey = "sms.options.status.failed"
	KeySMSOptionScopePhone              MessageKey = "sms.options.scope.phone"
	KeySMSOptionScopePrefix             MessageKey = "sms.options.scope.prefix"
	KeySMSOptionActionAllow             MessageKey = "sms.options.action.allow"
	KeySMSOptionActionDeny              MessageKey = "sms.options.action.deny"
	KeySMSOptionSceneLogin              MessageKey = "sms.options.scene.login"
	KeySMSOptionSceneForget             MessageKey = "sms.options.scene.forget"
	KeySMSOptionSceneBindPhone          MessageKey = "sms.options.scene.bindPhone"
	KeySMSOptionSceneChangePassword     MessageKey = "sms.options.scene.changePassword"
	KeySMSOptionRateLimitPolicy         MessageKey = "sms.options.rateLimit.policy"
	KeySMSOptionRateLimitMode           MessageKey = "sms.options.rateLimit.mode"
	KeySMSOptionRateLimitDimension      MessageKey = "sms.options.rateLimit.dimension"
	KeyUserContactActionChange          MessageKey = "user.options.contactAction.change"
	KeyUserContactActionBind            MessageKey = "user.options.contactAction.bind"
	KeyUserSessionStatusActive          MessageKey = "user.options.sessionStatus.active"
	KeyUserSessionStatusExpired         MessageKey = "user.options.sessionStatus.expired"
	KeyUserSessionStatusRevoked         MessageKey = "user.options.sessionStatus.revoked"
	KeyUserLoginEventRegister           MessageKey = "user.options.loginEvent.register"
	KeyUserLoginEventLogin              MessageKey = "user.options.loginEvent.login"
	KeyUserLoginEventLogout             MessageKey = "user.options.loginEvent.logout"
	KeySettingValueTypeString           MessageKey = "setting.options.valueType.string"
	KeySettingValueTypeNumber           MessageKey = "setting.options.valueType.number"
	KeySettingValueTypeBool             MessageKey = "setting.options.valueType.bool"
	KeySettingValueTypeJSON             MessageKey = "setting.options.valueType.json"
	KeySettingValueTypeMediaFile        MessageKey = "setting.options.valueType.mediaFile"
	KeySettingValueTypeMediaVisual      MessageKey = "setting.options.valueType.mediaVisual"
	KeyAuthPlatformLoginEmail           MessageKey = "authPlatform.options.loginType.email"
	KeyAuthPlatformLoginPhone           MessageKey = "authPlatform.options.loginType.phone"
	KeyAuthPlatformLoginPassword        MessageKey = "authPlatform.options.loginType.password"
	KeyMenuTypeDirectory                MessageKey = "menu.options.type.directory"
	KeyMenuTypePage                     MessageKey = "menu.options.type.page"
	KeyMenuTypeAction                   MessageKey = "menu.options.type.action"
	KeyNotificationVariantInfo          MessageKey = "notification.options.variant.info"
	KeyNotificationVariantSuccess       MessageKey = "notification.options.variant.success"
	KeyNotificationVariantWarning       MessageKey = "notification.options.variant.warning"
	KeyNotificationVariantError         MessageKey = "notification.options.variant.error"
	KeyNotificationPriorityNormal       MessageKey = "notification.options.priority.normal"
	KeyNotificationPriorityUrgent       MessageKey = "notification.options.priority.urgent"
	KeyNotificationTaskStatusDraft      MessageKey = "notificationTask.options.status.draft"
	KeyNotificationTaskStatusScheduled  MessageKey = "notificationTask.options.status.scheduled"
	KeyNotificationTaskStatusQueued     MessageKey = "notificationTask.options.status.queued"
	KeyNotificationTaskStatusProcessing MessageKey = "notificationTask.options.status.processing"
	KeyNotificationTaskStatusCompleted  MessageKey = "notificationTask.options.status.completed"
	KeyNotificationTaskStatusFailed     MessageKey = "notificationTask.options.status.failed"
	KeyNotificationTaskStatusCanceled   MessageKey = "notificationTask.options.status.canceled"
	KeyNotificationTaskAudienceUser     MessageKey = "notificationTask.options.audience.user"
	KeyNotificationTaskAudienceRole     MessageKey = "notificationTask.options.audience.role"
	KeyNotificationTaskAudiencePlatform MessageKey = "notificationTask.options.audience.platform"
	KeyNotificationTaskLinkNone         MessageKey = "notificationTask.options.link.none"
	KeyNotificationTaskLinkInternal     MessageKey = "notificationTask.options.link.internal"
	KeyNotificationTaskLinkExternal     MessageKey = "notificationTask.options.link.external"
	KeySchedulerErrorTimeout            MessageKey = "scheduler.error.timeout"
	KeySchedulerErrorCanceled           MessageKey = "scheduler.error.canceled"
	KeySchedulerErrorTaskFailed         MessageKey = "scheduler.error.taskFailed"
	KeySchedulerErrorUnknownTask        MessageKey = "scheduler.error.unknownTask"
	KeySchedulerErrorInvalidPayload     MessageKey = "scheduler.error.invalidPayload"
	KeySchedulerErrorWorkerLost         MessageKey = "scheduler.error.workerLost"
	KeySchedulerErrorPublish            MessageKey = "scheduler.error.publish"
	KeySchedulerErrorTransportLost      MessageKey = "scheduler.error.transportLost"
	KeySchedulerJobScheduled            MessageKey = "scheduler.options.jobStatus.scheduled"
	KeySchedulerJobQueued               MessageKey = "scheduler.options.jobStatus.queued"
	KeySchedulerJobRunning              MessageKey = "scheduler.options.status.running"
	KeySchedulerJobCompleted            MessageKey = "scheduler.options.jobStatus.completed"
	KeySchedulerJobFailed               MessageKey = "scheduler.options.status.failed"
	KeySchedulerJobCanceled             MessageKey = "scheduler.options.jobStatus.canceled"
	KeySchedulerRunSucceeded            MessageKey = "scheduler.options.runStatus.succeeded"
	KeySchedulerScheduleDisabled        MessageKey = "scheduler.options.scheduleStatus.disabled"
	KeySchedulerScheduleEnabled         MessageKey = "scheduler.options.scheduleStatus.enabled"
	KeySchedulerTriggerCron             MessageKey = "scheduler.options.trigger.cron"
	KeySchedulerTriggerManual           MessageKey = "scheduler.options.trigger.manual"
	KeySchedulerTriggerRetry            MessageKey = "scheduler.options.trigger.retry"
	KeySchedulerTriggerBusiness         MessageKey = "scheduler.options.trigger.business"
	KeySchedulerCronEveryMinute         MessageKey = "scheduler.options.cron.everyMinute"
	KeySchedulerCronEvery5Minutes       MessageKey = "scheduler.options.cron.every5Minutes"
	KeySchedulerCronEvery10Minutes      MessageKey = "scheduler.options.cron.every10Minutes"
	KeySchedulerCronEvery15Minutes      MessageKey = "scheduler.options.cron.every15Minutes"
	KeySchedulerCronEvery30Minutes      MessageKey = "scheduler.options.cron.every30Minutes"
	KeySchedulerCronHourly              MessageKey = "scheduler.options.cron.hourly"
	KeySchedulerCronDaily               MessageKey = "scheduler.options.cron.daily"
	KeySchedulerCronDaily0330           MessageKey = "scheduler.options.cron.daily0330"
	KeySchedulerCronWeekly              MessageKey = "scheduler.options.cron.weekly"
	KeySchedulerCronMonthly             MessageKey = "scheduler.options.cron.monthly"
	KeySchedulerTaskNotificationBatch   MessageKey = "scheduler.tasks.message.notificationtask.batch"
	KeySchedulerTaskRealtimeCleanup     MessageKey = "scheduler.tasks.realtime.retention.cleanup"
	KeySchedulerTaskNotificationCleanup MessageKey = "scheduler.tasks.message.notification.retention.cleanup"
	KeySchedulerTaskHistoryCleanup      MessageKey = "scheduler.tasks.system.scheduler.history.cleanup"
	KeyCachePublishReady                MessageKey = "configCacheStatus.options.publishState.ready"
	KeyCachePublishPending              MessageKey = "configCacheStatus.options.publishState.pending"
	KeyCachePublishRetrying             MessageKey = "configCacheStatus.options.publishState.retrying"
	KeyCacheNamespaceSystem             MessageKey = "configCacheStatus.options.namespace.system"
	KeyCacheNamespaceMail               MessageKey = "configCacheStatus.options.namespace.mail"
	KeyCacheNamespaceSMS                MessageKey = "configCacheStatus.options.namespace.sms"
	KeyCacheNamespaceCOS                MessageKey = "configCacheStatus.options.namespace.cos"
	KeyCacheScopeGlobal                 MessageKey = "configCacheStatus.options.scope.global"
	KeyCacheScopeCOS                    MessageKey = "configCacheStatus.options.scope.cos"
	KeyCacheScopeOther                  MessageKey = "configCacheStatus.options.scope.other"
	KeyCacheStatusReady                 MessageKey = "configCacheStatus.options.status.ready"
	KeyCacheStatusPending               MessageKey = "configCacheStatus.options.status.pending"
	KeyCacheStatusRetrying              MessageKey = "configCacheStatus.options.status.retrying"
	KeyCacheStatusInvalidating          MessageKey = "configCacheStatus.options.status.invalidating"
	KeyCacheStatusMissing               MessageKey = "configCacheStatus.options.status.missing"
	KeyCacheStatusMissingHint           MessageKey = "configCacheStatus.options.status.missingHint"
	KeyCacheStatusCorrupt               MessageKey = "configCacheStatus.options.status.corrupt"
	KeyCacheStatusUnavailable           MessageKey = "configCacheStatus.options.status.unavailable"
	KeyLoginTypeEmail                   MessageKey = "loginType.email"
	KeyLoginTypePhone                   MessageKey = "loginType.phone"
	KeyLoginTypePassword                MessageKey = "loginType.password"
	KeyAuthPlatformMaxSessionsUnlimited MessageKey = "authPlatform.presentation.maxSessions.unlimited"
	KeyAuthPlatformMaxSessionsSingle    MessageKey = "authPlatform.presentation.maxSessions.single"
	KeyAuthPlatformMaxSessionsMany      MessageKey = "authPlatform.presentation.maxSessions.many"
	KeyAuthPlatformDeleteReasonBuiltin  MessageKey = "authPlatform.presentation.deleteReason.builtin"
	KeyMenuVisibilityHidden             MessageKey = "menu.presentation.visibility.hidden"
	KeyMenuVisibilityVisible            MessageKey = "menu.presentation.visibility.visible"
	KeyMenuStatusDisabled               MessageKey = "menu.presentation.status.disabled"
	KeyMenuStatusEnabled                MessageKey = "menu.presentation.status.enabled"
	KeyMenuActionEnable                 MessageKey = "menu.presentation.action.enable"
	KeyMenuActionDisable                MessageKey = "menu.presentation.action.disable"
	KeyMenuProtectedReason              MessageKey = "menu.presentation.protected"
	KeyMenuParentDisabledReason         MessageKey = "menu.presentation.parentDisabled"
)

var catalogs = map[Locale]map[MessageKey]string{
	ZhCN: {
		"storage.file.category.image":        "图片",
		"storage.file.category.video":        "视频",
		"storage.file.category.audio":        "音频",
		"storage.file.category.document":     "文档",
		"storage.file.category.other":        "其他",
		"options.userGender.0":               "未知",
		"options.userGender.1":               "男",
		"options.userGender.2":               "女",
		"options.cosRegion.ap-guangzhou":     "广州（ap-guangzhou）",
		"options.cosRegion.ap-shanghai":      "上海（ap-shanghai）",
		"options.cosRegion.ap-nanjing":       "南京（ap-nanjing）",
		"options.cosRegion.ap-beijing":       "北京（ap-beijing）",
		"options.cosRegion.ap-chengdu":       "成都（ap-chengdu）",
		"options.cosRegion.ap-chongqing":     "重庆（ap-chongqing）",
		"options.cosRegion.ap-hongkong":      "中国香港（ap-hongkong）",
		"options.cosRegion.ap-singapore":     "新加坡（ap-singapore）",
		"options.cosRegion.ap-tokyo":         "东京（ap-tokyo）",
		"options.cosRegion.ap-seoul":         "首尔（ap-seoul）",
		"options.cosRegion.eu-frankfurt":     "法兰克福（eu-frankfurt）",
		"options.cosRegion.na-siliconvalley": "硅谷（na-siliconvalley）",
		"options.mailRegion.ap-guangzhou":    "广州（ap-guangzhou）",
		"options.mailRegion.ap-hongkong":     "中国香港（ap-hongkong）",
		"options.smsRegion.ap-guangzhou":     "广州",
		KeyMailRuleImportInvalid:             "XLSX 有错误，请重新预览并修正后导入，整批未写入",
		KeyMailRuleImportConflict:            "XLSX 中有重复或已存在的收件规则，请重新预览，整批未写入",
		KeyMailRuleExportLimit:               "XLSX 导出上限为 1000 条、2 MiB，未生成不完整文件",
		KeyInternal:                          "服务内部错误",
		KeyInvalidRequest:                    "请求参数错误",
		KeyUnauthorized:                      "未登录或登录已失效",
		KeyForbidden:                         "无权执行该操作",
		KeyNotFound:                          "请求的资源不存在",
		KeyConflict:                          "数据冲突",
		KeyDependencyUnavailable:             "服务暂未就绪",
		KeyRateLimited:                       "请求过于频繁",
		KeyUsernameConflict:                  "用户名已存在",
		KeyEmailConflict:                     "邮箱已存在",
		KeySuperAdminExists:                  "超级管理员已存在",
		KeyPermissionDenied:                  "无权执行 {{permission}}",
		KeyPermissionSnapshotInvalid:         "访问权限数据无效",
		KeyAccessUpdating:                    "访问权限正在更新",
		KeyMenuTreeInvalid:                   "菜单树数据无效",
		KeyMenuNotFound:                      "菜单不存在",
		KeyMenuCodeConflict:                  "菜单编码 {{code}} 已存在",
		KeyMenuPathConflict:                  "页面路径 {{path}} 已存在",
		KeyMenuInvalidParent:                 "父菜单无效",
		KeyMenuCycleDetected:                 "菜单层级存在循环",
		KeyMenuProtected:                     "基础菜单 {{code}} 不允许执行该操作",
		KeyMenuParentDisabled:                "菜单 {{code}} 的父级未全部启用",
		KeyMenuStructureConflict:             "菜单 {{code}} 的结构与现有子节点或授权冲突",
		KeyMenuInvalidFields:                 "菜单字段无效",
		KeyMenuPlatformUnavailable:           "菜单所属平台不存在或已删除",
		KeyRoleNotFound:                      "角色不存在",
		KeyRoleCodeConflict:                  "角色编码 {{code}} 已存在",
		KeyRoleNameConflict:                  "角色名称 {{name}} 已存在",
		KeyRoleSystemProtected:               "系统角色 {{code}} 不允许执行该操作",
		KeyRoleDefaultProtected:              "默认角色 {{code}} 不允许执行该操作",
		KeyRoleUsersAttached:                 "角色 {{code}} 仍绑定用户",
		KeyRoleInvalidState:                  "角色状态无效",
		KeyRoleInvalidPermission:             "授权菜单无效",
		KeyRoleSuperAdminAuthorization:       "超级管理员不允许配置角色授权",
		KeyRoleDataInvalid:                   "角色权限数据无效",
		KeyUserNotFound:                      "用户不存在",
		KeyUserUsernameConflict:              "用户名已存在",
		KeyUserPhoneConflict:                 "手机号已存在",
		KeyUserSelfOperation:                 "不能对自己的账号执行该操作",
		KeyUserSuperAdminProtected:           "只有超级管理员可以操作超级管理员账号",
		KeyUserLastSuperAdmin:                "系统必须保留至少一个有效超级管理员",
		KeyUserInvalidRoles:                  "用户角色集合无效",
		KeyUserRoleNotFound:                  "角色不存在",
		KeyUserDataInvalid:                   "用户角色数据无效",
		KeyAuthPlatformNotFound:              "认证平台不存在",
		KeyAuthPlatformCodeConflict:          "认证平台编码已存在",
		KeyAuthPlatformBuiltinProtected:      "内置认证平台不允许删除",
		KeyAuthPlatformInvalidPolicy:         "认证平台策略无效",
		KeyAuthPlatformMenusAttached:         "认证平台仍有关联菜单，请先删除菜单",
		KeyAuthPlatformDisabled:              "认证平台已停用",
		KeyAuthSessionUpdating:               "会话状态正在更新",
		KeyAuthPlatformUnavailable:           "认证平台服务暂不可用",
		KeySessionInvalidID:                  "会话 ID 无效",
		KeySessionNotFound:                   "会话不存在",
		KeySessionCurrentProtected:           "当前登录会话不能被踢出",
		KeySessionQueryFailed:                "会话查询失败",
		KeySessionRevokeFailed:               "会话撤销失败",
		KeyOperationLogQueryFailed:           "操作日志查询失败",
		KeyMailRecipientDenied:               "收件邮箱被收件规则拒绝",
		KeyMailSendFailed:                    "邮件发送失败，请稍后重试；如持续失败，请联系管理员",
		KeyMailSendTimeout:                   "邮件发送超时，请稍后重试",
		KeyMailRecipientRejected:             "邮件服务商拒绝向该邮箱发送邮件，请检查邮箱地址或更换邮箱",
		KeyMailProviderLimited:               "邮件服务商发送频率受限，请稍后重试",
		KeyMailProviderUnavailable:           "邮件发送服务暂不可用，请联系管理员处理",
		KeyMailRateLimitInvalid:              "邮件限流策略参数无效",
		KeyMailRateLimitNotFound:             "邮件限流策略不存在",
		KeyMailRateLimitUnavailable:          "邮件限流策略暂不可用",
		KeyMailOptionStatusPending:           "待发送",
		KeyMailOptionStatusSent:              "已发送",
		KeyMailOptionStatusFailed:            "发送失败",
		KeyMailOptionScopeEmail:              "邮箱",
		KeyMailOptionScopeDomain:             "域名",
		KeyMailOptionActionAllow:             "允许",
		KeyMailOptionActionDeny:              "拒绝",
		KeyMailOptionSceneLogin:              "登录验证码",
		KeyMailOptionSceneForget:             "找回密码",
		KeyMailOptionSceneBindEmail:          "绑定/换绑邮箱",
		KeyMailOptionSceneChangePassword:     "验证码改密",
		KeyMailOptionSubjectLogin:            "登录验证码",
		KeyMailOptionSubjectForget:           "找回密码验证码",
		KeyMailOptionSubjectBindEmail:        "绑定邮箱验证码",
		KeyMailOptionSubjectChangePassword:   "修改密码验证码",
		KeyMailOptionRateLimitPolicy:         "业务邮箱发送 · {{windowSeconds}} 秒窗口",
		KeyMailOptionRateLimitMode:           "业务发送",
		KeyMailOptionRateLimitDimension:      "认证平台 + 邮箱",
		KeyUserContactActionChange:           "换绑",
		KeyUserContactActionBind:             "绑定",
		KeyUserSessionStatusActive:           "活跃",
		KeyUserSessionStatusExpired:          "已过期",
		KeyUserSessionStatusRevoked:          "已撤销",
		KeyUserLoginEventRegister:            "注册",
		KeyUserLoginEventLogin:               "登录",
		KeyUserLoginEventLogout:              "退出登录",
		KeySettingValueTypeString:            "字符串",
		KeySettingValueTypeNumber:            "数字",
		KeySettingValueTypeBool:              "布尔",
		KeySettingValueTypeJSON:              "JSON",
		KeySettingValueTypeMediaFile:         "媒体-文件",
		KeySettingValueTypeMediaVisual:       "媒体-图片/视频",
		KeyAuthPlatformLoginEmail:            "邮箱验证码",
		KeyAuthPlatformLoginPhone:            "手机验证码",
		KeyAuthPlatformLoginPassword:         "密码",
		KeyMenuTypeDirectory:                 "目录",
		KeyMenuTypePage:                      "页面",
		KeyMenuTypeAction:                    "按钮权限",
		KeyNotificationVariantInfo:           "信息",
		KeyNotificationVariantSuccess:        "成功",
		KeyNotificationVariantWarning:        "警告",
		KeyNotificationVariantError:          "错误",
		KeyNotificationPriorityNormal:        "普通",
		KeyNotificationPriorityUrgent:        "紧急",
		KeyNotificationTaskStatusDraft:       "草稿",
		KeyNotificationTaskStatusScheduled:   "待定时发送",
		KeyNotificationTaskStatusQueued:      "排队中",
		KeyNotificationTaskStatusProcessing:  "发送中",
		KeyNotificationTaskStatusCompleted:   "已完成",
		KeyNotificationTaskStatusFailed:      "失败",
		KeyNotificationTaskStatusCanceled:    "已取消",
		KeyNotificationTaskAudienceUser:      "指定用户",
		KeyNotificationTaskAudienceRole:      "指定角色",
		KeyNotificationTaskAudiencePlatform:  "整个平台",
		KeyNotificationTaskLinkNone:          "无链接",
		KeyNotificationTaskLinkInternal:      "站内链接",
		KeyNotificationTaskLinkExternal:      "外部链接",
		KeySchedulerErrorTimeout:             "任务超过允许的执行时间",
		KeySchedulerErrorCanceled:            "任务执行被中断",
		KeySchedulerErrorTaskFailed:          "任务执行失败，请检查任务配置与依赖服务",
		KeySchedulerErrorUnknownTask:         "任务类型未注册，请检查 API 与 Worker 的部署版本",
		KeySchedulerErrorInvalidPayload:      "任务参数与当前任务定义不匹配",
		KeySchedulerErrorWorkerLost:          "执行节点未在规定时间内完成，本次执行已结束",
		KeySchedulerErrorPublish:             "任务投递失败，请检查队列服务",
		KeySchedulerErrorTransportLost:       "队列任务投递超时，系统已安排重新投递",
		KeySchedulerJobScheduled:             "待执行",
		KeySchedulerJobQueued:                "排队中",
		KeySchedulerJobRunning:               "执行中",
		KeySchedulerJobCompleted:             "执行成功",
		KeySchedulerJobFailed:                "执行失败",
		KeySchedulerJobCanceled:              "已取消",
		KeySchedulerRunSucceeded:             "执行成功",
		KeySchedulerScheduleDisabled:         "停用",
		KeySchedulerScheduleEnabled:          "启用",
		KeySchedulerTriggerCron:              "定时触发",
		KeySchedulerTriggerManual:            "手动触发",
		KeySchedulerTriggerRetry:             "重试",
		KeySchedulerTriggerBusiness:          "业务触发",
		KeySchedulerCronEveryMinute:          "每分钟",
		KeySchedulerCronEvery5Minutes:        "每 5 分钟",
		KeySchedulerCronEvery10Minutes:       "每 10 分钟",
		KeySchedulerCronEvery15Minutes:       "每 15 分钟",
		KeySchedulerCronEvery30Minutes:       "每 30 分钟",
		KeySchedulerCronHourly:               "每小时",
		KeySchedulerCronDaily:                "每天零点",
		KeySchedulerCronDaily0330:            "每天 03:30",
		KeySchedulerCronWeekly:               "每周一零点",
		KeySchedulerCronMonthly:              "每月 1 日零点",
		KeySchedulerTaskNotificationBatch:    "站内通知批次",
		KeySchedulerTaskRealtimeCleanup:      "实时事件历史清理",
		KeySchedulerTaskNotificationCleanup:  "站内通知历史清理",
		KeySchedulerTaskHistoryCleanup:       "调度器历史清理",
		KeyCachePublishReady:                 "已发布",
		KeyCachePublishPending:               "待发布",
		KeyCachePublishRetrying:              "重试中",
		KeyCacheNamespaceSystem:              "系统设置",
		KeyCacheNamespaceMail:                "邮件服务",
		KeyCacheNamespaceSMS:                 "短信服务",
		KeyCacheNamespaceCOS:                 "对象存储配置",
		KeyCacheScopeGlobal:                  "全局",
		KeyCacheScopeCOS:                     "对象存储配置 {{scope}}",
		KeyCacheScopeOther:                   "其他范围（{{scope}}）",
		KeyCacheStatusReady:                  "就绪",
		KeyCacheStatusPending:                "待发布",
		KeyCacheStatusRetrying:               "重试中",
		KeyCacheStatusInvalidating:           "变更中",
		KeyCacheStatusMissing:                "等待缓存重建",
		KeyCacheStatusMissingHint:            "Redis 状态为空，访问配置时会按现有协议自动重建。",
		KeyCacheStatusCorrupt:                "状态损坏",
		KeyCacheStatusUnavailable:            "Redis 不可用",
		KeySMSOptionStatusPending:            "待发送",
		KeySMSOptionStatusSent:               "已发送",
		KeySMSOptionStatusFailed:             "发送失败",
		KeySMSOptionScopePhone:               "手机号",
		KeySMSOptionScopePrefix:              "号段",
		KeySMSOptionActionAllow:              "允许",
		KeySMSOptionActionDeny:               "拒绝",
		KeySMSOptionSceneLogin:               "登录验证码",
		KeySMSOptionSceneForget:              "找回密码",
		KeySMSOptionSceneBindPhone:           "绑定/换绑手机号",
		KeySMSOptionSceneChangePassword:      "验证码改密",
		KeySMSOptionRateLimitPolicy:          "业务手机号发送 · {{windowSeconds}} 秒窗口",
		KeySMSOptionRateLimitMode:            "业务发送",
		KeySMSOptionRateLimitDimension:       "认证平台 + 手机号",
		KeyLoginTypeEmail:                    "邮箱验证码",
		KeyLoginTypePhone:                    "手机验证码",
		KeyLoginTypePassword:                 "账号密码",
		KeyAuthPlatformMaxSessionsUnlimited:  "不限",
		KeyAuthPlatformMaxSessionsSingle:     "单会话",
		KeyAuthPlatformMaxSessionsMany:       "最多 {{count}} 个",
		KeyAuthPlatformDeleteReasonBuiltin:   "内置认证平台不可删除",
		KeyMenuVisibilityHidden:              "隐藏",
		KeyMenuVisibilityVisible:             "显示",
		KeyMenuStatusDisabled:                "停用",
		KeyMenuStatusEnabled:                 "启用",
		KeyMenuActionEnable:                  "启用",
		KeyMenuActionDisable:                 "停用",
		KeyMenuProtectedReason:               "受保护菜单不可删除或禁用，仍可编辑展示信息和新增子节点",
		KeyMenuParentDisabledReason:          "请先启用父级菜单",
	},
	EnUS: {
		"storage.file.category.image":        "Images",
		"storage.file.category.video":        "Videos",
		"storage.file.category.audio":        "Audio",
		"storage.file.category.document":     "Documents",
		"storage.file.category.other":        "Other",
		"options.userGender.0":               "Unknown",
		"options.userGender.1":               "Male",
		"options.userGender.2":               "Female",
		"options.cosRegion.ap-guangzhou":     "Guangzhou (ap-guangzhou)",
		"options.cosRegion.ap-shanghai":      "Shanghai (ap-shanghai)",
		"options.cosRegion.ap-nanjing":       "Nanjing (ap-nanjing)",
		"options.cosRegion.ap-beijing":       "Beijing (ap-beijing)",
		"options.cosRegion.ap-chengdu":       "Chengdu (ap-chengdu)",
		"options.cosRegion.ap-chongqing":     "Chongqing (ap-chongqing)",
		"options.cosRegion.ap-hongkong":      "Hong Kong, China (ap-hongkong)",
		"options.cosRegion.ap-singapore":     "Singapore (ap-singapore)",
		"options.cosRegion.ap-tokyo":         "Tokyo (ap-tokyo)",
		"options.cosRegion.ap-seoul":         "Seoul (ap-seoul)",
		"options.cosRegion.eu-frankfurt":     "Frankfurt (eu-frankfurt)",
		"options.cosRegion.na-siliconvalley": "Silicon Valley (na-siliconvalley)",
		"options.mailRegion.ap-guangzhou":    "Guangzhou (ap-guangzhou)",
		"options.mailRegion.ap-hongkong":     "Hong Kong, China (ap-hongkong)",
		"options.smsRegion.ap-guangzhou":     "Guangzhou",
		KeyMailRuleImportInvalid:             "The XLSX is invalid. Preview and correct it before importing. No rows were written",
		KeyMailRuleImportConflict:            "Duplicate or existing recipient rules found in the XLSX. Preview again. No rows were written",
		KeyMailRuleExportLimit:               "XLSX export is limited to 1000 rules and 2 MiB. No partial file was generated",
		KeyInternal:                          "Internal server error",
		KeyInvalidRequest:                    "Invalid request",
		KeyUnauthorized:                      "Authentication is required or has expired",
		KeyForbidden:                         "Permission denied",
		KeyNotFound:                          "Resource not found",
		KeyConflict:                          "Data conflict",
		KeyDependencyUnavailable:             "Service is temporarily unavailable",
		KeyRateLimited:                       "Too many requests",
		KeyUsernameConflict:                  "Username already exists",
		KeyEmailConflict:                     "Email already exists",
		KeySuperAdminExists:                  "A super administrator already exists",
		KeyPermissionDenied:                  "Permission denied: {{permission}}",
		KeyPermissionSnapshotInvalid:         "Access permission data is invalid",
		KeyAccessUpdating:                    "Access permissions are updating",
		KeyMenuTreeInvalid:                   "Menu tree data is invalid",
		KeyMenuNotFound:                      "Menu not found",
		KeyMenuCodeConflict:                  "Menu code {{code}} already exists",
		KeyMenuPathConflict:                  "Page path {{path}} already exists",
		KeyMenuInvalidParent:                 "Invalid parent menu",
		KeyMenuCycleDetected:                 "Menu hierarchy contains a cycle",
		KeyMenuProtected:                     "Protected menu {{code}} cannot be changed by this operation",
		KeyMenuParentDisabled:                "The parent hierarchy of menu {{code}} is not fully enabled",
		KeyMenuStructureConflict:             "Menu {{code}} conflicts with existing children or grants",
		KeyMenuInvalidFields:                 "Invalid menu fields",
		KeyMenuPlatformUnavailable:           "The menu platform does not exist or has been deleted",
		KeyRoleNotFound:                      "Role not found",
		KeyRoleCodeConflict:                  "Role code {{code}} already exists",
		KeyRoleNameConflict:                  "Role name {{name}} already exists",
		KeyRoleSystemProtected:               "System role {{code}} cannot be changed by this operation",
		KeyRoleDefaultProtected:              "Default role {{code}} cannot be changed by this operation",
		KeyRoleUsersAttached:                 "Role {{code}} still has assigned users",
		KeyRoleInvalidState:                  "Invalid role state",
		KeyRoleInvalidPermission:             "Invalid permission menu",
		KeyRoleSuperAdminAuthorization:       "Super administrator permissions cannot be configured",
		KeyRoleDataInvalid:                   "Role permission data is invalid",
		KeyUserNotFound:                      "User not found",
		KeyUserUsernameConflict:              "Username already exists",
		KeyUserPhoneConflict:                 "Phone number already exists",
		KeyUserSelfOperation:                 "This operation cannot be performed on your own account",
		KeyUserSuperAdminProtected:           "Only a super administrator can operate on a super administrator account",
		KeyUserLastSuperAdmin:                "At least one effective super administrator must remain",
		KeyUserInvalidRoles:                  "Invalid user role set",
		KeyUserRoleNotFound:                  "Role not found",
		KeyUserDataInvalid:                   "User role data is invalid",
		KeyAuthPlatformNotFound:              "Authentication platform not found",
		KeyAuthPlatformCodeConflict:          "Authentication platform code already exists",
		KeyAuthPlatformBuiltinProtected:      "Builtin authentication platform cannot be deleted",
		KeyAuthPlatformInvalidPolicy:         "Invalid authentication platform policy",
		KeyAuthPlatformMenusAttached:         "Authentication platform still has active menus",
		KeyAuthPlatformDisabled:              "Authentication platform is disabled",
		KeyAuthSessionUpdating:               "Authentication session state is updating",
		KeyAuthPlatformUnavailable:           "Authentication platform service is temporarily unavailable",
		KeySessionInvalidID:                  "Invalid session ID",
		KeySessionNotFound:                   "Session not found",
		KeySessionCurrentProtected:           "The current login session cannot be revoked",
		KeySessionQueryFailed:                "Failed to query sessions",
		KeySessionRevokeFailed:               "Failed to revoke session",
		KeyOperationLogQueryFailed:           "Failed to query operation logs",
		KeyMailRecipientDenied:               "Recipient rejected by mail rule",
		KeyMailSendFailed:                    "Email could not be sent. Please try again later or contact an administrator if the problem persists",
		KeyMailSendTimeout:                   "Email sending timed out. Please try again later",
		KeyMailRecipientRejected:             "The email provider rejected this recipient. Please check the email address or use another one",
		KeyMailProviderLimited:               "The email provider is rate limiting delivery. Please try again later",
		KeyMailProviderUnavailable:           "The email sending service is unavailable. Please contact an administrator",
		KeyMailRateLimitInvalid:              "Mail rate limit policy parameters are invalid",
		KeyMailRateLimitNotFound:             "Mail rate limit policy not found",
		KeyMailRateLimitUnavailable:          "Mail rate limit policy is unavailable",
		KeyMailOptionStatusPending:           "Pending",
		KeyMailOptionStatusSent:              "Sent",
		KeyMailOptionStatusFailed:            "Failed",
		KeyMailOptionScopeEmail:              "Email",
		KeyMailOptionScopeDomain:             "Domain",
		KeyMailOptionActionAllow:             "Allow",
		KeyMailOptionActionDeny:              "Deny",
		KeyMailOptionSceneLogin:              "Login verification code",
		KeyMailOptionSceneForget:             "Password recovery",
		KeyMailOptionSceneBindEmail:          "Bind or change email",
		KeyMailOptionSceneChangePassword:     "Change password",
		KeyMailOptionSubjectLogin:            "Login verification code",
		KeyMailOptionSubjectForget:           "Password recovery code",
		KeyMailOptionSubjectBindEmail:        "Email binding verification code",
		KeyMailOptionSubjectChangePassword:   "Password change verification code",
		KeyMailOptionRateLimitPolicy:         "Business email sending · {{windowSeconds}}s window",
		KeyMailOptionRateLimitMode:           "Business sending",
		KeyMailOptionRateLimitDimension:      "Authentication platform + email",
		KeyUserContactActionChange:           "Change",
		KeyUserContactActionBind:             "Bind",
		KeyUserSessionStatusActive:           "Active",
		KeyUserSessionStatusExpired:          "Expired",
		KeyUserSessionStatusRevoked:          "Revoked",
		KeyUserLoginEventRegister:            "Registration",
		KeyUserLoginEventLogin:               "Login",
		KeyUserLoginEventLogout:              "Logout",
		KeySettingValueTypeString:            "String",
		KeySettingValueTypeNumber:            "Number",
		KeySettingValueTypeBool:              "Boolean",
		KeySettingValueTypeJSON:              "JSON",
		KeySettingValueTypeMediaFile:         "Media - File",
		KeySettingValueTypeMediaVisual:       "Media - Image/Video",
		KeyAuthPlatformLoginEmail:            "Email code",
		KeyAuthPlatformLoginPhone:            "Phone code",
		KeyAuthPlatformLoginPassword:         "Password",
		KeyMenuTypeDirectory:                 "Directory",
		KeyMenuTypePage:                      "Page",
		KeyMenuTypeAction:                    "Action permission",
		KeyNotificationVariantInfo:           "Info",
		KeyNotificationVariantSuccess:        "Success",
		KeyNotificationVariantWarning:        "Warning",
		KeyNotificationVariantError:          "Error",
		KeyNotificationPriorityNormal:        "Normal",
		KeyNotificationPriorityUrgent:        "Urgent",
		KeyNotificationTaskStatusDraft:       "Draft",
		KeyNotificationTaskStatusScheduled:   "Scheduled",
		KeyNotificationTaskStatusQueued:      "Queued",
		KeyNotificationTaskStatusProcessing:  "Processing",
		KeyNotificationTaskStatusCompleted:   "Completed",
		KeyNotificationTaskStatusFailed:      "Failed",
		KeyNotificationTaskStatusCanceled:    "Canceled",
		KeyNotificationTaskAudienceUser:      "Users",
		KeyNotificationTaskAudienceRole:      "Roles",
		KeyNotificationTaskAudiencePlatform:  "Platform",
		KeyNotificationTaskLinkNone:          "No link",
		KeyNotificationTaskLinkInternal:      "Internal",
		KeyNotificationTaskLinkExternal:      "External",
		KeySchedulerErrorTimeout:             "The task exceeded its allowed execution time",
		KeySchedulerErrorCanceled:            "Task execution was interrupted",
		KeySchedulerErrorTaskFailed:          "Task execution failed; check the task configuration and dependent services",
		KeySchedulerErrorUnknownTask:         "Task type is not registered; check the API and Worker deployment versions",
		KeySchedulerErrorInvalidPayload:      "Task parameters do not match the current task definition",
		KeySchedulerErrorWorkerLost:          "The worker did not finish within its execution lease; this attempt has ended",
		KeySchedulerErrorPublish:             "Task delivery failed; check the queue service",
		KeySchedulerErrorTransportLost:       "Queue delivery expired; the task has been scheduled for delivery again",
		KeySchedulerJobScheduled:             "Pending",
		KeySchedulerJobQueued:                "In queue",
		KeySchedulerJobRunning:               "Running",
		KeySchedulerJobCompleted:             "Succeeded",
		KeySchedulerJobFailed:                "Failed",
		KeySchedulerJobCanceled:              "Canceled",
		KeySchedulerRunSucceeded:             "Succeeded",
		KeySchedulerScheduleDisabled:         "Disabled",
		KeySchedulerScheduleEnabled:          "Enabled",
		KeySchedulerTriggerCron:              "Cron",
		KeySchedulerTriggerManual:            "Manual",
		KeySchedulerTriggerRetry:             "Retry",
		KeySchedulerTriggerBusiness:          "Business",
		KeySchedulerCronEveryMinute:          "Every minute",
		KeySchedulerCronEvery5Minutes:        "Every 5 minutes",
		KeySchedulerCronEvery10Minutes:       "Every 10 minutes",
		KeySchedulerCronEvery15Minutes:       "Every 15 minutes",
		KeySchedulerCronEvery30Minutes:       "Every 30 minutes",
		KeySchedulerCronHourly:               "Hourly",
		KeySchedulerCronDaily:                "Daily at midnight",
		KeySchedulerCronDaily0330:            "Daily at 03:30",
		KeySchedulerCronWeekly:               "Monday at midnight",
		KeySchedulerCronMonthly:              "First day of month",
		KeySchedulerTaskNotificationBatch:    "Notification batch",
		KeySchedulerTaskRealtimeCleanup:      "Realtime event history cleanup",
		KeySchedulerTaskNotificationCleanup:  "Notification history cleanup",
		KeySchedulerTaskHistoryCleanup:       "Scheduler history cleanup",
		KeyCachePublishReady:                 "Published",
		KeyCachePublishPending:               "Pending",
		KeyCachePublishRetrying:              "Retrying",
		KeyCacheNamespaceSystem:              "System settings",
		KeyCacheNamespaceMail:                "Mail service",
		KeyCacheNamespaceSMS:                 "SMS service",
		KeyCacheNamespaceCOS:                 "Object storage configuration",
		KeyCacheScopeGlobal:                  "Global",
		KeyCacheScopeCOS:                     "Object storage configuration {{scope}}",
		KeyCacheScopeOther:                   "Other scope ({{scope}})",
		KeyCacheStatusReady:                  "Ready",
		KeyCacheStatusPending:                "Pending",
		KeyCacheStatusRetrying:               "Retrying",
		KeyCacheStatusInvalidating:           "Invalidating",
		KeyCacheStatusMissing:                "Waiting for cache rebuild",
		KeyCacheStatusMissingHint:            "Redis state is empty; configuration access rebuilds it using the existing protocol.",
		KeyCacheStatusCorrupt:                "Corrupt",
		KeyCacheStatusUnavailable:            "Redis unavailable",
		KeySMSOptionStatusPending:            "Pending",
		KeySMSOptionStatusSent:               "Sent",
		KeySMSOptionStatusFailed:             "Failed",
		KeySMSOptionScopePhone:               "Phone number",
		KeySMSOptionScopePrefix:              "Phone prefix",
		KeySMSOptionActionAllow:              "Allow",
		KeySMSOptionActionDeny:               "Deny",
		KeySMSOptionSceneLogin:               "Login verification code",
		KeySMSOptionSceneForget:              "Password recovery",
		KeySMSOptionSceneBindPhone:           "Bind or change phone",
		KeySMSOptionSceneChangePassword:      "Change password",
		KeySMSOptionRateLimitPolicy:          "Business phone sending · {{windowSeconds}}s window",
		KeySMSOptionRateLimitMode:            "Business sending",
		KeySMSOptionRateLimitDimension:       "Authentication platform + phone number",
		KeyLoginTypeEmail:                    "Email verification code",
		KeyLoginTypePhone:                    "SMS verification code",
		KeyLoginTypePassword:                 "Password",
		KeyAuthPlatformMaxSessionsUnlimited:  "Unlimited",
		KeyAuthPlatformMaxSessionsSingle:     "Single session",
		KeyAuthPlatformMaxSessionsMany:       "Up to {{count}}",
		KeyAuthPlatformDeleteReasonBuiltin:   "Built-in authentication platforms cannot be deleted",
		KeyMenuVisibilityHidden:              "Hidden",
		KeyMenuVisibilityVisible:             "Visible",
		KeyMenuStatusDisabled:                "Disabled",
		KeyMenuStatusEnabled:                 "Enabled",
		KeyMenuActionEnable:                  "Enable",
		KeyMenuActionDisable:                 "Disable",
		KeyMenuProtectedReason:               "Protected menus cannot be deleted or disabled; display information and child nodes remain editable",
		KeyMenuParentDisabledReason:          "Enable parent menus first",
	},
}

var interpolationPattern = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_]*)\}\}`)

type localeContextKey struct{}

func ParseLocale(value string) (Locale, error) {
	locale := Locale(value)
	if locale != ZhCN && locale != EnUS {
		return "", fmt.Errorf("unsupported locale %q", value)
	}
	return locale, nil
}

func WithLocale(ctx context.Context, locale Locale) context.Context {
	return context.WithValue(ctx, localeContextKey{}, locale)
}

func LocaleFromContext(ctx context.Context) Locale {
	locale, ok := ctx.Value(localeContextKey{}).(Locale)
	if !ok {
		return ZhCN
	}
	return locale
}

func ValidateCatalogs() error {
	reference, ok := catalogs[ZhCN]
	if !ok {
		return fmt.Errorf("catalog %q is missing", ZhCN)
	}
	for _, locale := range []Locale{ZhCN, EnUS} {
		catalog, exists := catalogs[locale]
		if !exists {
			return fmt.Errorf("catalog %q is missing", locale)
		}
		if err := validateCatalog(locale, reference, catalog); err != nil {
			return err
		}
	}
	return nil
}

func Translate(locale Locale, key MessageKey, params map[string]string) (string, error) {
	catalog, ok := catalogs[locale]
	if !ok {
		return "", fmt.Errorf("unsupported locale %q", locale)
	}
	template, ok := catalog[key]
	if !ok {
		return "", fmt.Errorf("message key %q is not defined for locale %q", key, locale)
	}

	expected := interpolationParameters(template)
	if err := validateParameters(key, expected, params); err != nil {
		return "", err
	}
	message := template
	for name := range expected {
		message = strings.ReplaceAll(message, "{{"+name+"}}", params[name])
	}
	return message, nil
}

func validateCatalog(locale Locale, reference, candidate map[MessageKey]string) error {
	if len(candidate) != len(reference) {
		return fmt.Errorf("catalog %q has %d keys, want %d", locale, len(candidate), len(reference))
	}
	for key, referenceMessage := range reference {
		candidateMessage, ok := candidate[key]
		if !ok {
			return fmt.Errorf("catalog %q is missing message key %q", locale, key)
		}
		if !sameNames(interpolationParameters(referenceMessage), interpolationParameters(candidateMessage)) {
			return fmt.Errorf("catalog %q message key %q has different interpolation parameters", locale, key)
		}
	}
	return nil
}

func interpolationParameters(message string) map[string]struct{} {
	parameters := make(map[string]struct{})
	for _, match := range interpolationPattern.FindAllStringSubmatch(message, -1) {
		parameters[match[1]] = struct{}{}
	}
	return parameters
}

func validateParameters(key MessageKey, expected map[string]struct{}, params map[string]string) error {
	if len(params) != len(expected) {
		return fmt.Errorf("message key %q parameters = %v, want %v", key, sortedMapKeys(params), sortedSetKeys(expected))
	}
	for name := range expected {
		if _, ok := params[name]; !ok {
			return fmt.Errorf("message key %q is missing parameter %q", key, name)
		}
	}
	return nil
}

func sameNames(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for name := range left {
		if _, ok := right[name]; !ok {
			return false
		}
	}
	return true
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
