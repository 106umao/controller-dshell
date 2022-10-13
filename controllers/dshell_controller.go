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

	dshellv1beta1 "dshell/api/v1beta1"
)

// DShellReconciler reconciles a DShell object
type DShellReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

const (
	ShellPath = "/bin/bash"
)

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
	r.log.Info(fmt.Sprintf("current reconcile request is %v", req))

	var ds dshellv1beta1.DShell
	if err := r.Get(ctx, req.NamespacedName, &ds); err != nil {
		r.log.Info(fmt.Sprintf("%v unable to fetch DShell or it have been delted.", req))
		return ctrl.Result{Requeue: true}, client.IgnoreNotFound(err)
	}

	// have already executed.
	if r.isAlreadyExecuted(ds.Status.NodesResults) {
		return ctrl.Result{Requeue: true}, nil
	}

	var (
		cmd       = ds.Spec.Command
		startTime v1.Time
		endTime   v1.Time
		// 命令执行超时时间，通过控制读取标准输出流和标准错误流的时间实现
		timeout = ds.Spec.Timeout
	)

	ctx, cancel := context.WithCancel(ctx)
	go func(cancelFunc context.CancelFunc) {
		// 处理如 ping 这类命令的超时情况，timeout 值由 CR的 .spec.timeout 设置
		time.Sleep(time.Millisecond * time.Duration(timeout))
		cancelFunc()
	}(cancel)

	startTime = v1.Now()
	stdout, stderr := r.executeCommand(ctx, cmd)
	r.log.Info(fmt.Sprintf("the execution stdout %v, stderr %v", stdout, stderr))
	endTime = v1.Now()

	// 切片 NodesResult 第一次被设值时为 nil，需要初始化
	if ds.Status.NodesResults == nil {
		ds.Status.NodesResults = make([]dshellv1beta1.ExecResult, 0)
	}

	ds.Status.NodesResults = append(ds.Status.NodesResults, dshellv1beta1.ExecResult{
		PodName:   r.getPodName(),
		PodIp:     r.getPodIp(),
		StartTime: startTime,
		EndTime:   endTime,
		Stdout:    stdout,
		Stderr:    stderr,
	})

	if err := r.Status().Update(context.Background(), &ds); err != nil {
		if !errors.IsConflict(err) {
			r.log.Info(fmt.Sprintf("resource %v status update failed", req))
			return ctrl.Result{}, err
		}

		// 采用乐观机制，update 时采用 cas 的方式对 api-server 进行更行，更行失败时返回 Requeue=true
		// 此时不重新入队时因为 old 版本被更新时会出发新的 update 事件
		r.log.Info(fmt.Sprintf("pod %v %v race lock failed.", r.getPodName(), r.getPodIp()))
	}

	return ctrl.Result{Requeue: true}, nil
}

// executeCommand 在 controller pod 中执行 shell 命令
func (r *DShellReconciler) executeCommand(ctx context.Context, cmd string) (stdout, stderr string) {
	if cmd == "" {
		stderr = "dshell controller: command is blank."
	}

	r.log.Info(fmt.Sprintf("to execute the command %v", cmd))
	c := exec.CommandContext(ctx, ShellPath, "-c", cmd)
	outPipe, err := c.StdoutPipe()
	if err != nil {
		stdout = err.Error()
		return
	}

	errPipe, err := c.StderrPipe()
	if err != nil {
		stderr = err.Error()
		return
	}

	var (
		stdoRd    = bufio.NewReader(outPipe)
		stdeRd    = bufio.NewReader(errPipe)
		stdoSlice = make([]byte, 0)
		stdeSlice = make([]byte, 0)
		wg        = sync.WaitGroup{}
	)
	wg.Add(2)

	// 处理标准输出流
	go func() {
		// 标准输出流处理完毕
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				if ctx.Err() != nil {
					stdoSlice = append(stdoSlice, []byte(fmt.Sprintf("timeout cancled: %q", ctx.Err()))...)
				} else {
					stdoSlice = append(stdoSlice, []byte("interupted")...)
				}
				return
			default:
				s, err := stdoRd.ReadSlice('\n')
				if err != nil || err == io.EOF {
					return
				}
				stdoSlice = append(stdoSlice, s...)
			}
		}
	}()

	// 处理标准错误流
	go func() {
		// 标准错误流处理完毕
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				if ctx.Err() != nil {
					stdeSlice = append(stdeSlice, []byte(fmt.Sprintf("timeout cancled: %q", ctx.Err()))...)
				} else {
					stdeSlice = append(stdeSlice, []byte("interupted")...)
				}
				return
			default:
				stderrSlice, err := stdeRd.ReadSlice('\n')
				if err != nil || err == io.EOF {
					break
				}
				stdeSlice = append(stdeSlice, stderrSlice...)
			}
		}
	}()

	if err = c.Start(); err != nil {
		stderr = err.Error()
		return
	}

	// 阻塞等待标准输入流和标准错误流都处理完
	wg.Wait()

	return string(stdoSlice), string(stdeSlice)
}

// isAlreadyExecuted 判断当前 reconciler 是否执行过，取决于 CR 中 status 列表是否有 pod ip 和 pod name 相同的元素
func (r *DShellReconciler) isAlreadyExecuted(resulsts []dshellv1beta1.ExecResult) bool {
	// 列表为空表示第一次执行
	if resulsts == nil {
		return false
	}

	for _, result := range resulsts {
		if result.PodIp == r.getPodIp() && result.PodName == r.getPodName() {
			return true
		}
	}
	return false
}

// getPodIp 获取当前 pod 在 k8s 集群中 CIDR 分配到的 IP
func (r *DShellReconciler) getPodIp() string {
	op, err := exec.Command(ShellPath, "-c", "hostname -i").CombinedOutput()
	if err != nil {
		r.log.Info("pod ip not found")
		return "pod ip not found"
	}
	r.log.Info(fmt.Sprintf("pod ip is %v", string(op)))
	return string(op)
}

// getPodIp 获取当前 pod 在 k8s 集群中分配到的名称
func (r *DShellReconciler) getPodName() string {
	op, err := exec.Command(ShellPath, "-c", "hostname").CombinedOutput()
	if err != nil {
		r.log.Info("pod name not found")
		return "pod name not found"
	}
	r.log.Info(fmt.Sprintf("pod ip is %v", string(op)))
	return string(op)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DShellReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dshellv1beta1.DShell{}).
		Complete(r)
}
