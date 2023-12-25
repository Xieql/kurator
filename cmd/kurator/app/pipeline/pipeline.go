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
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"kurator.dev/kurator/pkg/generic"
	pipelinelist "kurator.dev/kurator/pkg/pipeline"
	"runtime"

	"github.com/spf13/cobra"
)

var ListArgs = pipelinelist.ListArgs{}

func NewCmd(opts *generic.Options) *cobra.Command {
	PipelineListCmd := &cobra.Command{
		Use:     "pipeline",
		Short:   "Print the info of kurator pipeline",
		Example: getExample(),
		RunE: func(cmd *cobra.Command, args []string) error {

			_ = RunVersion(cmd)

			PipelineList, err := pipelinelist.NewPipelineList(opts, &ListArgs)
			if err != nil {
				logrus.Errorf("pipeline init error: %v", err)
				return fmt.Errorf("pipeline init error: %v", err)
			}

			logrus.Debugf("start list pipeline obj, Global: %+v ", opts)
			if err := PipelineList.Execute(); err != nil {
				logrus.Errorf("pipeline execute error: %v", err)
				return fmt.Errorf("pipeline execute error: %v", err)
			}

			return nil
		},
	}

	PipelineListCmd.PersistentFlags().StringVarP(&ListArgs.Namespace, "namespace", "n", "default", "Comma separated list of namespace")

	return PipelineListCmd
}

func getExample() string {
	return `  # List kurator pipeline obj in default ns
  kurator pipeline list

  # List the pipeline in xxx ns
  kurator pipeline list -n xxx

  # List specified components Cli tool.
  kurator pipeline list istio karmada

  # List component Cli tools in JSON output format.
  kurator pipeline list -o json

  # List component Cli tools in YAML output format.
  kurator pipeline list -o yaml

  # List a single components Cli tool in JSON output format.
  kurator pipeline list istio -o json`
}

// RunVersion provides the version information of keadm in format depending on arguments
// specified in cobra.Command.
func RunVersion(cmd *cobra.Command) error {
	v := GetInfo()

	y, err := json.MarshalIndent(&v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(y))

	return nil
}

// Info contains Pipeline information.
type Info struct {
	Say       string `json:"say"`
	GoVersion string `json:"goVersion"`
	Compiler  string `json:"compiler"`
	Platform  string `json:"platform"`
}

// String returns a Go-syntax representation of the Info.
func (info Info) String() string {
	return fmt.Sprintf("%#v", info)
}

// GetInfo returns the overall Pipeline information. It's for test
func GetInfo() Info {
	return Info{
		Say:       "hello world!",
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
