package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
)

// remapWARoleSettings keeps notification preferences attached to a renamed
// role. Removing the last explicit event recipient disables that event instead
// of silently widening it back to automatic recipients.
func remapWARoleSettings(current map[string]string, oldName, newName string) (map[string]string, error) {
	changed := map[string]string{}
	if raw := current["wa_role_notifications"]; raw != "" {
		var policies map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &policies); err != nil {
			return nil, fmt.Errorf("pengaturan notifikasi role tidak dapat dibaca: %w", err)
		}
		if policy, exists := policies[oldName]; exists {
			delete(policies, oldName)
			if newName != "" {
				policies[newName] = policy
			}
			data, err := json.Marshal(policies)
			if err != nil {
				return nil, err
			}
			changed["wa_role_notifications"] = string(data)
		}
	}
	if raw := current["wa_event_roles"]; raw != "" {
		var events map[string][]string
		if err := json.Unmarshal([]byte(raw), &events); err != nil {
			return nil, fmt.Errorf("pengaturan penerima notifikasi tidak dapat dibaca: %w", err)
		}
		updated := false
		disabled := []string{}
		for event, roles := range events {
			seen := map[string]bool{}
			out := []string{}
			found := false
			for _, role := range roles {
				if role == oldName {
					found = true
					role = newName
				}
				if role != "" && !seen[role] {
					out = append(out, role)
					seen[role] = true
				}
			}
			if !found {
				continue
			}
			updated = true
			if len(out) == 0 {
				delete(events, event)
				disabled = append(disabled, event)
			} else {
				events[event] = out
			}
		}
		if updated {
			data, err := json.Marshal(events)
			if err != nil {
				return nil, err
			}
			changed["wa_event_roles"] = string(data)
		}
		if len(disabled) > 0 {
			flags := map[string]bool{}
			if raw := current["wa_events"]; raw != "" {
				if err := json.Unmarshal([]byte(raw), &flags); err != nil {
					return nil, fmt.Errorf("status notifikasi tidak dapat dibaca: %w", err)
				}
			}
			if flags == nil {
				flags = map[string]bool{}
			}
			for _, event := range disabled {
				flags[event] = false
			}
			data, err := json.Marshal(flags)
			if err != nil {
				return nil, err
			}
			changed["wa_events"] = string(data)
		}
	}
	return changed, nil
}

func rewriteWARoleReferencesTx(tx *sql.Tx, oldName, newName string) error {
	rows, err := tx.Query(`SELECT key, value FROM app_settings
		WHERE key IN ('wa_event_roles','wa_events','wa_role_notifications') ORDER BY key FOR UPDATE`)
	if err != nil {
		return err
	}
	current := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			rows.Close()
			return err
		}
		current[key] = value
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	changed, err := remapWARoleSettings(current, oldName, newName)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(changed))
	for key := range changed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := tx.Exec(`INSERT INTO app_settings (key,value,updated_at)
			VALUES ($1,$2,now() AT TIME ZONE 'UTC')
			ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=EXCLUDED.updated_at`, key, changed[key]); err != nil {
			return err
		}
	}
	return nil
}
