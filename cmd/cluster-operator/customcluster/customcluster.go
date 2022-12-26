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

package customcluster

import (
	"context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"kurator.dev/kurator/pkg/controllers"
)

var log = ctrl.Log.WithName("custom_cluster")

func InitControllers(ctx context.Context, mgr ctrl.Manager) error {
	log.Info("~~~~~~~~~~~start init CustomClusterController ")

	resetConfig, err := rest.InClusterConfig()
	if err != nil {
		resetConfig, err = clientcmd.BuildConfigFromFlags("", os.Getenv("HOME")+"/.kube/config")
		if err != nil {
			return err
		}
	}
	ClientSet, err := kubernetes.NewForConfig(resetConfig)

	if err := (&controllers.CustomClusterController{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		ClientSet: ClientSet,
		APIReader: mgr.GetAPIReader(),
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		log.Error(err, "unable to create controller", "controller", "CustomCluster")
		return err
	}

	if mgr.GetClient() == nil {
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetClient() is nil !!!")
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetClient() is nil !!!")

	} else {
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetClient() is not nil !!!")
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetClient() is not nil !!!")

	}

	if mgr.GetAPIReader() == nil {
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetAPIReader() is nil !!!")
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetAPIReader() is nil !!!")

	} else {
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetAPIReader() is not nil !!!")
		log.Info("~~~~~~~!!!!!!!!!!!!!!!!!!  mgr.GetAPIReader() is not nil !!!")

	}

	if err := (&controllers.CustomMachineController{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		APIReader: mgr.GetAPIReader(),
	}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
		log.Error(err, "unable to create controller", "controller", "CustomMachine")
		return err
	}

	log.Info("~~~~~~~~~~~finish init CustomClusterController22222222 ")

	return nil
}
