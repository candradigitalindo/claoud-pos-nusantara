package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// A digest is rendered separately after routing using each recipient's effective
// OutletIDs. An empty outlet ID instead means a genuinely global payload.
const waDigestOutlet = "__per_recipient__"

// A central work-unit document is scoped to that unit, not to the organization.
const waWorkUnitPrefix = "__unit__:"

func waIsDigestEvent(key string) bool {
	return key == "sales.daily" || key == "ppic.alert" || key == "asset.maintenance_due"
}

func waRoleCoversWorkUnit(info waRoleInfo, unitID string) bool {
	unitID = strings.TrimSpace(unitID)
	if unitID == "" {
		return false
	}
	if info.scopeType == "all" {
		return true
	}
	if info.scopeType == "specific" {
		for _, id := range info.workUnitIDs {
			if id == unitID {
				return true
			}
		}
	}
	return false
}

func waRolePolicy(role string, info waRoleInfo, s models.WASettings) (models.WARoleNotificationPolicy, bool) {
	p, exists := s.RoleNotifications[role]
	if !exists {
		p = models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role", IncludeGlobal: info.scopeType == "all"}
	}
	if p.OutletMode == "" {
		p.OutletMode = "role"
	}
	if p.OutletIDs == nil {
		p.OutletIDs = []string{}
	}
	if p.Events == nil {
		p.Events = map[string]string{}
	}
	if info.scopeType != "all" || p.OutletMode != "role" {
		p.IncludeGlobal = false
	}
	return p, exists
}

func waRoleEventEligible(spec *waEventSpec, role string, info waRoleInfo, mapPerm waPermMapper) bool {
	if spec == nil || spec.Audience != "internal" {
		return false
	}
	if role == "superadmin" {
		return true
	}
	if strings.HasPrefix(spec.Key, "purchase.") && WARequesterOnlyRole(role, info) {
		return false
	}
	for _, required := range spec.Permissions {
		if mapPerm != nil {
			required = mapPerm(required)
		}
		if waRoleHasPermission(info.perms, required) {
			return true
		}
	}
	return false
}

func waRoleEventEligibleAny(spec *waEventSpec, role string, info waRoleInfo) bool {
	return waRoleEventEligible(spec, role, info, nil) ||
		waRoleEventEligible(spec, role, info, waJasaPermMapper) ||
		waRoleEventEligible(spec, role, info, waBahanPermMapper)
}

// nil means all outlets. A nonnil empty/none scope must never be interpreted as
// unrestricted by digest renderers, including after a role's scope changes.
func waEffectiveRoleOutlets(info waRoleInfo, p models.WARoleNotificationPolicy) []string {
	if p.OutletMode == "role" && info.scopeType == "all" {
		return nil
	}
	allowed := map[string]bool{}
	if p.OutletMode == "role" {
		for _, id := range info.outletIDs {
			allowed[strings.TrimSpace(id)] = true
		}
	} else if p.OutletMode == "selected" {
		for _, id := range p.OutletIDs {
			id = strings.TrimSpace(id)
			if id != "" && (info.scopeType == "all" || waRoleCoversOutlet(info, id)) {
				allowed[id] = true
			}
		}
	}
	delete(allowed, "")
	ids := make([]string, 0, len(allowed))
	for id := range allowed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return []string{"__none__"}
	}
	return ids
}

func waPolicyCoversOutlet(info waRoleInfo, p models.WARoleNotificationPolicy, outletID string) bool {
	if !p.Enabled {
		return false
	}
	if strings.HasPrefix(outletID, waWorkUnitPrefix) {
		return p.OutletMode == "role" && waRoleCoversWorkUnit(info, strings.TrimPrefix(outletID, waWorkUnitPrefix))
	}
	if outletID == "" || outletID == waCentralOutlet {
		return info.scopeType == "all" && p.OutletMode == "role" && p.IncludeGlobal
	}
	ids := waEffectiveRoleOutlets(info, p)
	if outletID == waDigestOutlet {
		if len(ids) == 0 {
			return p.IncludeGlobal
		}
		return len(ids) != 1 || ids[0] != "__none__"
	}
	if len(ids) == 0 {
		return true
	}
	for _, id := range ids {
		if id == outletID {
			return true
		}
	}
	return false
}

