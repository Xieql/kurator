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

package plugin

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/fluxcd/pkg/runtime/transform"
	sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	fleetv1a1 "kurator.dev/kurator/pkg/apis/fleet/v1alpha1"
)

const (
	MetricPluginName  = "metric"
	GrafanaPluginName = "grafana"
	KyvernoPluginName = "kyverno"
	BackupPluginName  = "backup"

	ThanosComponentName        = "thanos"
	PrometheusComponentName    = "prometheus"
	GrafanaComponentName       = "grafana"
	KyvernoComponentName       = "kyverno"
	KyvernoPolicyComponentName = "kyverno-policies"
	VeleroComponentName        = "velero"

	OCIReposiotryPrefix = "oci://"
)

type GrafanaDataSource struct {
	Name       string `json:"name"`
	SourceType string `json:"type"`
	URL        string `json:"url"`
	Access     string `json:"access"`
	IsDefault  bool   `json:"isDefault"`
}

func RenderKyvernoPolicy(fsys fs.FS, fleetNN types.NamespacedName, fleetRef *metav1.OwnerReference, cluster FleetCluster, kyvernoCfg *fleetv1a1.KyvernoConfig) ([]byte, error) {
	c, err := getFleetPluginChart(fsys, KyvernoPolicyComponentName)
	if err != nil {
		return nil, err
	}

	mergeChartConfig(c, kyvernoCfg.Chart)

	values := map[string]interface{}{
		"podSecurityStandard":     kyvernoCfg.PodSecurity.Standard,
		"podSecuritySeverity":     kyvernoCfg.PodSecurity.Severity,
		"validationFailureAction": kyvernoCfg.PodSecurity.ValidationFailureAction,
	}

	return renderFleetPlugin(fsys, FleetPluginConfig{
		Name:           KyvernoPluginName,
		Component:      KyvernoPolicyComponentName,
		Fleet:          fleetNN,
		Cluster:        &cluster,
		OwnerReference: fleetRef,
		Chart:          *c,
		Values:         values,
	})
}

func RenderKyverno(fsys fs.FS, fleetNN types.NamespacedName, fleetRef *metav1.OwnerReference, cluster FleetCluster, kyvernoCfg *fleetv1a1.KyvernoConfig) ([]byte, error) {
	c, err := getFleetPluginChart(fsys, KyvernoComponentName)
	if err != nil {
		return nil, err
	}

	mergeChartConfig(c, kyvernoCfg.Chart)

	values, err := toMap(kyvernoCfg.ExtraArgs)
	if err != nil {
		return nil, err
	}

	return renderFleetPlugin(fsys, FleetPluginConfig{
		Name:           KyvernoPluginName,
		Component:      KyvernoComponentName,
		Fleet:          fleetNN,
		Cluster:        &cluster,
		OwnerReference: fleetRef,
		Chart:          *c,
		Values:         values,
	})
}

func RenderGrafana(fsys fs.FS, fleetNN types.NamespacedName, fleetRef *metav1.OwnerReference,
	grafanaCfg *fleetv1a1.GrafanaConfig, datasources []*GrafanaDataSource) ([]byte, error) {
	c, err := getFleetPluginChart(fsys, GrafanaComponentName)
	if err != nil {
		return nil, err
	}

	mergeChartConfig(c, grafanaCfg.Chart)
	c.TargetNamespace = fleetNN.Namespace // grafana chart is fleet scoped

	values, err := toMap(grafanaCfg.ExtraArgs)
	if err != nil {
		return nil, err
	}

	if len(datasources) != 0 {
		values = transform.MergeMaps(values, map[string]interface{}{
			"datasources": map[string]interface{}{
				"secretDefinition": map[string]interface{}{
					"apiVersion":  1,
					"datasources": datasources,
				},
			},
		})
	}

	grafanaPluginCfg := FleetPluginConfig{
		Name:           GrafanaPluginName,
		Component:      GrafanaComponentName,
		Fleet:          fleetNN,
		OwnerReference: fleetRef,
		Chart:          *c,
		Values:         values,
	}

	return renderFleetPlugin(fsys, grafanaPluginCfg)
}

