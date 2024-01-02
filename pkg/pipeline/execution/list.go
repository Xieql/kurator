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

package execution

import (
	"context"
	"fmt"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"kurator.dev/kurator/pkg/client"
	"kurator.dev/kurator/pkg/generic"
	"os"
)

type PipelineStatus string

const (
	Ready   PipelineStatus = "ready"
	Unready PipelineStatus = "unready"
)

// Info is the status of pipeline
type Info struct {
	Name   string `yaml:"name"`
	Status string `yaml:"status"`
}

// listOptions is the option about command "kurator pipeline list"
type listOptions struct {
	options *generic.Options
	info    map[string]Info
	output  string
}

// pipelineList is the struct for command for list pipeline obj
type pipelineList struct {
	*client.Client

	args    *ListArgs
	options *generic.Options
}

type ListArgs struct {
	ExecutionName   string
	Namespace       string
	CreateTime      string
	CreatorPipeline string
}

func NewPipelineList(opts *generic.Options, args *ListArgs) (*pipelineList, error) {
	pList := &pipelineList{
		options: opts,
		args:    args,
	}
	rest := opts.RESTClientGetter()
	c, err := client.NewClient(rest)
	if err != nil {
		return nil, err
	}
	pList.Client = c

	return pList, nil
}

func (p *pipelineList) ListExecute() error {

	pipelineRunList := &tektonapi.PipelineRunList{}
	if err := p.CtrlRuntimeClient().List(context.Background(), pipelineRunList); err != nil {
		fmt.Fprintf(os.Stderr, "获取 Pipeline 列表失败: %v\n", err)
		os.Exit(1)
	}

	// 打印 Pipeline 的名称
	fmt.Println("------------------------------- Pipeline execution -----------------------")
	fmt.Println("  Execution Name           |   Creation Time     |   Namespace     | Creator Pipeline")
	fmt.Println("-------------------------------------------------------------------------")

	for _, tr := range pipelineRunList.Items {
		fmt.Printf("%-25s | %-16s | %-12s | %s\n",
			tr.Name,
			tr.CreationTimestamp.Format("2006-01-02 15:04:05"),
			tr.Namespace,
			tr.Spec.PipelineRef.Name)
	}

	return nil
}