func waRoleEventHasContext(spec *waEventSpec, info waRoleInfo, policy models.WARoleNotificationPolicy, activeOutlets map[string]string, centralUnits ...map[string]string) bool {
	if spec == nil || !policy.Enabled {
		return false
	}
	if waIsDigestEvent(spec.Key) {
		if !waPolicyCoversOutlet(info, policy, waDigestOutlet) {
			return false
		}
		if len(waEffectiveRoleOutlets(info, policy)) == 0 {
			return true
		}
		for id := range activeOutlets {
			if waPolicyCoversOutlet(info, policy, id) {
				return true
			}
		}
		return false
	}
	kind, _ := waEventScope(spec.Key)
	if kind == "global" {
		return waPolicyCoversOutlet(info, policy, "")
	}
	for id := range activeOutlets {
		if waPolicyCoversOutlet(info, policy, id) {
			return true
		}
	}
	if strings.HasPrefix(spec.Key, "purchase.") {
		if waPolicyCoversOutlet(info, policy, waCentralOutlet) {
			return true
		}
		for _, units := range centralUnits {
			for id := range units {
				if waPolicyCoversOutlet(info, policy, waWorkUnitPrefix+id) {
					return true
				}
			}
		}
	}
	return false
}

func waCentralWorkUnitNames() (map[string]string, error) {
	rows, err := database.DB.Query(`SELECT id, name FROM work_units WHERE outlet_id IS NULL ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	units := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		units[strings.TrimSpace(id)] = name
	}
	return units, rows.Err()
}

func waActiveOutletNames() (map[string]string, error) {
	rows, err := database.DB.Query(`SELECT id, name FROM outlets WHERE is_active = true ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(id)] = name
	}
	return out, rows.Err()
}

func normalizeWARolePolicies(input map[string]models.WARoleNotificationPolicy, roles map[string]waRoleInfo, outlets map[string]string) (map[string]models.WARoleNotificationPolicy, error) {
	out := map[string]models.WARoleNotificationPolicy{}
	for rawRole, p := range input {
		role := strings.TrimSpace(rawRole)
		info, exists := roles[role]
		if role == "" || !exists {
			return nil, Invalid("role notifikasi %q tidak ditemukan", role)
		}
		if _, duplicate := out[role]; duplicate {
			return nil, Invalid("role notifikasi %q dikirim lebih dari sekali", role)
		}
		p.OutletMode = strings.TrimSpace(p.OutletMode)
		if p.OutletMode == "" {
			p.OutletMode = "role"
		}
		if p.OutletMode != "role" && p.OutletMode != "selected" {
			return nil, Invalid("cakupan notifikasi role %q harus role atau selected", role)
		}
		if p.IncludeGlobal && (info.scopeType != "all" || p.OutletMode != "role") {
			return nil, Invalid("role %q hanya boleh menerima informasi global pada cakupan seluruh outlet role; pilihan outlet tertentu tidak boleh menerima global", role)
		}
		ids := map[string]bool{}
		for _, id := range p.OutletIDs {
			id = strings.TrimSpace(id)
			if _, active := outlets[id]; !active || (info.scopeType != "all" && !waRoleCoversOutlet(info, id)) {
				return nil, Invalid("outlet notifikasi %q tidak aktif atau di luar cakupan role %q", id, role)
			}
			ids[id] = true
		}
		p.OutletIDs = []string{}
		if p.OutletMode == "selected" {
			for id := range ids {
				p.OutletIDs = append(p.OutletIDs, id)
			}
			sort.Strings(p.OutletIDs)
		}
		events := map[string]string{}
		for key, mode := range p.Events {
			spec := waSpec(key)
			if spec == nil || spec.Audience != "internal" {
				return nil, Invalid("event notifikasi internal %q tidak ditemukan", key)
			}
			mode = strings.TrimSpace(mode)
			if mode != "auto" && mode != "on" && mode != "off" {
				return nil, Invalid("pilihan event %s pada role %q harus auto, on, atau off", key, role)
			}
			if mode == "on" && !waRoleEventEligibleAny(spec, role, info) {
				return nil, Invalid("role %q tidak punya izin penerima event %s; gunakan otomatis untuk pemberitahuan pengajuan sendiri", role, key)
			}
			if mode != "auto" {
				events[key] = mode
			}
		}
		p.Events = events
		out[role] = p
	}
	return out, nil
}

