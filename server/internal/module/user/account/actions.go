package account

import (
	"context"

	"admin/server/internal/module/permission/role"
	"admin/server/internal/shared/option"
	"admin/server/internal/shared/yesno"
)

type Actions struct {
	Update    bool `json:"update"`
	Status    bool `json:"status"`
	Delete    bool `json:"delete"`
	Authorize bool `json:"authorize"`
}
type ActionLabels struct {
	Update    string `json:"update"`
	Status    string `json:"status"`
	Delete    string `json:"delete"`
	Authorize string `json:"authorize"`
}
type AssignmentRole struct {
	RoleSummary
	Selectable bool
	Locked     bool
}
type actorFacts struct {
	ActorActive bool
	ActorSuper  bool
	// Two is sufficient to distinguish the last effective administrator.
	EffectiveSuperAdmins int64
}

func accountActions(ctx context.Context, actorID int64, facts actorFacts, target ListItem) (Actions, ActionLabels) {
	label := func(zh, en string) string { return option.New(ctx, "", zh, en).Label }
	actions := Actions{Update: true, Status: true, Delete: true, Authorize: true}
	labels := ActionLabels{Update: label("编辑", "Edit"), Status: label("启用", "Enable"), Delete: label("删除用户", "Delete user"), Authorize: label("分配角色", "Assign roles")}
	if target.IsEnabled == yesno.Yes {
		labels.Status = label("禁用", "Disable")
	}
	targetSuper := false
	for _, item := range target.Roles {
		if item.Code == role.CodeSuperAdmin {
			targetSuper = true
			break
		}
	}
	if targetSuper && !facts.ActorSuper {
		reason := label("只有超级管理员可以管理超级管理员账号", "Only a super administrator can manage a super administrator account")
		return Actions{}, ActionLabels{reason, reason, reason, reason}
	}
	if actorID == target.ID {
		actions.Status, actions.Delete, actions.Authorize = false, false, false
		labels.Status = label("不能停用自己的账号", "You cannot disable your own account")
		labels.Delete = label("不能删除自己的账号", "You cannot delete your own account")
		labels.Authorize = label("不能修改自己的角色", "You cannot change your own roles")
	} else if targetSuper && target.IsEnabled == yesno.Yes && facts.EffectiveSuperAdmins < 2 {
		actions.Status, actions.Delete = false, false
		reason := label("必须保留至少一个有效超级管理员", "At least one effective super administrator must remain")
		labels.Status, labels.Delete = reason, reason
	}
	return actions, labels
}

func assignmentOptions(actorID int64, facts actorFacts, target Summary, options []RoleSummary, selectedIDs []int64) []AssignmentRole {
	selected := make(map[int64]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = true
	}
	targetSuper := false
	for _, item := range options {
		if item.Code == role.CodeSuperAdmin && selected[item.ID] {
			targetSuper = true
		}
	}
	canAuthorize := actorID != target.ID && (!targetSuper || facts.ActorSuper)
	result := make([]AssignmentRole, 0, len(options))
	for _, item := range options {
		selectable := canAuthorize && (item.Code != role.CodeSuperAdmin || facts.ActorSuper)
		if item.Code == role.CodeSuperAdmin && selected[item.ID] && target.IsEnabled == yesno.Yes && facts.EffectiveSuperAdmins < 2 {
			selectable = false
		}
		result = append(result, AssignmentRole{RoleSummary: item, Selectable: selectable, Locked: !selectable && selected[item.ID]})
	}
	return result
}
