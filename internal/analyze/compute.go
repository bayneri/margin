package analyze

import "math"

func ComputeBudget(goal, compliance float64) (allowedBad, bad, consumed float64, notes []string) {
	allowedBad = 1 - goal
	complianceClamped, clampNote := clamp01(compliance)
	if clampNote != "" {
		notes = append(notes, clampNote)
	}
	bad = 1 - complianceClamped
	if allowedBad <= 0 {
		return allowedBad, bad, 0, notes
	}
	consumed = (bad / allowedBad) * 100
	return allowedBad, bad, consumed, notes
}

func clamp01(value float64) (float64, string) {
	if value < 0 {
		return 0, "compliance clamped to 0"
	}
	if value > 1 {
		return 1, "compliance clamped to 1"
	}
	return value, ""
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
