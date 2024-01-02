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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kurator.dev/kurator/pkg/client"
	"kurator.dev/kurator/pkg/generic"
	"os"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
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
	Namespace string
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

type PipelineRunValue struct {
	Name              string
	CreationTimestamp metav1.Time
	Namespace         string
	CreatorPipeline   string
}

// ListExecute retrieves and prints a formatted list of PipelineRuns.
func (p *pipelineList) ListExecute() error {
	// 创建 ListOptions，设置 Namespace 筛选条件
	listOpts := &ctrlclient.ListOptions{
		Namespace: p.args.Namespace,
	}
	// Get all pipelineRuns
	pipelineRunList := &tektonapi.PipelineRunList{}
	if err := p.CtrlRuntimeClient().List(context.Background(), pipelineRunList, listOpts); err != nil {
		fmt.Fprintf(os.Stderr, "failed to get PipelineRunList: %v\n", err)
		return err
	}

	valueList := []PipelineRunValue{}
	for _, tr := range pipelineRunList.Items {
		valueList = append(valueList, PipelineRunValue{
			Name:              tr.Name,
			CreationTimestamp: tr.CreationTimestamp,
			Namespace:         tr.Namespace,
			CreatorPipeline:   tr.Spec.PipelineRef.Name,
		})
	}

	groupedRuns := GroupAndSortPipelineRuns(valueList)

	// Print the formatted list
	fmt.Println("------------------------------------- Pipeline Execution -----------------------------")
	fmt.Println("  Execution Name          |   Creation Time     |   Namespace      | Creator Pipeline")
	fmt.Println("--------------------------------------------------------------------------------------")

	for _, runs := range groupedRuns {
		for _, tr := range runs {
			fmt.Printf("%-25s | %-20s | %-16s | %s\n",
				tr.Name,
				tr.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
				tr.Namespace,
				tr.CreatorPipeline)
		}
	}

	return nil
}

// GroupAndSortPipelineRuns groups PipelineRunValues by CreatorPipeline and sorts each group by CreationTimestamp.
func GroupAndSortPipelineRuns(runs []PipelineRunValue) map[string][]PipelineRunValue {
	groups := make(map[string][]PipelineRunValue)
	for _, run := range runs {
		groups[run.CreatorPipeline] = append(groups[run.CreatorPipeline], run)
	}

	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].CreationTimestamp.Time.Before(group[j].CreationTimestamp.Time)
		})
	}

	return groups
}
