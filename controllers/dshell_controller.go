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
	"bufio"
	"context"
	dshellv1beta1 "dshell/api/v1beta1"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DShellReconciler reconciles a DShell object
type DShellReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

const ShellPath = "/bin/bash"

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
	r.log = log.FromContext(ctx)
	r.log.V(1).Info(fmt.Sprintf("current reconcile request is %v", req))

	var ds dshellv1beta1.DShell
	if err := r.Get(ctx, req.NamespacedName, &ds); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	podName, err := r.getPodName()
	if err != nil {
		r.log.V(1).Error(err, "key pod name error")
	}

	podIp, err := r.getPodIp()
	if err != nil {
		r.log.V(1).Error(err, "key pod ip error")
	}

	// return with not to requeue, if the current pod has already executed the CR Shell command.
	if r.isAlreadyExecuted(ds.Status.NodesResults, podName, podIp) {
		return ctrl.Result{}, nil
	}

	// todo 当存在变量第一次出现的作用域比后续使用到的作用域小时，需要在较大的作用域下进行变量声明，而如果需要声明的变量较多，使用声明块更整洁
	var (
		stdout    string
		stderr    string
		startTime v1.Time
		endTime   v1.Time
		ctrlErr   string
	)
	if podName == "" || podIp == "" {
		ctrlErr = "there are not pod keys in some pods, please check pod log for detail."
	} else {
		r.log.V(1).Info(fmt.Sprintf("to execute the command %v", ds.Spec.Command))
		startTime = v1.Now()
		stdout, stderr = r.executeCommand(ctx, ds.Spec.Command, ds.Spec.TimeoutMs)
		endTime = v1.Now()
		r.log.V(1).Info(fmt.Sprintf("the command %v execution stdout is %v, stderr is %v, start time is %v, end time is %v",
			ds.Spec.Command, stdout, stderr, startTime, endTime))
	}

	// NodesResults is nil when the first pod to process CR arrives, which needs to be initialized
	if ds.Status.NodesResults == nil {
		ds.Status.NodesResults = make([]dshellv1beta1.ExecResult, 0)
	}

	ds.Status.NodesResults = append(ds.Status.NodesResults, dshellv1beta1.ExecResult{
		PodName:   podName,
		PodIp:     podIp,
		StartTime: startTime,
		EndTime:   endTime,
		Stdout:    stdout,
		Stderr:    stderr,
		CtrlErr:   ctrlErr,
	})
	if err := r.Status().Update(context.Background(), &ds); err != nil {
		if !errors.IsConflict(err) {
			r.log.V(1).Error(err, fmt.Sprintf("resource %v status update failed", req))
			return ctrl.Result{Requeue: true}, err
		}

		// use the atomicity of the k8s API server to achieve concurrency
		// because a new update event will be emitted when the old version is updated by another controller,
		// it is not required to requeue here.
		r.log.Info(fmt.Sprintf("pod %v %v race lock failed.", podName, podIp))
	}

	return ctrl.Result{}, nil
}

// executeCommand start a shell process in the controller pod and execute the command
func (r *DShellReconciler) executeCommand(ctx context.Context, command string, timeout int64) (stdout, stderr string) {

	// command execution timeout, to implement by controlling the time to read the standard  stream
	// handle the timeout of commands such as ping, the timeout value is set by .spec.timeout of CR
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*time.Duration(timeout))
	defer func() {
		cancel()
	}()

	cmd := exec.CommandContext(ctx, ShellPath, "-c", command)
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdout = err.Error()
		return
	}

	errPipe, err := cmd.StderrPipe()
	if err != nil {
		stderr = err.Error()
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	// process the standard output stream
	go r.readWithCancel(ctx, &wg, bufio.NewReader(outPipe), &stdout)
	// process the standard error stream
	go r.readWithCancel(ctx, &wg, bufio.NewReader(errPipe), &stderr)

	if err = cmd.Start(); err != nil {
		stderr = err.Error()
		return
	}

	// blocks waiting for standard output and standard error to be processed
	wg.Wait()
	return
}

// isAlreadyExecuted check whether the current pod has been executed,
// depending on whether there is an element record with the same pod IP and pod name in the status list in the CR
func (r *DShellReconciler) isAlreadyExecuted(results []dshellv1beta1.ExecResult, podName, podIp string) bool {
	// an empty list means the CR is processed for the first time
	if results == nil {
		return false
	}

	for _, result := range results {
		// todo 用稳定的信息作为 key 在 k8s 中 pod 的 ip 可能会因为 pod 重启而更改
		if result.PodIp == podIp && result.PodName == podName {
			return true
		}
	}
	return false
}

// getPodIp get the IP assigned by the CIDR of the current pod in the k8s cluster
func (r *DShellReconciler) getPodIp() (string, error) {
	return r.cmdOutput("hostname -i")
}

// getPodIp get the name assigned to the current pod in the k8s cluster
func (r *DShellReconciler) getPodName() (string, error) {
	return r.cmdOutput("hostname")
}

func (r *DShellReconciler) cmdOutput(cmd string) (rs string, re error) {
	op, re := exec.Command(ShellPath, "-c", cmd).CombinedOutput()
	if re != nil {
		return "", re
	}

	if rs = string(purify(op)); rs == "" {
		return rs, fmt.Errorf("command result %v is empty", cmd)
	}

	r.log.V(1).Info(fmt.Sprintf("command %v's result is %v", cmd, rs))
	return
}

func (r *DShellReconciler) readWithCancel(ctx context.Context, wg *sync.WaitGroup, rd *bufio.Reader, res *string) {
	s := make([]byte, 0)
	defer func() {
		// the stream is processed
		*res = string(purify(s))
		r.log.Info(fmt.Sprintf("read result: %v", *res))
		wg.Done()
	}()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				s = append(s, []byte(fmt.Sprintf("timeout cancled: %q", ctx.Err()))...)
			} else {
				s = append(s, []byte("interrupted")...)
			}
			return
		default:
			rs, err := rd.ReadSlice('\n')
			if err != nil || err == io.EOF {
				return
			}
			s = append(s, rs...)
		}
	}
}

// purify the standard output stream and standard error tend to end with a newline,
// which affects reading when kubectl describe outputs, remove it here
func purify(in []byte) (out []byte) {
	if in == nil {
		return nil
	}

	if l := len(in); l > 0 && in[l-1] == '\n' {
		return in[:l-1]
	}

	return in
}

// SetupWithManager sets up the controller with the Manager.
func (r *DShellReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dshellv1beta1.DShell{}).
		Complete(r)
}
