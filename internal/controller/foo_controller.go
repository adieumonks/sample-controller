/*
Copyright 2024.

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

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	samplecontrollerv1alpha1 "github.com/adieumonks/sample-controller/api/v1alpha1"
	"github.com/go-logr/logr"
)

// FooReconciler reconciles a Foo object
type FooReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=samplecontroller.adieumonks.github.io,resources=foos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=samplecontroller.adieumonks.github.io,resources=foos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=samplecontroller.adieumonks.github.io,resources=foos/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Foo object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *FooReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var foo samplecontrollerv1alpha1.Foo
	logger.Info("fetching Foo Resource")
	if err := r.Get(ctx, req.NamespacedName, &foo); err != nil {
		logger.Error(err, "unable to fetch Foo")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.cleanupOwnedResources(ctx, logger, &foo); err != nil {
		logger.Error(err, "failed to clean up old Deployment resources for this Foo")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FooReconciler) cleanupOwnedResources(ctx context.Context, log logr.Logger, foo *samplecontrollerv1alpha1.Foo) error {
	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, client.InNamespace(foo.Namespace), client.MatchingFields(map[string]string{deploymentOwnerKey: foo.Name})); err != nil {
		log.Error(err, "unable to list owned Deployments")
		return err
	}

	for _, deployment := range deployments.Items {
		if deployment.Name == foo.Spec.DeploymentName {
			continue
		}

		if err := r.Delete(ctx, &deployment); err != nil {
			log.Error(err, "failed to delete Deployment resource")
			return err
		}

		log.Info("delete deployment resource: " + deployment.Name)
		r.Recorder.Eventf(foo, corev1.EventTypeNormal, "Deleted", "Deleted deployement %q", deployment.Name)
	}

	return nil
}

var (
	deploymentOwnerKey = ".metadata.controller"
	apiGVStr           = samplecontrollerv1alpha1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *FooReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1.Deployment{}, deploymentOwnerKey, func(rawObj client.Object) []string {
		deployment := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deployment)
		if owner != nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != "Foo" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&samplecontrollerv1alpha1.Foo{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
