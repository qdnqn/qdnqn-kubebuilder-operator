/*
Copyright 2021.

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

package controllers

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	qdnqnv1 "clientmgr.io/tutorial/api/v1"
	"clientmgr.io/tutorial/pkg/resources"
	"clientmgr.io/tutorial/pkg/rest"
)

// ClientReconciler reconciles a Client object
type ClientReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=qdnqn.clientmgr.io,resources=clients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=qdnqn.clientmgr.io,resources=clients/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=qdnqn.clientmgr.io,resources=clients/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Client object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var clientResource = &qdnqnv1.Client{}
	if err := r.Get(ctx, req.NamespacedName, clientResource); err != nil {
		log.Error(err, "unable to fetch client")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	clientResourceOld := clientResource.DeepCopy()

	if clientResource.Status.ClientStatus == "" {
		clientResource.Status.ClientStatus = qdnqnv1.StatusPending
	}

	switch clientResource.Status.ClientStatus {
	case qdnqnv1.StatusPending:
		clientResource.Status.ClientStatus = qdnqnv1.StatusRunning

		err := r.Status().Update(context.TODO(), clientResource)
		if err != nil {
			log.Error(err, "failed to update client status")
			return ctrl.Result{}, err
		} else {
			log.Info("updated client status: " + clientResource.Status.ClientStatus)
			return ctrl.Result{Requeue: true}, nil
		}
	case qdnqnv1.StatusRunning:
		pod := resources.CreatePod(clientResource)

		query := &corev1.Pod{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.ObjectMeta.Name}, query)
		if err != nil && errors.IsNotFound(err) {
			if clientResource.Status.LastPodName == "" {
				err = ctrl.SetControllerReference(clientResource, pod, r.Scheme)
				if err != nil {
					return ctrl.Result{}, err
				}

				err = r.Create(context.TODO(), pod)
				if err != nil {
					return ctrl.Result{}, err
				}

				log.Info("pod created successfully", "name", pod.Name)

				return ctrl.Result{}, nil
			} else {
				clientResource.Status.ClientStatus = qdnqnv1.StatusCleaning
			}
		} else if err != nil {
			log.Error(err, "cannot get pod")
			return ctrl.Result{}, err
		} else if query.Status.Phase == corev1.PodFailed ||
			query.Status.Phase == corev1.PodSucceeded {
			log.Info("container terminated", "reason", query.Status.Reason, "message", query.Status.Message)

			clientResource.Status.ClientStatus = qdnqnv1.StatusCleaning
		} else if query.Status.Phase == corev1.PodRunning {
			if clientResource.Status.LastPodName != clientResource.Spec.ContainerImage+clientResource.Spec.ContainerTag {
				if query.Status.ContainerStatuses[0].Ready {
					log.Info("Trying to bind to: " + query.Status.PodIP)

					if !rest.GetClient(clientResource, query.Status.PodIP) {
						if rest.BindClient(clientResource, query.Status.PodIP) {
							log.Info("Client" + clientResource.Spec.ClientId + " is binded to pod " + query.ObjectMeta.GetName() + ".")
							clientResource.Status.ClientStatus = qdnqnv1.StatusCleaning
						} else {
							log.Info("Client not added.")
						}
					} else {
						log.Info("Client binded already.")
					}
				} else {
					log.Info("Container not ready, reschedule bind")
					return ctrl.Result{Requeue: true}, err
				}

				log.Info("Client last pod name: " + clientResource.Status.LastPodName)
				log.Info("Pod is running.")
			}
		} else if query.Status.Phase == corev1.PodPending {
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{Requeue: true}, err
		}

		if !reflect.DeepEqual(clientResourceOld.Status, clientResource.Status) {
			err = r.Status().Update(context.TODO(), clientResource)
			if err != nil {
				log.Error(err, "failed to update client status from running")
				return ctrl.Result{}, err
			} else {
				log.Info("updated client status RUNNING -> " + clientResource.Status.ClientStatus)
				return ctrl.Result{Requeue: true}, nil
			}
		}
	case qdnqnv1.StatusCleaning:
		query := &corev1.Pod{}
		HasClients := rest.HasClients(clientResource, query.Status.PodIP)

		err := r.Client.Get(ctx, client.ObjectKey{Namespace: clientResource.Namespace, Name: clientResource.Status.LastPodName}, query)
		if err == nil && clientResource.ObjectMeta.DeletionTimestamp.IsZero() {
			if !HasClients {
				err = r.Delete(context.TODO(), query)
				if err != nil {
					log.Error(err, "Failed to remove old pod")
					return ctrl.Result{}, err
				} else {
					log.Info("Old pod removed")
					return ctrl.Result{Requeue: true}, nil
				}
			}
		}

		if clientResource.Status.LastPodName != clientResource.Spec.ContainerImage+clientResource.Spec.ContainerTag {
			clientResource.Status.ClientStatus = qdnqnv1.StatusRunning
			clientResource.Status.LastPodName = clientResource.Spec.ContainerImage + clientResource.Spec.ContainerTag
		} else {
			clientResource.Status.ClientStatus = qdnqnv1.StatusPending
			clientResource.Status.LastPodName = ""
		}

		if !reflect.DeepEqual(clientResourceOld.Status, clientResource.Status) {
			err = r.Status().Update(context.TODO(), clientResource)
			if err != nil {
				log.Error(err, "failed to update client status from cleaning")
				return ctrl.Result{}, err
			} else {
				log.Info("updated client status CLEANING -> " + clientResource.Status.ClientStatus)
				return ctrl.Result{Requeue: true}, nil
			}
		}
	default:
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qdnqnv1.Client{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
