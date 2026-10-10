package menu

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/yesno"
	"context"
	"encoding/json"
	"testing"
)

func TestManagedMenuOwnsPresentationAndActions(t *testing.T) {
	body := newMenuCatalogResponse(context.Background(), Catalog{MenuTree: []ManagedMenu{{ID: 1, MenuType: TypePage, IsProtected: true, IsEnabled: yesno.Yes}}})
	raw, _ := json.Marshal(body)
	var decoded struct {
		AllowedRootTypes []Type `json:"allowedRootTypes"`
		MenuTree         []struct {
			Presentation struct{ TypeLabel, TypeTone, ProtectionReason string } `json:"presentation"`
			Actions      struct {
				Update, Status, Delete, AddChild bool
				AllowedChildTypes                []Type `json:"allowedChildTypes"`
			} `json:"actions"`
		} `json:"menuTree"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	row := decoded.MenuTree[0]
	if row.Presentation.TypeLabel != "页面" || row.Presentation.TypeTone != "success" || row.Presentation.ProtectionReason == "" {
		t.Fatalf("missing server presentation: %s", raw)
	}
	if !row.Actions.Update || !row.Actions.AddChild || row.Actions.Status || row.Actions.Delete || len(row.Actions.AllowedChildTypes) != 1 || row.Actions.AllowedChildTypes[0] != TypeAction {
		t.Fatalf("protected page capabilities: %s", raw)
	}
	if len(decoded.AllowedRootTypes) != 2 {
		t.Fatalf("root capabilities: %s", raw)
	}
}

func TestMenuCapabilitiesPreserveProtectedEditingAndParentRules(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.EnUS)
	body := newMenuCatalogResponse(ctx, Catalog{MenuTree: []ManagedMenu{{ID: 1, MenuType: TypeDirectory, IsEnabled: yesno.No, Children: []ManagedMenu{{ID: 2, MenuType: TypePage, IsEnabled: yesno.No, IsProtected: true}}}}})
	root, child := body.MenuTree[0], body.MenuTree[0].Children[0]
	if !root.Actions.AddChild || len(root.Actions.AllowedChildTypes) != 2 || !root.Actions.Status {
		t.Fatalf("directory capabilities: %#v", root)
	}
	if !child.Actions.Update || !child.Actions.AddChild || child.Actions.Status || child.Actions.Delete || child.Presentation.StatusReason != "Enable parent menus first" {
		t.Fatalf("protected child: %#v", child)
	}
	enabledParent := newManagedMenuResponse(ctx, ManagedMenu{MenuType: TypePage, IsProtected: true, IsEnabled: yesno.No}, false)
	if !enabledParent.Actions.Status {
		t.Fatal("protected disabled page can still be enabled")
	}
	action := newManagedMenuResponse(ctx, ManagedMenu{MenuType: TypeAction}, false)
	if action.Actions.AddChild || len(action.Actions.AllowedChildTypes) != 0 || action.Presentation.TypeLabel != "Action permission" {
		t.Fatalf("action: %#v", action)
	}
	unknown := newMenuPresentation(ctx, ManagedMenu{MenuType: "future"})
	if unknown.TypeLabel != "future" || unknown.TypeTone != "info" {
		t.Fatalf("unknown type guessed: %#v", unknown)
	}
}