func RenderThanos(fsys fs.FS, fleetNN types.NamespacedName, fleetRef *metav1.OwnerReference, metricCfg *fleetv1a1.MetricConfig) ([]byte, error) {
	thanosChart, err := getFleetPluginChart(fsys, ThanosComponentName)
	if err != nil {
		return nil, err
	}

	mergeChartConfig(thanosChart, metricCfg.Thanos.Chart)
	thanosChart.TargetNamespace = fleetNN.Namespace // thanos chart is fleet scoped

	values, err := toMap(metricCfg.Thanos.ExtraArgs)
	if err != nil {
		return nil, err
	}

	values = transform.MergeMaps(values, map[string]interface{}{
		"existingObjstoreSecret": metricCfg.Thanos.ObjectStoreConfig.SecretName, // always use secret from API
		"query": map[string]interface{}{
			"dnsDiscovery": map[string]interface{}{
				"sidecarsNamespace": fleetNN.Namespace, // override default namespace
			},
		},
	})

	thanosCfg := FleetPluginConfig{
		Name:           MetricPluginName,
		Component:      ThanosComponentName,
		Fleet:          fleetNN,
		OwnerReference: fleetRef,
		Chart:          *thanosChart,
		Values:         values,
	}

	return renderFleetPlugin(fsys, thanosCfg)
}

func RenderPrometheus(fsys fs.FS, fleetName types.NamespacedName, fleetRef *metav1.OwnerReference, cluster FleetCluster, metricCfg *fleetv1a1.MetricConfig) ([]byte, error) {
	promChart, err := getFleetPluginChart(fsys, PrometheusComponentName)
	if err != nil {
		return nil, err
	}

	mergeChartConfig(promChart, metricCfg.Prometheus.Chart)

	values, err := toMap(metricCfg.Prometheus.ExtraArgs)
	if err != nil {
		return nil, err
	}

	values = transform.MergeMaps(values, map[string]interface{}{
		"prometheus": map[string]interface{}{
			"externalLabels": map[string]interface{}{
				"cluster": cluster.Name,
			},
			"thanos": map[string]interface{}{
				"objectStorageConfig": map[string]interface{}{
					"secretName": metricCfg.Thanos.ObjectStoreConfig.SecretName,
				},
			},
		},
	})

	promCfg := FleetPluginConfig{
		Name:           MetricPluginName,
		Component:      PrometheusComponentName,
		Fleet:          fleetName,
		OwnerReference: fleetRef,
		Cluster:        &cluster,
		Chart:          *promChart,
		Values:         values,
	}

	return renderFleetPlugin(fsys, promCfg)
}

