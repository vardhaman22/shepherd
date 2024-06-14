/*
Copyright 2024 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v1

import (
	"context"
	"sync"
	"time"

	v1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PlanController interface for managing Plan resources.
type PlanController interface {
	generic.ControllerInterface[*v1.Plan, *v1.PlanList]
}

// PlanClient interface for managing Plan resources in Kubernetes.
type PlanClient interface {
	generic.ClientInterface[*v1.Plan, *v1.PlanList]
}

// PlanCache interface for retrieving Plan resources in memory.
type PlanCache interface {
	generic.CacheInterface[*v1.Plan]
}

// PlanStatusHandler is executed for every added or modified Plan. Should return the new status to be updated
type PlanStatusHandler func(obj *v1.Plan, status v1.PlanStatus) (v1.PlanStatus, error)

// PlanGeneratingHandler is the top-level handler that is executed for every Plan event. It extends PlanStatusHandler by a returning a slice of child objects to be passed to apply.Apply
type PlanGeneratingHandler func(obj *v1.Plan, status v1.PlanStatus) ([]runtime.Object, v1.PlanStatus, error)

// RegisterPlanStatusHandler configures a PlanController to execute a PlanStatusHandler for every events observed.
// If a non-empty condition is provided, it will be updated in the status conditions for every handler execution
func RegisterPlanStatusHandler(ctx context.Context, controller PlanController, condition condition.Cond, name string, handler PlanStatusHandler) {
	statusHandler := &planStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, generic.FromObjectHandlerToHandler(statusHandler.sync))
}

// RegisterPlanGeneratingHandler configures a PlanController to execute a PlanGeneratingHandler for every events observed, passing the returned objects to the provided apply.Apply.
// If a non-empty condition is provided, it will be updated in the status conditions for every handler execution
func RegisterPlanGeneratingHandler(ctx context.Context, controller PlanController, apply apply.Apply,
	condition condition.Cond, name string, handler PlanGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &planGeneratingHandler{
		PlanGeneratingHandler: handler,
		apply:                 apply,
		name:                  name,
		gvk:                   controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterPlanStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type planStatusHandler struct {
	client    PlanClient
	condition condition.Cond
	handler   PlanStatusHandler
}

// sync is executed on every resource addition or modification. Executes the configured handlers and sends the updated status to the Kubernetes API
func (a *planStatusHandler) sync(key string, obj *v1.Plan) (*v1.Plan, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type planGeneratingHandler struct {
	PlanGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
	seen  sync.Map
}

// Remove handles the observed deletion of a resource, cascade deleting every associated resource previously applied
func (a *planGeneratingHandler) Remove(key string, obj *v1.Plan) (*v1.Plan, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1.Plan{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	if a.opts.UniqueApplyForResourceVersion {
		a.seen.Delete(key)
	}

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

// Handle executes the configured PlanGeneratingHandler and pass the resulting objects to apply.Apply, finally returning the new status of the resource
func (a *planGeneratingHandler) Handle(obj *v1.Plan, status v1.PlanStatus) (v1.PlanStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.PlanGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}
	if !a.isNewResourceVersion(obj) {
		return newStatus, nil
	}

	err = generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
	if err != nil {
		return newStatus, err
	}
	a.storeResourceVersion(obj)
	return newStatus, nil
}

// isNewResourceVersion detects if a specific resource version was already successfully processed.
// Only used if UniqueApplyForResourceVersion is set in generic.GeneratingHandlerOptions
func (a *planGeneratingHandler) isNewResourceVersion(obj *v1.Plan) bool {
	if !a.opts.UniqueApplyForResourceVersion {
		return true
	}

	// Apply once per resource version
	key := obj.Namespace + "/" + obj.Name
	previous, ok := a.seen.Load(key)
	return !ok || previous != obj.ResourceVersion
}

// storeResourceVersion keeps track of the latest resource version of an object for which Apply was executed
// Only used if UniqueApplyForResourceVersion is set in generic.GeneratingHandlerOptions
func (a *planGeneratingHandler) storeResourceVersion(obj *v1.Plan) {
	if !a.opts.UniqueApplyForResourceVersion {
		return
	}

	key := obj.Namespace + "/" + obj.Name
	a.seen.Store(key, obj.ResourceVersion)
}
