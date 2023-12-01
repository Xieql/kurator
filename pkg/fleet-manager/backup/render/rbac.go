package render

import (
	"fmt"
	"io/fs"
)

const (
	ServiceAccountNamePrefix     = "kurator-pipeline-robot-"
	RoleBindingNameSuffix        = "-binding"
	ClusterRoleBindingNameSuffix = "-clusterbinding"
	// RBACTemplateFileName is the name of the RBAC template file.
	RBACTemplateFileName = "rbac.tpl"
	RBACTemplateName     = "pipeline rbac template"
)

// RBACConfig contains the configuration data required for the RBAC template.
// Both PipelineName and PipelineNamespace are required.
type RBACConfig struct {
	PipelineName      string // Name of the pipeline.
	PipelineNamespace string // Kubernetes namespace where the pipeline is deployed.
}

// ServiceAccountName generates the service account name using the pipeline name and namespace.
func (rbac RBACConfig) ServiceAccountName() string {
	return ServiceAccountNamePrefix + rbac.PipelineName + "-" + rbac.PipelineNamespace
}

// RoleBindingName generates the role binding name using the service account name.
func (rbac RBACConfig) RoleBindingName() string {
	return rbac.ServiceAccountName() + RoleBindingNameSuffix
}

// ClusterRoleBindingName generates the cluster role binding name using the service account name.
func (rbac RBACConfig) ClusterRoleBindingName() string {
	return rbac.ServiceAccountName() + ClusterRoleBindingNameSuffix
}

// renderRBAC renders the RBAC configuration using a specified template.
func renderRBAC(fsys fs.FS, cfg RBACConfig) ([]byte, error) {
	if cfg.PipelineName == "" || cfg.PipelineNamespace == "" {
		return nil, fmt.Errorf("invalid RBACConfig: PipelineName and PipelineNamespace must not be empty")
	}
	return renderPipelineTemplate(fsys, RBACTemplateFileName, RBACTemplateName, cfg)
}
