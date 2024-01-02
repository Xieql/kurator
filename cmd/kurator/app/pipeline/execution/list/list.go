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

package list

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"kurator.dev/kurator/pkg/generic"
	pipelinelist "kurator.dev/kurator/pkg/pipeline/execution"
)

var ListArgs = pipelinelist.ListArgs{}

func NewCmd(opts *generic.Options) *cobra.Command {
	PipelineListCmd := &cobra.Command{
		Use:     "list",
		Short:   "list the kurator pipeline",
		Example: getExample(),
		RunE: func(cmd *cobra.Command, args []string) error {

			PipelineList, err := pipelinelist.NewPipelineList(opts, &ListArgs)
			if err != nil {
				logrus.Errorf("pipeline init error: %v", err)
				return fmt.Errorf("pipeline init error: %v", err)
			}

			logrus.Debugf("start list pipeline obj, Global: %+v ", opts)
			if err := PipelineList.ListExecute(); err != nil {
				logrus.Errorf("pipeline execute error: %v", err)
				return fmt.Errorf("pipeline execute error: %v", err)
			}

			return nil
		},
	}

	PipelineListCmd.PersistentFlags().StringVarP(&ListArgs.Namespace, "namespace", "n", "default", "Comma separated list of namespace")
	PipelineListCmd.PersistentFlags().BoolVarP(&ListArgs.AllNamespaces, "all-namespaces", "A", false, "If true, list the pipelineRuns across all namespaces")

	return PipelineListCmd
}

func getExample() string {
	return `  # List kurator pipeline objects in the default namespace
  kurator pipeline list

  # List the pipelines in a specific namespace (replace 'example-namespace' with your namespace)
  kurator pipeline list -n example-namespace

  # List the pipelines across all namespaces
  kurator pipeline list -A
`
}
