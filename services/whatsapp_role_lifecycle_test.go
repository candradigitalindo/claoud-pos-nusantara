package services

import (
	"encoding/json"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestWARoleSettingsRenameKeepsPreferences(t *testing.T) {
	current := map[string]string{
		"wa_role_notifications": `{"lama":{"enabled":false,"outlet_mode":"selected","outlet_ids":["A"],"include_global":false,"events":{"sales.daily":"off"}},"lain":{"enabled":true}}`,
		"wa_event_roles":        `{"sales.daily":["lama","lain"],"device.offline":["lain"]}`,
		"wa_events":             `{"sales.daily":true}`,
	}
	updated, err := remapWARoleSettings(current, "lama", "baru")
	if err != nil {
		t.Fatal(err)
	}
	var policies map[string]json.RawMessage
	if err := json.Unmarshal([]byte(updated["wa_role_notifications"]), &policies); err != nil {
		t.Fatal(err)
	}
	if _, exists := policies["lama"]; exists || policies["baru"] == nil || policies["lain"] == nil {
		t.Fatalf("role preferences lost on rename: %s", updated["wa_role_notifications"])
	}
	var events map[string][]string
	if err := json.Unmarshal([]byte(updated["wa_event_roles"]), &events); err != nil {
		t.Fatal(err)
	}
	if events["sales.daily"][0] != "baru" || events["device.offline"][0] != "lain" || updated["wa_events"] != "" {
		t.Fatalf("rename changed unrelated routing: %+v", updated)
	}
}

func TestWARoleSettingsDeleteDoesNotWidenRecipients(t *testing.T) {
	updated, err := remapWARoleSettings(map[string]string{
		"wa_role_notifications": `{"hapus":{"enabled":false},"tetap":{"enabled":true}}`,
		"wa_event_roles":        `{"sales.daily":["hapus"],"device.offline":["hapus","tetap"]}`,
		"wa_events":             `{"reservation.created":false,"sales.daily":true}`,
	}, "hapus", "")
	if err != nil {
		t.Fatal(err)
	}
	var flags map[string]bool
	if err := json.Unmarshal([]byte(updated["wa_events"]), &flags); err != nil {
		t.Fatal(err)
	}
	if flags["sales.daily"] || flags["reservation.created"] {
		t.Fatalf("removing last explicit recipient opened automatic routing: %+v", flags)
	}
	var policies map[string]json.RawMessage
	json.Unmarshal([]byte(updated["wa_role_notifications"]), &policies)
	if _, exists := policies["hapus"]; exists || policies["tetap"] == nil {
		t.Fatal("deleted role remains or another role's policy disappeared")
	}
	var events map[string][]string
	json.Unmarshal([]byte(updated["wa_event_roles"]), &events)
	if _, exists := events["sales.daily"]; exists || len(events["device.offline"]) != 1 || events["device.offline"][0] != "tetap" {
		t.Fatalf("incorrect remaining recipients: %+v", events)
	}
}

func TestWARoleSettingsRejectCorruptConfiguration(t *testing.T) {
	for _, key := range []string{"wa_role_notifications", "wa_event_roles", "wa_events"} {
		current := map[string]string{"wa_event_roles": `{"sales.daily":["lama"]}`, key: `invalid-json`}
		if _, err := remapWARoleSettings(current, "lama", ""); err == nil {
			t.Errorf("corrupt %s silently discarded", key)
		}
	}
}

func TestWARoleNotificationLifecycleFollowsRoleRenameAndDelete(t *testing.T) {
	requireDB(t)
	oldName, newName, otherName := "notif_old_"+NewULID()[18:], "notif_new_"+NewULID()[18:], "notif_other_"+NewULID()[18:]
	if _, err := database.DB.Exec(`INSERT INTO roles (name,description,is_system) VALUES ($1,'Notification lifecycle test',false)`, oldName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM roles WHERE name IN ($1,$2)`, oldName, newName)
	})
	keys := []string{"wa_role_notifications", "wa_event_roles", "wa_events"}
	before := map[string]*string{}
	for _, key := range keys {
		var value string
		if err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key=$1`, key).Scan(&value); err == nil {
			v := value
			before[key] = &v
		} else {
			before[key] = nil
		}
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if before[key] == nil {
				execScratch(`DELETE FROM app_settings WHERE key=$1`, key)
			} else {
				execScratch(`UPDATE app_settings SET value=$2 WHERE key=$1`, key, *before[key])
			}
		}
	})
	policies, _ := json.Marshal(map[string]map[string]bool{oldName: {"enabled": false}, otherName: {"enabled": true}})
	events, _ := json.Marshal(map[string][]string{"sales.daily": {oldName}, "device.offline": {oldName, "superadmin"}})
	if err := UpdateSettings(map[string]string{"wa_role_notifications": string(policies), "wa_event_roles": string(events), "wa_events": `{"sales.daily":true}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateRole(oldName, models.UpdateRoleRequest{Name: newName, Description: "Renamed notification role", RedirectTo: "/"}); err != nil {
		t.Fatal(err)
	}
	var stored map[string]json.RawMessage
	value, err := GetSetting("wa_role_notifications")
	if err != nil || json.Unmarshal([]byte(value), &stored) != nil || stored[newName] == nil || stored[oldName] != nil {
		t.Fatalf("rename lost notification preference: %q / %v", value, err)
	}
	if err := DeleteRole(newName); err != nil {
		t.Fatal(err)
	}
	value, err = GetSetting("wa_role_notifications")
	stored = nil
	json.Unmarshal([]byte(value), &stored)
	if err != nil || stored[newName] != nil || stored[otherName] == nil {
		t.Fatalf("delete lost unrelated preferences: %q / %v", value, err)
	}
	var flags map[string]bool
	value, err = GetSetting("wa_events")
	if err != nil || json.Unmarshal([]byte(value), &flags) != nil || flags["sales.daily"] {
		t.Fatal("deleting last explicit receiver widened sales notifications")
	}
}