type veleroObjectStoreLocation struct {
	Bucket   string                 `json:"bucket"`
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

func RenderVelero(
	fsys fs.FS,
	fleetNN types.NamespacedName,
	fleetRef *metav1.OwnerReference,
	cluster FleetCluster,
	backupCfg *fleetv1a1.BackupConfig,
	veleroSecretName string,
) ([]byte, error) {
	// get and merge the chart config
	c, err := getFleetPluginChart(fsys, VeleroComponentName)
	if err != nil {
		return nil, err
	}
	mergeChartConfig(c, backupCfg.Chart)

	// get default values
	defaultValues := c.Values
	// providerValues is a map that stores default configurations associated with the specific provider. These configurations are necessary for the proper functioning of the Velero tool with the provider. Currently, this includes configurations for initContainers.
	providerValues, err := getProviderValues(backupCfg.Storage.Location.Provider)
	if err != nil {
		return nil, err
	}
	// add providerValues to default values
	defaultValues = transform.MergeMaps(defaultValues, providerValues)

	// get custom values
	customValues := map[string]interface{}{}
	locationConfig := stringMapToInterfaceMap(backupCfg.Storage.Location.Config)
	// generate velero config. "backupCfg.Storage.Location.Endpoint" and "backupCfg.Storage.Location.Region" will overwrite the value of "backupCfg.Storage.Location.config"
	// because "backupCfg.Storage.Location.config" is optional, it should take effect only when current setting is not enough.
	Config := transform.MergeMaps(locationConfig, map[string]interface{}{
		"s3Url":            backupCfg.Storage.Location.Endpoint,
		"region":           backupCfg.Storage.Location.Region,
		"s3ForcePathStyle": true,
	})
	provider := getProviderFrombackupCfg(backupCfg)
	configurationValues := map[string]interface{}{
		"configuration": map[string]interface{}{
			"backupStorageLocation": []veleroObjectStoreLocation{
				{
					Bucket:   backupCfg.Storage.Location.Bucket,
					Provider: provider,
					Config:   Config,
				},
			},
		},
		"credentials": map[string]interface{}{
			"useSecret":      true,
			"existingSecret": veleroSecretName,
		},
	}
	// add custom configurationValues to customValues
	customValues = transform.MergeMaps(customValues, configurationValues)
	extraValues, err := toMap(backupCfg.ExtraArgs)
	if err != nil {
		return nil, err
	}
	// add custom extraValues to customValues
	customValues = transform.MergeMaps(customValues, extraValues)

	// replace the default values with custom values to obtain the actual values.
	values := transform.MergeMaps(defaultValues, customValues)

	return renderFleetPlugin(fsys, FleetPluginConfig{
		Name:           BackupPluginName,
		Component:      VeleroComponentName,
		Fleet:          fleetNN,
		Cluster:        &cluster,
		OwnerReference: fleetRef,
		Chart:          *c,
		Values:         values,
	})
}

func mergeChartConfig(origin *ChartConfig, target *fleetv1a1.ChartConfig) {
	if target == nil {
		return
	}

	origin.Name = target.Name
	origin.Repo = target.Repository
	origin.Version = target.Version
	if target.Repository != "" &&
		strings.HasPrefix(target.Repository, OCIReposiotryPrefix) {
		origin.Type = sourcev1b2.HelmRepositoryTypeOCI
	} else {
		origin.Type = sourcev1b2.HelmRepositoryTypeDefault
	}
}

func toMap(args apiextensionsv1.JSON) (map[string]interface{}, error) {
	if args.Raw == nil {
		return nil, nil
	}

	var m map[string]interface{}
	err := json.Unmarshal(args.Raw, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func stringMapToInterfaceMap(args map[string]string) map[string]interface{} {
	m := make(map[string]interface{})
	for s, s2 := range args {
		m[s] = s2
	}

	return m
}

// getProviderValues return the map that stores default configurations associated with the specific provider.
// The provider parameter can be one of the following values: "aws", "huaweicloud", "gcp", "azure".
func getProviderValues(provider string) (map[string]interface{}, error) {
	switch provider {
	case "aws":
		return buildAWSProviderValues(), nil
	case "huaweicloud":
		return buildHuaWeiCloudProviderValues(), nil
	case "gcp":
		return buildGCPProviderValues(), nil
	case "azure":
		return buildAzureProviderValues(), nil
	default:
		return nil, fmt.Errorf("unknown objStoreProvider: %v", provider)
	}
}

// buildAWSProviderValues constructs the default provider values for AWS.
func buildAWSProviderValues() map[string]interface{} {
	values := map[string]interface{}{}

	// currently, the default provider-related extra configuration only sets up initContainers
	initContainersConfig := map[string]interface{}{
		"initContainers": []interface{}{
			map[string]interface{}{
				"image": "velero/velero-plugin-for-aws:v1.7.1",
				"name":  "velero-plugin-for-aws",
				"volumeMounts": []interface{}{
					map[string]interface{}{
						"mountPath": "/target",
						"name":      "plugins",
					},
				},
			},
		},
	}
	values = transform.MergeMaps(values, initContainersConfig)

	return values
}

func buildHuaWeiCloudProviderValues() map[string]interface{} {
	return buildAWSProviderValues()
}

// TODO： accomplish those function after investigation
func buildGCPProviderValues() map[string]interface{} {
	return nil
}
func buildAzureProviderValues() map[string]interface{} {
	return nil
}

func getProviderFrombackupCfg(backupCfg *fleetv1a1.BackupConfig) string {
	provider := backupCfg.Storage.Location.Provider
	// there no "huaweicloud" provider in velero
	if provider == "huaweicloud" {
		provider = "aws"
	}
	return provider
}
