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

const (
	ShellPath     = "/bin/bash"
	UnKnowPodName = "UnKnowPodName"
	UnKnowPodIp   = "UnKnowPodIp"
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

	var (
		ds dshellv1beta1.DShell
		// todo 变量名见名知意，可以缩写不能太短，本来就很短的一般不用缩写
		pn string
		pi string
	)
	if err := r.Get(ctx, req.NamespacedName, &ds); err != nil {
		// todo Info Error 日志分组输出等，了解库之后做最佳实践
		// todo 明确预期之内的需求
		r.log.Info(fmt.Sprintf("%v unable to fetch DShell or it have been deleted.", req))
		return ctrl.Result{Requeue: true}, client.IgnoreNotFound(err)
	}

	pn, err := r.getPodName()
	if err != nil {
		// todo 一个变量不能出现两个含义
		// todo 不能占用户合法的名称空间
		pn = UnKnowPodName
		r.log.Error(err, "key pod name error")
	}

	pi, err = r.getPodIp()
	if err != nil {
		pi = UnKnowPodIp
		r.log.Error(err, "key pod ip error")
	}

	// 当前 pod 已经执行过 CR Shell 命令
	if r.isAlreadyExecuted(ds.Status.NodesResults, pn, pi) {
		return ctrl.Result{Requeue: true}, nil
	}

	// todo 当存在变量第一次出现的作用域比后续使用到的作用域小时，需要在较大的作用域下进行变量声明，而如果需要声明的变量较多，使用声明块更整洁
	var (
		stdout    string
		stderr    string
		startTime v1.Time
		endTime   v1.Time
	)
	if pn == UnKnowPodName || pi == UnKnowPodIp {
		// todo 一个变量不能有两个含义
		stderr = "there are not pod keys in some pods, please check pod log for detail."
	} else {
		startTime = v1.Now()
		stdout, stderr = r.executeCommand(ctx, ds.Spec.Command, ds.Spec.Timeout)
		endTime = v1.Now()
		// todo 通常在命令执行前记录
		r.log.Info(fmt.Sprintf("the command %v execution stdout %v, stderr %v", ds.Spec.Command, stdout, stderr))
	}

	// todo 注释用英文
	// 第一个处理 CR 的 pod 达到时 NodesResult 为 nil，需要初始化
	if ds.Status.NodesResults == nil {
		ds.Status.NodesResults = make([]dshellv1beta1.ExecResult, 0)
	}

	ds.Status.NodesResults = append(ds.Status.NodesResults, dshellv1beta1.ExecResult{
		PodName:   pn,
		PodIp:     pi,
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

		r.log.Info(fmt.Sprintf("pod %v %v race lock failed.", pn, pi))
	}

	// 采用 k8s api server 的乐观机制实现并发
	// 因为 old 版本被更新时会出发新的 update 事件，所以在此不重新入队时
	return ctrl.Result{Requeue: true}, nil
}

// executeCommand 在 controller pod 中执行 shell 命令
func (r *DShellReconciler) executeCommand(ctx context.Context, cmd string, timeout int64) (stdout, stderr string) {
	if cmd == "" {
		// todo 同上，一个变量不能有两个语义
		stderr = "dshell: command is blank."
	}

	// 命令执行超时时间，通过控制读取标准输出流和标准错误流的时间实现
	// 处理如 ping 这类命令的超时情况，timeout 值由 CR的 .spec.timeout 设置
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*time.Duration(timeout))
	defer func() {
		cancel()
	}()

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

	wg := sync.WaitGroup{}
	wg.Add(2)

	// 处理标准输出流
	go r.readWithCancel(ctx, &wg, bufio.NewReader(outPipe), &stdout)
	// 处理标准错误流
	go r.readWithCancel(ctx, &wg, bufio.NewReader(errPipe), &stderr)

	if err = c.Start(); err != nil {
		stderr = err.Error()
		return
	}

	// 阻塞等待标准输入流和标准错误流都处理完
	wg.Wait()
	return
}

// isAlreadyExecuted 判断当前 pod 是否执行过，取决于 CR 中 status 列表是否有 pod ip 和 pod name 相同的元素记录
func (r *DShellReconciler) isAlreadyExecuted(results []dshellv1beta1.ExecResult, pn, pi string) bool {
	// 列表为空表示 CR 第一次被处理
	if results == nil {
		return false
	}

	for _, result := range results {
		// todo 不能依赖于可变的key做condition
		if result.PodIp == pi && result.PodName == pn {
			return true
		}
	}
	return false
}

// getPodIp 获取当前 pod 在 k8s 集群中 CIDR 分配到的 IP
func (r *DShellReconciler) getPodIp() (string, error) {
	return r.cmdOutput("hostname -i")
}

// getPodIp 获取当前 pod 在 k8s 集群中分配到的名称
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

	r.log.Info(fmt.Sprintf("command %v's result is %v", cmd, rs))
	return
}

func (r *DShellReconciler) readWithCancel(ctx context.Context, wg *sync.WaitGroup, rd *bufio.Reader, res *string) {
	// 流处理完毕
	s := make([]byte, 0)
	defer func() {
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

// purify 标准输出流和标准错往往会以一个换行符结束，在 kubectl describe 输出时会影响阅读，在这里把它去掉
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
