/*
Copyright Kurator Authors.

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

package pipeline

import (
	"context"
	"fmt"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"kurator.dev/kurator/pkg/client"
	"kurator.dev/kurator/pkg/generic"
	"os"
)

type DescribeArgs struct {
	PipelineName string
}

// describeOptions is the option about command "kurator pipeline describe"
type describeOptions struct {
	options *generic.Options
	output  string
}

// pipelineDescribe is the struct for command for describe pipeline obj
type pipelineDescribe struct {
	*client.Client

	args    *DescribeArgs
	options *generic.Options
}

func NewPipelineDescribe(opts *generic.Options, args *DescribeArgs) (*pipelineDescribe, error) {
	pDescribe := &pipelineDescribe{
		options: opts,
		args:    args,
	}
	rest := opts.RESTClientGetter()
	c, err := client.NewClient(rest)
	if err != nil {
		return nil, err
	}
	pDescribe.Client = c

	return pDescribe, nil
}

func (p *pipelineDescribe) DescribeExecute() error {
	// 获取集群中的所有节点
	nodeList := &corev1.NodeList{}
	if err := p.Client.CtrlRuntimeClient().List(context.Background(), nodeList); err != nil {
		fmt.Fprintf(os.Stderr, "获取节点信息失败: %v\n", err)
		os.Exit(1)
	}
	// 打印节点信息
	fmt.Println("集群节点列表 :")
	for _, node := range nodeList.Items {
		fmt.Println(node.Name)
	}

	taskRunList := &tektonapi.TaskRunList{}
	if err := p.CtrlRuntimeClient().List(context.Background(), taskRunList); err != nil {
		fmt.Fprintf(os.Stderr, "获取 Pipeline 列表失败: %v\n", err)
		os.Exit(1)
	}

	// 打印 Pipeline 的名称
	fmt.Println("Pipeline:")
	for _, tr := range taskRunList.Items {
		fmt.Println(tr.Name)
	}

	return nil
}
