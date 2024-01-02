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
	pipelinelist "kurator.dev/kurator/pkg/pipeline/excution"
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

	return PipelineListCmd
}

// TODO ：了解 这种 写法，是不是{}   kurator pipeline list -n {namespace}
func getExample() string {
	return `  # List kurator pipeline obj in default ns
  kurator pipeline list

  # List the pipeline in xxx ns
  kurator pipeline list -n {namespace}

  # List the pipeline in all ns
  kurator pipeline list -A
`
}
