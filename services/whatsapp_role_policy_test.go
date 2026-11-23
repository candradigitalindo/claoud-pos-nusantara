package services

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestWARolePolicyScopesAndPrecedence(t *testing.T) {
	all := waRoleInfo{scopeType: "all", perms: []string{"reservations.view"}}
	limited := waRoleInfo{scopeType: "specific", outletIDs: []string{"A"}, perms: all.perms}
	s := models.WASettings{Events: map[string]bool{}, EventRoles: map[string][]string{"reservation.created": {"other"}}, RoleNotifications: map[string]models.WARoleNotificationPolicy{}}
	spec := waSpec("reservation.created")
	p, exists := waRolePolicy("manager", limited, s)
	if exists || !p.Enabled || p.IncludeGlobal || p.OutletMode != "role" {
		t.Fatalf("limited default policy: %+v, explicit=%v", p, exists)
	}
	if waPolicyCoversOutlet(limited, p, "") || waPolicyCoversOutlet(limited, p, waCentralOutlet) ||
		!waPolicyCoversOutlet(limited, p, "A") || waPolicyCoversOutlet(limited, p, "B") || !waPolicyCoversOutlet(limited, p, waDigestOutlet) {
		t.Fatal("limited role escaped outlet/global boundary")
	}
	if waRoleCoversOutlet(waRoleInfo{}, "") || waRoleCoversOutlet(waRoleInfo{}, "A") {
		t.Fatal("unknown role scope widened access")
	}
	if waUserMatchesEvent(spec, "manager", all, s, nil) {
		t.Fatal("auto ignored legacy EventRoles selection")
	}
	p = models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role", IncludeGlobal: true, Events: map[string]string{"reservation.created": "on"}}
	s.RoleNotifications["manager"] = p
	if !waUserMatchesEvent(spec, "manager", all, s, nil) {
		t.Fatal("eligible explicit subscription did not override automatic role selection")
	}
	if waUserMatchesEvent(spec, "manager", waRoleInfo{scopeType: "all"}, s, nil) {
		t.Fatal("on enabled a role without eligible permissions")
	}
	s.Events[spec.Key] = false
	if waUserMatchesEvent(spec, "manager", all, s, nil) {
		t.Fatal("role on overrode the event master switch")
	}
	s.Events[spec.Key] = true
	p.Events[spec.Key] = "off"
	s.RoleNotifications["manager"] = p
	if waUserMatchesEvent(spec, "manager", all, s, nil) {
		t.Fatal("role off ignored")
	}
	p.Enabled, p.Events[spec.Key] = false, "on"
	s.RoleNotifications["manager"] = p
	if waUserMatchesEvent(spec, "manager", all, s, nil) {
		t.Fatal("role disabled ignored")
	}

	selected := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "selected", OutletIDs: []string{"A", "B"}}
	if got := waEffectiveRoleOutlets(limited, selected); !reflect.DeepEqual(got, []string{"A"}) {
		t.Fatalf("selected outlets widened role access: %v", got)
	}
	selected.OutletIDs = []string{"B"}
	if waPolicyCoversOutlet(limited, selected, "B") || waPolicyCoversOutlet(limited, selected, waDigestOutlet) {
		t.Fatal("empty intersection was treated as unrestricted")
	}
	unrestrictedNoGlobal := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role"}
	if waPolicyCoversOutlet(all, unrestrictedNoGlobal, "") || waPolicyCoversOutlet(all, unrestrictedNoGlobal, waDigestOutlet) || !waPolicyCoversOutlet(all, unrestrictedNoGlobal, "A") {
		t.Fatal("global off must stop unrestricted digests while retaining outlet events")
	}
}

