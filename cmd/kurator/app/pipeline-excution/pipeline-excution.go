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

package pipeline_excution

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline-execution",
		Short: "Print the info of kurator pipeline-execution",
		Run: func(cmd *cobra.Command, args []string) {
			_ = RunVersion(cmd)
		},
	}
	return cmd
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
