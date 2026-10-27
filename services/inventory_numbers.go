package services

import "math"

// Stock quantities and recipe costs use four decimal places across inventory,
// F&B and PPIC. Keep this shared precision independent of any planning module.
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