func validateWARolePolicies(input map[string]models.WARoleNotificationPolicy) (map[string]models.WARoleNotificationPolicy, error) {
	outlets, err := waActiveOutletNames()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca outlet notifikasi: %w", err)
	}
	return normalizeWARolePolicies(input, waLoadRoles(), outlets)
}

// UpdateWARoleNotification changes just one role while preserving every other
// role under a row lock. nil restores this role's inherited default policy.
func UpdateWARoleNotification(role string, policy *models.WARoleNotificationPolicy) error {
	role = strings.TrimSpace(role)
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if role != "superadmin" {
		var locked string
		if err := tx.QueryRow(`SELECT name FROM roles WHERE name = $1 FOR SHARE`, role).Scan(&locked); err != nil {
			if err == sql.ErrNoRows {
				return Invalid("role notifikasi %q tidak ditemukan", role)
			}
			return err
		}
	}
	var normalized models.WARoleNotificationPolicy
	if policy != nil {
		policies, err := validateWARolePolicies(map[string]models.WARoleNotificationPolicy{role: *policy})
		if err != nil {
			return err
		}
		normalized = policies[role]
	}
	if _, err := tx.Exec(`INSERT INTO app_settings (key, value, updated_at) VALUES ('wa_role_notifications', '{}', $1) ON CONFLICT (key) DO NOTHING`, time.Now().UTC()); err != nil {
		return err
	}
	var raw string
	if err := tx.QueryRow(`SELECT value FROM app_settings WHERE key = 'wa_role_notifications' FOR UPDATE`).Scan(&raw); err != nil {
		return err
	}
	current := map[string]models.WARoleNotificationPolicy{}
	if err := json.Unmarshal([]byte(raw), &current); err != nil {
		return fmt.Errorf("pengaturan notifikasi role tidak valid: %w", err)
	}
	if current == nil {
		current = map[string]models.WARoleNotificationPolicy{}
	}
	if policy == nil {
		delete(current, role)
	} else {
		current[role] = normalized
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE app_settings SET value = $1, updated_at = $2 WHERE key = 'wa_role_notifications'`, string(encoded), time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func waMergeSettingsPatch(in, current models.WASettings) models.WASettings {
	if in.ProvidedFields == nil {
		return in
	}
	// Marshal/unmarshal aliases retain typed fields while overlaying only the
	// keys actually supplied by the caller, including explicit false/empty maps.
	type settings models.WASettings
	baseJSON, _ := json.Marshal(settings(current))
	inputJSON, _ := json.Marshal(settings(in))
	var base, incoming map[string]json.RawMessage
	json.Unmarshal(baseJSON, &base)
	json.Unmarshal(inputJSON, &incoming)
	for key := range in.ProvidedFields {
		if value, exists := incoming[key]; exists {
			base[key] = value
		}
	}
	mergedJSON, _ := json.Marshal(base)
	var merged settings
	json.Unmarshal(mergedJSON, &merged)
	merged.ProvidedFields = in.ProvidedFields
	return models.WASettings(merged)
}