func TestWARolePolicyValidationAndBahanMapping(t *testing.T) {
	roles := map[string]waRoleInfo{
		"limited": {scopeType: "specific", outletIDs: []string{"A"}, perms: []string{"reports.sales.view"}},
		"all":     {scopeType: "all", perms: []string{"reports.sales.view"}},
		"raw":     {scopeType: "all", perms: []string{"procurement.rawmaterials.approve"}},
		"request": {scopeType: "specific", outletIDs: []string{"A"}, perms: []string{"procurement.requests.submit"}},
	}
	outlets := map[string]string{"A": "Outlet A", "B": "Outlet B"}
	valid := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "selected", OutletIDs: []string{" A ", "A"}, Events: map[string]string{"sales.daily": "on", "reservation.created": "off"}}
	got, err := normalizeWARolePolicies(map[string]models.WARoleNotificationPolicy{"limited": valid}, roles, outlets)
	if err != nil || !reflect.DeepEqual(got["limited"].OutletIDs, []string{"A"}) {
		t.Fatalf("valid normalized policy: %+v, %v", got, err)
	}
	cases := []struct {
		name, role string
		policy     models.WARoleNotificationPolicy
	}{
		{"unknown role", "missing", valid},
		{"outside role", "limited", models.WARoleNotificationPolicy{Enabled: true, OutletMode: "selected", OutletIDs: []string{"B"}}},
		{"inactive or missing", "all", models.WARoleNotificationPolicy{Enabled: true, OutletMode: "selected", OutletIDs: []string{"C"}}},
		{"limited global", "limited", models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role", IncludeGlobal: true}},
		{"selected global", "all", models.WARoleNotificationPolicy{Enabled: true, OutletMode: "selected", OutletIDs: []string{"A"}, IncludeGlobal: true}},
		{"unknown mode", "all", models.WARoleNotificationPolicy{Enabled: true, OutletMode: "anything"}},
		{"requester subscribing peers", "request", models.WARoleNotificationPolicy{Enabled: true, Events: map[string]string{"purchase.approved": "on"}}},
		{"wrong permission", "limited", models.WARoleNotificationPolicy{Enabled: true, Events: map[string]string{"reservation.created": "on"}}},
		{"unknown event", "all", models.WARoleNotificationPolicy{Enabled: true, Events: map[string]string{"missing": "off"}}},
		{"customer event", "all", models.WARoleNotificationPolicy{Enabled: true, Events: map[string]string{"customer.reservation_created": "on"}}},
		{"unknown event selection", "all", models.WARoleNotificationPolicy{Enabled: true, Events: map[string]string{"sales.daily": "yes"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := normalizeWARolePolicies(map[string]models.WARoleNotificationPolicy{tc.role: tc.policy}, roles, outlets); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
	rawPolicy := models.WARoleNotificationPolicy{Enabled: true, Events: map[string]string{"purchase.verified": "on"}}
	if _, err := normalizeWARolePolicies(map[string]models.WARoleNotificationPolicy{"raw": rawPolicy}, roles, outlets); err != nil {
		t.Fatalf("raw-material permission not recognized: %v", err)
	}
	s := models.WASettings{RoleNotifications: map[string]models.WARoleNotificationPolicy{"raw": rawPolicy}}
	if waUserMatchesEvent(waSpec("purchase.verified"), "raw", roles["raw"], s, nil) || !waUserMatchesEvent(waSpec("purchase.verified"), "raw", roles["raw"], s, waBahanPermMapper) {
		t.Fatal("explicit raw-material subscription escaped its actual procurement type")
	}

	// Approval Jasa hanya menerima notifikasi pengajuan jasa, bukan barang atau bahan baku.
	roles["jasa"] = waRoleInfo{scopeType: "all", perms: []string{"procurement.requests.approve_services"}}
	jasaPolicy := models.WARoleNotificationPolicy{Enabled: true, Events: map[string]string{"purchase.verified": "on"}}
	if _, err := normalizeWARolePolicies(map[string]models.WARoleNotificationPolicy{"jasa": jasaPolicy}, roles, outlets); err != nil {
		t.Fatalf("service approval permission not recognized: %v", err)
	}
	sj := models.WASettings{RoleNotifications: map[string]models.WARoleNotificationPolicy{"jasa": jasaPolicy}}
	if waUserMatchesEvent(waSpec("purchase.verified"), "jasa", roles["jasa"], sj, nil) ||
		waUserMatchesEvent(waSpec("purchase.verified"), "jasa", roles["jasa"], sj, waBahanPermMapper) ||
		!waUserMatchesEvent(waSpec("purchase.verified"), "jasa", roles["jasa"], sj, waJasaPermMapper) {
		t.Fatal("service approver subscription must match only jasa procurement type")
	}
}

func TestWASettingsPartialPresence(t *testing.T) {
	current := models.WASettings{Enabled: true, GapMinSec: 4, GapMaxSec: 9, Events: map[string]bool{"sales.daily": false}, RoleNotifications: map[string]models.WARoleNotificationPolicy{"role": {Enabled: false}}}
	var patch models.WASettings
	if err := json.Unmarshal([]byte(`{"enabled":false}`), &patch); err != nil {
		t.Fatal(err)
	}
	got := waMergeSettingsPatch(patch, current)
	if got.Enabled || got.GapMinSec != 4 || got.GapMaxSec != 9 || got.Events["sales.daily"] || !reflect.DeepEqual(got.RoleNotifications, current.RoleNotifications) {
		t.Fatalf("partial input clobbered omitted fields: %+v", got)
	}
	if err := json.Unmarshal([]byte(`{"role_notifications":{}}`), &patch); err != nil {
		t.Fatal(err)
	}
	if patch.RoleNotifications == nil || len(patch.RoleNotifications) != 0 || !patch.ProvidedFields["role_notifications"] {
		t.Fatal("explicit empty policies must be distinguishable from omitted/null")
	}
}

func TestWALegacyRoleOverridesRespectProcurementType(t *testing.T) {
	goods := waRoleInfo{scopeType: "all", perms: []string{"procurement.requests.purchasing"}}
	s := models.WASettings{EventRoles: map[string][]string{"purchase.approved": {"goods"}}}
	spec := waSpec("purchase.approved")
	if !waUserMatchesEvent(spec, "goods", goods, s, nil) {
		t.Fatal("eligible goods purchasing role lost goods override")
	}
	if waUserMatchesEvent(spec, "goods", goods, s, waBahanPermMapper) {
		t.Fatal("legacy goods override leaked raw-material purchase event")
	}
	finance := waRoleInfo{scopeType: "all", perms: []string{"finance.payments.pay"}}
	s.EventRoles["purchase.payment_requested"] = []string{"finance"}
	for _, mapper := range []waPermMapper{nil, waBahanPermMapper} {
		if !waUserMatchesEvent(waSpec("purchase.payment_requested"), "finance", finance, s, mapper) {
			t.Fatal("finance role lost event whose payment permission is shared by both procurement types")
		}
	}
}

func TestWARoleEventContextsAlignWithEffectiveScope(t *testing.T) {
	info := waRoleInfo{scopeType: "all"}
	policy := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role", IncludeGlobal: false}
	outlets := map[string]string{"A": "Outlet A"}
	for _, key := range []string{"sales.daily", "ppic.alert", "asset.maintenance_due"} {
		if waRoleEventHasContext(waSpec(key), info, policy, outlets) {
			t.Errorf("unrestricted digest %s listed despite global off", key)
		}
	}
	if !waRoleEventHasContext(waSpec("reservation.created"), info, policy, outlets) || !waRoleEventHasContext(waSpec("purchase.approved"), info, policy, outlets) {
		t.Fatal("global off removed reachable outlet/document events")
	}
	global := &waEventSpec{Key: "internal.global", Audience: "internal"}
	if waRoleEventHasContext(global, info, policy, outlets) {
		t.Fatal("global-only event listed with global off")
	}
	policy.OutletMode, policy.OutletIDs = "selected", []string{"A"}
	if !waRoleEventHasContext(waSpec("sales.daily"), info, policy, outlets) {
		t.Fatal("selected outlet digest was hidden")
	}
	if waRoleEventHasContext(waSpec("sales.daily"), info, policy, map[string]string{}) {
		t.Fatal("digest listed without any active selected outlet")
	}
	policy.IncludeGlobal = true // stale/invalid data must still fail closed.
	if waPolicyCoversOutlet(info, policy, "") {
		t.Fatal("selected scope allowed central/global data")
	}
}

func TestWARolePolicyCentralWorkUnitContext(t *testing.T) {
	info := waRoleInfo{scopeType: "specific", workUnitIDs: []string{"unit-own"}}
	policy := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role"}
	if !waPolicyCoversOutlet(info, policy, waWorkUnitPrefix+"unit-own") || waPolicyCoversOutlet(info, policy, waWorkUnitPrefix+"unit-other") || waPolicyCoversOutlet(info, policy, "") || waPolicyCoversOutlet(info, policy, waDigestOutlet) {
		t.Fatal("central work-unit rights were treated as global or another unit")
	}
	units := map[string]string{"unit-own": "Unit Sendiri", "unit-other": "Unit Lain"}
	if !waRoleEventHasContext(waSpec("purchase.approved"), info, policy, map[string]string{}, units) || waRoleEventHasContext(waSpec("reservation.created"), info, policy, map[string]string{}, units) {
		t.Fatal("work-unit document reachability differs from outlet events")
	}
	policy.OutletMode, policy.OutletIDs = "selected", []string{"outlet"}
	if waPolicyCoversOutlet(info, policy, waWorkUnitPrefix+"unit-own") {
		t.Fatal("selected outlet mode included central work units")
	}
	policy.OutletMode = "role"
	if !waPolicyCoversOutlet(waRoleInfo{scopeType: "all"}, policy, waWorkUnitPrefix+"unit-own") || waPolicyCoversOutlet(waRoleInfo{scopeType: "all"}, policy, waDigestOutlet) {
		t.Fatal("all-role global-off should permit unit documents, never unrestricted digest")
	}
}

func waPolicyPreserveSettings(t *testing.T, keys ...string) {
	t.Helper()
	type snapshot struct {
		key, value string
		exists     bool
	}
	before := []snapshot{}
	for _, key := range keys {
		var value string
		err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key = $1`, key).Scan(&value)
		before = append(before, snapshot{key: key, value: value, exists: err == nil})
	}
	t.Cleanup(func() {
		for _, saved := range before {
			if saved.exists {
				execScratch(`INSERT INTO app_settings (key, value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, saved.key, saved.value)
			} else {
				execScratch(`DELETE FROM app_settings WHERE key=$1`, saved.key)
			}
		}
	})
}

func TestWARolePoliciesSettingsRoutingAndConcurrentEdits(t *testing.T) {
	requireDB(t)
	waPolicyPreserveSettings(t, "wa_enabled", "wa_events", "wa_event_roles", "wa_role_notifications", "wa_templates", "wa_gap_min_sec")
	if err := UpdateSettings(map[string]string{"wa_enabled": "true", "wa_events": "{}", "wa_event_roles": "{}", "wa_role_notifications": "{}", "wa_templates": `{"sales.daily":"KEEP"}`, "wa_gap_min_sec": "4"}); err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToLower(NewULID()[20:])
	outlets, units := []string{NewULID(), NewULID()}, []string{NewULID(), NewULID()}
	for i := range outlets {
		if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1,$2,$3,$4)`, outlets[i], fmt.Sprintf("WPN%s%d", suffix, i), fmt.Sprintf("Outlet Notifikasi %s %d", suffix, i), "wa-policy-"+outlets[i]); err != nil {
			t.Fatal(err)
		}
		if _, err := database.DB.Exec(`INSERT INTO work_units (id, name, outlet_id) VALUES ($1,$2,$3)`, units[i], "Unit Notifikasi "+units[i], outlets[i]); err != nil {
			t.Fatal(err)
		}
		i := i
		t.Cleanup(func() {
			execScratch(`DELETE FROM work_units WHERE id=$1`, units[i])
			execScratch(`DELETE FROM outlets WHERE id=$1`, outlets[i])
		})
	}
	centralUnits := []string{NewULID(), NewULID()}
	for _, id := range centralUnits {
		if _, err := database.DB.Exec(`INSERT INTO work_units (id, name, outlet_id) VALUES ($1,$2,NULL)`, id, "Unit Pusat "+id); err != nil {
			t.Fatal(err)
		}
		id := id
		t.Cleanup(func() { execScratch(`DELETE FROM work_units WHERE id=$1`, id) })
	}
	allRole, limitedRole, requesterRole := "wpa"+suffix, "wpl"+suffix, "wpr"+suffix
	admins := map[string][]string{}
	for index, role := range []string{allRole, limitedRole, requesterRole} {
		scope := "specific"
		if role == allRole {
			scope = "all"
		}
		if _, err := database.DB.Exec(`INSERT INTO roles (name, scope_type) VALUES ($1,$2)`, role, scope); err != nil {
			t.Fatal(err)
		}
		perms := []string{"reservations.view", "reports.sales.view", "procurement.requests.approve"}
		if role == requesterRole {
			perms = []string{"procurement.requests.submit"}
		}
		for _, p := range perms {
			if _, err := database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1,$2)`, role, p); err != nil {
				t.Fatal(err)
			}
		}
		if scope == "specific" {
			if _, err := database.DB.Exec(`INSERT INTO role_work_unit_scope (role, work_unit_id) VALUES ($1,$2)`, role, units[0]); err != nil {
				t.Fatal(err)
			}
			if _, err := database.DB.Exec(`INSERT INTO role_work_unit_scope (role, work_unit_id) VALUES ($1,$2)`, role, centralUnits[0]); err != nil {
				t.Fatal(err)
			}
		}
		count := 1
		if role == requesterRole {
			count = 2
		}
		for n := 0; n < count; n++ {
			id := NewULID()
			phone := fmt.Sprintf("0815%08d", (time.Now().UnixNano()+int64(index*10+n))%100000000)
			if _, err := database.DB.Exec(`INSERT INTO cloud_admins (id, username, password_hash, name, role, wa_phone) VALUES ($1,$2,'x',$3,$4,$5)`, id, "wan_"+id, "Akun Notifikasi "+id, role, phone); err != nil {
				t.Fatal(err)
			}
			admins[role] = append(admins[role], id)
		}
		role := role
		t.Cleanup(func() {
			execScratch(`DELETE FROM cloud_admins WHERE role=$1`, role)
			execScratch(`DELETE FROM role_work_unit_scope WHERE role=$1`, role)
			execScratch(`DELETE FROM role_permissions WHERE role=$1`, role)
			execScratch(`DELETE FROM roles WHERE name=$1`, role)
		})
	}
	has := func(event, outlet, id string, extras ...string) bool {
		for _, recipient := range waRecipientsForMapped(event, outlet, nil, extras...) {
			if recipient.ID == id {
				return true
			}
		}
		return false
	}
	allID, limitedID, requesterID := admins[allRole][0], admins[limitedRole][0], admins[requesterRole][0]
	if !has("reservation.created", outlets[0], limitedID) || has("reservation.created", outlets[1], limitedID) || has("reservation.created", "", limitedID) || !has("reservation.created", "", allID) {
		t.Fatal("default routing/global protection incorrect")
	}
	policies := map[string]models.WARoleNotificationPolicy{
		allRole:     {Enabled: true, OutletMode: "selected", OutletIDs: []string{outlets[0]}, Events: map[string]string{"reservation.created": "on"}},
		limitedRole: {Enabled: true, OutletMode: "role", Events: map[string]string{"reservation.created": "off"}},
	}
	var wg sync.WaitGroup
	for role, policy := range policies {
		role, policy := role, policy
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := UpdateWARoleNotification(role, &policy); err != nil {
				t.Errorf("concurrent role save: %v", err)
			}
		}()
	}
	wg.Wait()
	s := GetWASettings()
	if _, exists := s.RoleNotifications[allRole]; !exists {
		t.Fatal("all-role concurrent save lost")
	}
	if _, exists := s.RoleNotifications[limitedRole]; !exists {
		t.Fatal("limited-role concurrent save lost")
	}
	if has("reservation.created", outlets[0], limitedID) || !has("reservation.created", outlets[0], allID) || has("reservation.created", outlets[1], allID) || has("reservation.created", "", allID) {
		t.Fatal("explicit role selection or selected outlet narrowing ignored")
	}
	personal := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role", Events: map[string]string{"purchase.approved": "off"}}
	if err := UpdateWARoleNotification(requesterRole, &personal); err != nil {
		t.Fatal(err)
	}
	extra := waExtraIDPrefix + requesterID
	if has("purchase.approved", outlets[0], requesterID, extra) {
		t.Fatal("off did not suppress named requester")
	}
	if err := UpdateWARoleNotification(requesterRole, nil); err != nil {
		t.Fatal(err)
	}
	if !has("purchase.approved", outlets[0], requesterID, extra) || has("purchase.approved", outlets[1], requesterID, extra) || has("purchase.approved", "", requesterID, extra) || has("purchase.approved", outlets[0], admins[requesterRole][1], extra) {
		t.Fatal("requester notification escaped document identity or outlet/global scope")
	}
	if !has("purchase.approved", waWorkUnitPrefix+centralUnits[0], requesterID, extra) || has("purchase.approved", waWorkUnitPrefix+centralUnits[1], requesterID, extra) {
		t.Fatal("named requester did not retain own work-unit rights or escaped to another unit")
	}
	if !has("purchase.verified", waWorkUnitPrefix+centralUnits[0], limitedID) || has("purchase.verified", waWorkUnitPrefix+centralUnits[1], limitedID) {
		t.Fatal("automatic stage recipient escaped its central work-unit scope")
	}
	personal.Enabled, personal.Events = false, map[string]string{}
	if err := UpdateWARoleNotification(requesterRole, &personal); err != nil {
		t.Fatal(err)
	}
	if has("purchase.approved", outlets[0], requesterID, extra) {
		t.Fatal("disabled role received named requester notification")
	}
	beforePolicies := GetWASettings().RoleNotifications
	var patch models.WASettings
	json.Unmarshal([]byte(`{"gap_min_sec":6}`), &patch)
	if err := UpdateWASettings(patch); err != nil {
		t.Fatal(err)
	}
	if got := GetWASettings(); !reflect.DeepEqual(got.RoleNotifications, beforePolicies) || got.GapMinSec != 6 || got.Templates["sales.daily"] != "KEEP" || len(got.RoleNotifications) != 3 {
		t.Fatal("partial settings update clobbered policies/templates")
	}
	json.Unmarshal([]byte(`{"role_notifications":null}`), &patch)
	if err := UpdateWASettings(patch); err != nil {
		t.Fatal(err)
	}
	if len(GetWASettings().RoleNotifications) != 3 {
		t.Fatal("null policies reset existing roles")
	}
	json.Unmarshal([]byte(`{"gap_min_sec":7,"role_notifications":{"role-does-not-exist":{"enabled":true,"outlet_mode":"role"}}}`), &patch)
	if err := UpdateWASettings(patch); err == nil || GetWASettings().GapMinSec != 6 || len(GetWASettings().RoleNotifications) != 3 {
		t.Fatal("invalid role policies caused a partial settings write")
	}
	for _, option := range WARoleOptions() {
		if option.Name == limitedRole && (!option.HasPolicy || len(option.OutletIDs) != 1 || option.OutletIDs[0] != outlets[0] || len(option.EligibleEvents) == 0 || len(option.CentralWorkUnitIDs) != 1 || option.CentralWorkUnitIDs[0] != centralUnits[0]) {
			t.Fatalf("role metadata: %+v", option)
		}
		if option.Name == requesterRole && (!option.RequesterOnly || len(option.PersonalEvents) == 0 || len(option.EligibleEvents) != 0) {
			t.Fatalf("requester metadata: %+v", option)
		}
	}
	for _, user := range WARecipientUsers() {
		if user.ID == allID && (user.ScopeType != "all" || user.NotificationScopeType != "specific" || len(user.NotificationOutletNames) != 1 || user.IncludeGlobal || !user.RoleNotificationsEnabled) {
			t.Fatalf("selected notification scope differs from displayed user scope: %+v", user)
		}
		if user.ID == allID && len(user.NotificationWorkUnitNames) != 0 {
			t.Fatalf("selected outlets exposed central work units: %+v", user)
		}
		if user.ID == limitedID && len(user.NotificationWorkUnitNames) != 1 {
			t.Fatalf("own central work unit missing from displayed notification scope: %+v", user)
		}
		if user.ID == requesterID && (user.RoleNotificationsEnabled || user.NotificationScopeType != "none" || len(user.Events) != 0) {
			t.Fatalf("disabled role still displayed as receiving events: %+v", user)
		}
	}
	unrestrictedNoGlobal := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role", Events: map[string]string{"reservation.created": "on"}}
	if err := UpdateWARoleNotification(allRole, &unrestrictedNoGlobal); err != nil {
		t.Fatal(err)
	}
	for _, user := range WARecipientUsers() {
		if user.ID != allID {
			continue
		}
		if user.NotificationScopeType != "all" || user.IncludeGlobal {
			t.Fatalf("all outlets/global-off notification scope: %+v", user)
		}
		for _, event := range user.Events {
			if event == "sales.daily" {
				t.Fatal("user list advertises sales digest while global reports are disabled")
			}
		}
	}
	json.Unmarshal([]byte(`{"events":{"reservation.created":false}}`), &patch)
	if err := UpdateWASettings(patch); err != nil {
		t.Fatal(err)
	}
	if has("reservation.created", outlets[0], allID) {
		t.Fatal("master event off did not suppress explicit role on")
	}
	json.Unmarshal([]byte(`{"role_notifications":{}}`), &patch)
	if err := UpdateWASettings(patch); err != nil {
		t.Fatal(err)
	}
	if len(GetWASettings().RoleNotifications) != 0 || GetWASettings().Events["reservation.created"] {
		t.Fatal("explicit empty policies did not reset or clobbered master event")
	}
}
