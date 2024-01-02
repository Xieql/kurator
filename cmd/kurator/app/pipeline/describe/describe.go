package describe

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"kurator.dev/kurator/pkg/generic"
	pipelinelist "kurator.dev/kurator/pkg/pipeline"
)

var ListArgs = pipelinelist.ListArgs{}

func NewCmd(opts *generic.Options) *cobra.Command {
	PipelineListCmd := &cobra.Command{
		Use:     "describe",
		Short:   "describe the info of kurator pipeline",
		Example: getExample(),
		RunE: func(cmd *cobra.Command, args []string) error {

			PipelineList, err := pipelinelist.NewPipelineList(opts, &ListArgs)
			if err != nil {
				logrus.Errorf("pipeline init error: %v", err)
				return fmt.Errorf("pipeline init error: %v", err)
			}

			logrus.Debugf("start list pipeline obj, Global: %+v ", opts)
			if err := PipelineList.DescribeExecute(); err != nil {
				logrus.Errorf("pipeline describe execute error: %v", err)
				return fmt.Errorf("pipeline execute error: %v", err)
			}

			return nil
		},
	}

	return PipelineListCmd
}

func getExample() string {
	return `  # describe kurator pipeline object info 
  kurator pipeline describe {}

`
}
