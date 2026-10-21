package services

import (
	"fmt"
	"math"
	"strings"
)

// stockQuantityToBase converts the quantity in the selected unit to the one
// shared inventory unit. An omitted unit preserves legacy distribution input.
func stockQuantityToBase(qty float64, unit, baseUnit, distUnit string, distRatio float64) (float64, string, error) {
	if qty <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) {
		return 0, "", fmt.Errorf("jumlah harus lebih dari 0 dan berupa angka valid")
	}
	baseUnit, distUnit = strings.TrimSpace(baseUnit), strings.TrimSpace(distUnit)
	if distUnit == "" {
		distUnit = baseUnit
	}
	unit = strings.TrimSpace(unit)
	if unit == "" {
		unit = distUnit
	}
	if baseUnit == "" {
		return 0, "", fmt.Errorf("satuan dasar barang belum diatur")
	}
	ratio := 1.0
	switch unit {
	case baseUnit:
	case distUnit:
		if distRatio <= 0 || math.IsNaN(distRatio) || math.IsInf(distRatio, 0) {
			return 0, "", fmt.Errorf("rasio konversi %s ke %s tidak valid", distUnit, baseUnit)
		}
		ratio = distRatio
	default:
		return 0, "", fmt.Errorf("satuan %q tidak tersedia untuk barang ini", unit)
	}
	baseQty := round4(qty * ratio)
	if baseQty <= 0 || math.IsNaN(baseQty) || math.IsInf(baseQty, 0) {
		return 0, "", fmt.Errorf("hasil konversi jumlah tidak valid atau terlalu kecil")
	}
	return baseQty, unit, nil
}
