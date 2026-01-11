package registry

import (
	"strings"

	"github.com/klauspost/cpuid/v2"
)

// DetectCPUFeatures returns the CPU capabilities of the current system.
func DetectCPUFeatures() CPUFeatures {
	return CPUFeatures{
		AVX512: cpuid.CPU.Supports(cpuid.AVX512F),
		AVX2:   cpuid.CPU.Supports(cpuid.AVX2),
		BMI2:   cpuid.CPU.Supports(cpuid.BMI2),
		POPCNT: cpuid.CPU.Supports(cpuid.POPCNT),
		SSE42:  cpuid.CPU.Supports(cpuid.SSE42),
	}
}

// FeatureString returns a human-readable string of supported features.
func (f CPUFeatures) FeatureString() string {
	var features []string
	if f.AVX512 {
		features = append(features, "AVX512")
	}
	if f.AVX2 {
		features = append(features, "AVX2")
	}
	if f.BMI2 {
		features = append(features, "BMI2")
	}
	if f.POPCNT {
		features = append(features, "POPCNT")
	}
	if f.SSE42 {
		features = append(features, "SSE42")
	}
	if len(features) == 0 {
		return "none"
	}
	return strings.Join(features, ", ")
}

// BestFeature returns the highest-priority feature this CPU supports.
func (f CPUFeatures) BestFeature() string {
	switch {
	case f.AVX512:
		return "avx512"
	case f.BMI2:
		return "bmi2"
	case f.AVX2:
		return "avx2"
	case f.POPCNT:
		return "popcnt"
	case f.SSE42:
		return "sse42"
	default:
		return ""
	}
}
