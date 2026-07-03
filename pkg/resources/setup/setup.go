package setup

import (
	ctrl "github.com/ryobi-project/ryobi/pkg/api/frontend/controller"
	"github.com/ryobi-project/ryobi/pkg/api/frontend/defaultoperation"
	"github.com/ryobi-project/ryobi/pkg/gateway"
	"github.com/ryobi-project/ryobi/pkg/resources/datamodel"
	resctrl "github.com/ryobi-project/ryobi/pkg/resources/frontend/controller"
)

const rootScope = "/api/v1"

// EnvironmentResourceOptions returns typed resource options for environments.
func EnvironmentResourceOptions() ctrl.ResourceOptions[datamodel.Environment] {
	return ctrl.ResourceOptions[datamodel.Environment]{
		RequestConverter:  resctrl.EnvironmentFromJSON,
		ResponseConverter: resctrl.EnvironmentToResponse,
		UpdateFilters: []ctrl.UpdateFilter[datamodel.Environment]{
			resctrl.ValidateEnvironmentUpdate,
		},
		DeleteFilters: []ctrl.DeleteFilter[datamodel.Environment]{
			resctrl.PreventEnvironmentDeleteIfInUse,
		},
	}
}

// ApplicationResourceOptions returns typed resource options for applications.
func ApplicationResourceOptions() ctrl.ResourceOptions[datamodel.Application] {
	return ctrl.ResourceOptions[datamodel.Application]{
		RequestConverter:  resctrl.ApplicationFromJSON,
		ResponseConverter: resctrl.ApplicationToResponse,
		UpdateFilters: []ctrl.UpdateFilter[datamodel.Application]{
			resctrl.ValidateApplicationUpdate,
		},
	}
}

// ResourceResourceOptions returns typed resource options for Terraform-managed resources.
func ResourceResourceOptions() ctrl.ResourceOptions[datamodel.Resource] {
	return ctrl.ResourceOptions[datamodel.Resource]{
		RequestConverter:  resctrl.ResourceFromJSON,
		ResponseConverter: resctrl.ResourceToResponse,
		UpdateFilters: []ctrl.UpdateFilter[datamodel.Resource]{
			resctrl.ValidateResourceUpdate,
		},
	}
}

// SetupRoutes registers all resource routes on the router.
func SetupRoutes(router *gateway.Router, opts ctrl.Options) {
	// Health check
	router.RegisterHealthCheck()

	// Operation status
	router.RegisterOperationStatus(resctrl.NewGetOperationStatus)

	envOpts := EnvironmentResourceOptions()
	appOpts := ApplicationResourceOptions()
	resOpts := ResourceResourceOptions()

	// Environments: sync CRUD (no async needed)
	router.RegisterResourceRoutes("/api/v1/environments", "ryobi/environments", gateway.ResourceFactories{
		List:   resctrl.NewGenericListFactory(rootScope),
		Get:    resctrl.NewGenericGetFactory(rootScope),
		Put:    defaultoperation.NewDefaultSyncPutFactory(rootScope, envOpts),
		Delete: defaultoperation.NewDefaultSyncDeleteFactory(rootScope, envOpts),
	})

	// Applications: sync CRUD
	router.RegisterResourceRoutes("/api/v1/applications", "ryobi/applications", gateway.ResourceFactories{
		List:   resctrl.NewGenericListFactory(rootScope),
		Get:    resctrl.NewGenericGetFactory(rootScope),
		Put:    defaultoperation.NewDefaultSyncPutFactory(rootScope, appOpts),
		Delete: defaultoperation.NewDefaultSyncDeleteFactory(rootScope, appOpts),
	})

	// Resources: async PUT/DELETE (triggers Terraform recipes)
	router.RegisterResourceRoutes("/api/v1/applications/{appName}/resources", "ryobi/resources", gateway.ResourceFactories{
		List:   resctrl.NewGenericListFactory(rootScope),
		Get:    resctrl.NewGenericGetFactory(rootScope),
		Put:    defaultoperation.NewDefaultAsyncPutFactory(rootScope, resOpts),
		Delete: defaultoperation.NewDefaultAsyncDeleteFactory(rootScope, resOpts),
	})

	// Credentials: sync CRUD
	router.RegisterResourceRoutes("/api/v1/credentials", "ryobi/credentials", gateway.ResourceFactories{
		List:   resctrl.NewGenericListFactory(rootScope),
		Get:    resctrl.NewGenericGetFactory(rootScope),
		Put:    resctrl.NewGenericPutFactory(rootScope),
		Delete: resctrl.NewGenericDeleteFactory(rootScope),
	})

	// Placements: admin-defined placement rules (sync CRUD)
	router.RegisterResourceRoutes("/api/v1/placements", "ryobi/placements", gateway.ResourceFactories{
		List:   resctrl.NewGenericListFactory(rootScope),
		Get:    resctrl.NewGenericGetFactory(rootScope),
		Put:    resctrl.NewGenericPutFactory(rootScope),
		Delete: resctrl.NewGenericDeleteFactory(rootScope),
	})

	// Catalog Items: reusable application templates (sync CRUD)
	router.RegisterResourceRoutes("/api/v1/catalog-items", "ryobi/catalog-items", gateway.ResourceFactories{
		List:   resctrl.NewGenericListFactory(rootScope),
		Get:    resctrl.NewGenericGetFactory(rootScope),
		Put:    resctrl.NewGenericPutFactory(rootScope),
		Delete: resctrl.NewGenericDeleteFactory(rootScope),
	})
}
