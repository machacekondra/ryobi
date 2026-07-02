package controller

import (
	"encoding/json"

	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
)

// EnvironmentFromJSON converts JSON bytes to an Environment.
func EnvironmentFromJSON(body []byte) (*datamodel.Environment, error) {
	var env datamodel.Environment
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	env.Type = datamodel.EnvironmentResourceType
	return &env, nil
}

// EnvironmentToResponse converts an Environment to an API response body.
func EnvironmentToResponse(env *datamodel.Environment) (any, error) {
	return env, nil
}

// ApplicationFromJSON converts JSON bytes to an Application.
func ApplicationFromJSON(body []byte) (*datamodel.Application, error) {
	var app datamodel.Application
	if err := json.Unmarshal(body, &app); err != nil {
		return nil, err
	}
	app.Type = datamodel.ApplicationResourceType
	return &app, nil
}

// ApplicationToResponse converts an Application to an API response body.
func ApplicationToResponse(app *datamodel.Application) (any, error) {
	return app, nil
}

// ResourceFromJSON converts JSON bytes to a Resource.
func ResourceFromJSON(body []byte) (*datamodel.Resource, error) {
	var res datamodel.Resource
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	res.Type = datamodel.ResourceResourceType
	return &res, nil
}

// ResourceToResponse converts a Resource to an API response body.
func ResourceToResponse(res *datamodel.Resource) (any, error) {
	return res, nil
}
