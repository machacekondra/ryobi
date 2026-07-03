package placement

import (
	"fmt"
	"strings"
)

// Engine selects the best environment for a resource based on constraints and preferences.
type Engine interface {
	Place(request PlacementRequest, environments []EnvironmentInfo) (*PlacementResult, error)
}

// DefaultEngine implements the placement Engine with constraint filtering + preference scoring.
type DefaultEngine struct{}

// NewEngine creates a new DefaultEngine.
func NewEngine() Engine {
	return &DefaultEngine{}
}

func (e *DefaultEngine) Place(request PlacementRequest, environments []EnvironmentInfo) (*PlacementResult, error) {
	// Step 1: Filter by hard constraints
	candidates := e.filterByConstraints(request, environments)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no environment matches constraints for resource type %q", request.ResourceType)
	}

	// Step 2: Score by preferences
	best := e.scoreAndSelect(request, candidates)

	// Step 3: Find the recipe for this resource type
	recipeName, ok := best.RecipeTypes[request.ResourceType]
	if !ok {
		return nil, fmt.Errorf("environment %q has no recipe for resource type %q", best.Name, request.ResourceType)
	}

	return &PlacementResult{
		EnvironmentName: best.Name,
		RecipeName:      recipeName,
	}, nil
}

// filterByConstraints removes environments that don't meet hard requirements.
func (e *DefaultEngine) filterByConstraints(request PlacementRequest, environments []EnvironmentInfo) []EnvironmentInfo {
	var result []EnvironmentInfo

	for _, env := range environments {
		if !env.Connected {
			continue
		}

		// Must support the resource type
		if _, ok := env.RecipeTypes[request.ResourceType]; !ok {
			continue
		}

		c := request.Constraints

		// Region constraint
		if c.Region != "" && !strings.EqualFold(env.Static.Region, c.Region) {
			continue
		}

		// Sovereignty constraint
		if c.Sovereignty != "" && !strings.EqualFold(env.Static.Sovereignty, c.Sovereignty) {
			continue
		}

		// Required capabilities
		if len(c.Capabilities) > 0 && !hasAllCapabilities(env.Static.Capabilities, c.Capabilities) {
			continue
		}

		result = append(result, env)
	}

	return result
}

// scoreAndSelect ranks candidates by preferences and returns the best one.
func (e *DefaultEngine) scoreAndSelect(request PlacementRequest, candidates []EnvironmentInfo) *EnvironmentInfo {
	if len(candidates) == 1 {
		return &candidates[0]
	}

	bestIdx := 0
	bestScore := -1.0

	for i := range candidates {
		score := e.computeScore(request.Preferences, &candidates[i])
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return &candidates[bestIdx]
}

// computeScore calculates a weighted score for an environment based on preferences.
func (e *DefaultEngine) computeScore(prefs Preferences, env *EnvironmentInfo) float64 {
	score := 0.0
	weights := 0.0

	if prefs.Cost == "minimize" {
		score += scoreCost(env)
		weights++
	}

	if prefs.AvailableResources == "maximize" {
		score += scoreAvailableResources(env)
		weights++
	}

	// Default: score by running resources (prefer less busy)
	if weights == 0 {
		return scoreRunningResources(env)
	}

	return score / weights
}

func hasAllCapabilities(have []string, need []string) bool {
	set := make(map[string]bool, len(have))
	for _, c := range have {
		set[strings.ToLower(c)] = true
	}
	for _, c := range need {
		if !set[strings.ToLower(c)] {
			return false
		}
	}
	return true
}
