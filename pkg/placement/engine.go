package placement

import (
	"fmt"
	"sort"
	"strings"
)

// Engine selects the best environment for a resource based on administrator-defined placement rules.
type Engine interface {
	Place(request PlacementRequest, rules []PlacementRule, environments []EnvironmentInfo) (*PlacementResult, error)
}

// DefaultEngine implements the placement Engine.
type DefaultEngine struct{}

// NewEngine creates a new DefaultEngine.
func NewEngine() Engine {
	return &DefaultEngine{}
}

func (e *DefaultEngine) Place(request PlacementRequest, rules []PlacementRule, environments []EnvironmentInfo) (*PlacementResult, error) {
	// Find matching rules for this resource type, sorted by priority (highest first)
	matchingRules := e.findMatchingRules(request.ResourceType, rules)
	if len(matchingRules) == 0 {
		// No placement rules — fall back to any connected environment that supports the type
		return e.placeFallback(request.ResourceType, environments)
	}

	// Try each rule in priority order until one produces a result
	for _, rule := range matchingRules {
		result, err := e.placeWithRule(request.ResourceType, &rule, environments)
		if err == nil {
			result.RuleName = rule.Name
			return result, nil
		}
	}

	return nil, fmt.Errorf("no environment matches placement rules for resource type %q", request.ResourceType)
}

func (e *DefaultEngine) findMatchingRules(resourceType string, rules []PlacementRule) []PlacementRule {
	var matched []PlacementRule
	for _, rule := range rules {
		if strings.EqualFold(rule.Properties.ResourceType, resourceType) {
			matched = append(matched, rule)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Properties.Priority > matched[j].Properties.Priority
	})
	return matched
}

func (e *DefaultEngine) placeWithRule(resourceType string, rule *PlacementRule, environments []EnvironmentInfo) (*PlacementResult, error) {
	candidates := filterByConstraints(resourceType, rule.Properties.Constraints, environments)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates for rule %q", rule.Name)
	}

	best := scoreAndSelect(rule.Properties.Preferences, candidates)
	recipeName, ok := best.RecipeTypes[resourceType]
	if !ok {
		return nil, fmt.Errorf("environment %q has no recipe for %q", best.Name, resourceType)
	}

	return &PlacementResult{
		EnvironmentName: best.Name,
		RecipeName:      recipeName,
	}, nil
}

func (e *DefaultEngine) placeFallback(resourceType string, environments []EnvironmentInfo) (*PlacementResult, error) {
	candidates := filterByConstraints(resourceType, Constraints{}, environments)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no connected environment supports resource type %q", resourceType)
	}

	best := scoreAndSelect(Preferences{}, candidates)
	recipeName, ok := best.RecipeTypes[resourceType]
	if !ok {
		return nil, fmt.Errorf("environment %q has no recipe for %q", best.Name, resourceType)
	}

	return &PlacementResult{
		EnvironmentName: best.Name,
		RecipeName:      recipeName,
	}, nil
}

func filterByConstraints(resourceType string, constraints Constraints, environments []EnvironmentInfo) []EnvironmentInfo {
	var result []EnvironmentInfo
	for _, env := range environments {
		if !env.Connected {
			continue
		}
		if _, ok := env.RecipeTypes[resourceType]; !ok {
			continue
		}
		if constraints.Region != "" && !strings.EqualFold(env.Static.Region, constraints.Region) {
			continue
		}
		if constraints.Sovereignty != "" && !strings.EqualFold(env.Static.Sovereignty, constraints.Sovereignty) {
			continue
		}
		if len(constraints.Capabilities) > 0 && !hasAllCapabilities(env.Static.Capabilities, constraints.Capabilities) {
			continue
		}
		result = append(result, env)
	}
	return result
}

func scoreAndSelect(prefs Preferences, candidates []EnvironmentInfo) *EnvironmentInfo {
	if len(candidates) == 1 {
		return &candidates[0]
	}

	bestIdx := 0
	bestScore := -1.0
	for i := range candidates {
		score := computeScore(prefs, &candidates[i])
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return &candidates[bestIdx]
}

func computeScore(prefs Preferences, env *EnvironmentInfo) float64 {
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
