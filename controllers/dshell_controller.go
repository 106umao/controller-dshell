/*
Copyright 2022.

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
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dshellv1beta1 "dshell/api/v1beta1"
)

// DShellReconciler reconciles a DShell object
type DShellReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Generation int64
}

//+kubebuilder:rbac:groups=dshell.smartx.com,resources=dshells,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dshell.smartx.com,resources=dshells/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dshell.smartx.com,resources=dshells/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DShell object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *DShellReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("current reconcile request is %v", req))

	var ds dshellv1beta1.DShell
	if err := r.Get(ctx, req.NamespacedName, &ds); err != nil {
		logger.Info(fmt.Sprintf("%v unable to fetch DShell or it have been delted.", req))
		return ctrl.Result{Requeue: true}, client.IgnoreNotFound(err)
	}

	// have already executed.
	if isAlreadyExecuted(getPodIp(), ds.Status.NodesResults) {
		return ctrl.Result{Requeue: true}, nil
	}
	var (
		cmd       = ds.Spec.Command
		msg       string
		startTime v1.Time
		endTime   v1.Time
	)

	if cmd == "" {
		msg = "command is blank."
	}

	startTime = v1.Now()
	msg, err := r.executeCommand(ctx, cmd)
	endTime = v1.Now()
	if err != nil {
		return ctrl.Result{}, err
	}

	if ds.Status.NodesResults == nil {
		ds.Status.NodesResults = make([]dshellv1beta1.ExecResult, 0)
	}

	ds.Status.NodesResults = append(ds.Status.NodesResults, dshellv1beta1.ExecResult{
		ControllerPodIp: getPodIp(),
		StartTime:       startTime,
		EndTime:         endTime,
		Message:         msg,
	})

	if err := r.Status().Update(context.Background(), &ds); err != nil {
		logger.Info(fmt.Sprintf("pod %v race lock failed.", getPodIp()))
	}

	return ctrl.Result{Requeue: true}, nil
}

// executeCommand 在 controller pod 中执行 shell 命令
func (r *DShellReconciler) executeCommand(ctx context.Context, cmd string) (msg string, rerr error) {
	// todo execute
	return ",mock msg", nil
}

func isAlreadyExecuted(podIp string, resulsts []dshellv1beta1.ExecResult) bool {
	if resulsts == nil {
		return false
	}

	for _, r := range resulsts {
		if r.ControllerPodIp == podIp {
			return true
		}
	}
	return false
}

// getPodIp 获取当前 pod 在 k8s 中分配到的 ip
func getPodIp() string {
	// todo get ip
	return "mock ip"
}

// SetupWithManager sets up the controller with the Manager.
func (r *DShellReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dshellv1beta1.DShell{}).
		Complete(r)
}
