package placement

import "math"

// scoreCost scores an environment by cost. Lower cost = higher score.
func scoreCost(env *EnvironmentInfo) float64 {
	if env.Static.CostPerHour <= 0 {
		return 0.5 // neutral if no cost data
	}
	// Inverse: cheaper environments get higher scores
	return 1.0 / (1.0 + env.Static.CostPerHour)
}

// scoreAvailableResources scores by available capacity. More headroom = higher score.
func scoreAvailableResources(env *EnvironmentInfo) float64 {
	cpu := float64(env.Dynamic.AvailableCPUMillicores)
	mem := float64(env.Dynamic.AvailableMemoryMB)

	if cpu <= 0 && mem <= 0 {
		return 0.5 // neutral if no data
	}

	// Normalize: assume 64 cores (64000m) and 256GB (262144MB) as "maximum"
	cpuScore := math.Min(cpu/64000.0, 1.0)
	memScore := math.Min(mem/262144.0, 1.0)

	return (cpuScore + memScore) / 2.0
}

// scoreRunningResources scores by how few resources are running (less busy = higher score).
func scoreRunningResources(env *EnvironmentInfo) float64 {
	if env.Static.MaxReplicas <= 0 {
		return 0.5
	}
	used := float64(env.Dynamic.RunningResources) / float64(env.Static.MaxReplicas)
	return 1.0 - math.Min(used, 1.0)
}
