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

package fleet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	backupapi "kurator.dev/kurator/pkg/apis/backups/v1alpha1"
	fleetapi "kurator.dev/kurator/pkg/apis/fleet/v1alpha1"
)

const (
	testFleetName   = "test-fleet"
	testNamespace   = "default"
	testRestoreName = "test-restore"
)

func setupTest(t *testing.T) *RestoreManager {
	scheme := runtime.NewScheme()

	if err := backupapi.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add backupapi to scheme: %v", err)
	}
	if err := fleetapi.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add fleetapi to scheme: %v", err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	mgr := &RestoreManager{Client: client, Scheme: scheme}

	return mgr
}

// createTestReconcileRequest creates a test Reconcile request for the given Restore object.
func createTestReconcileRequest(restore *backupapi.Restore) reconcile.Request {
	if restore == nil {
		return reconcile.Request{}
	}
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      restore.Name,
			Namespace: restore.Namespace,
		},
	}
}

// createTestRestore creates a test Restore for the given Restore name and namespace.
func createTestRestore(name, namespace string) *backupapi.Restore {
	return &backupapi.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createTestFleet(name, namespace string) *fleetapi.Fleet {
	return &fleetapi.Fleet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestReconcileRestore(t *testing.T) {
	tests := []struct {
		name       string
		restore    *backupapi.Restore
		wantResult ctrl.Result
		wantErr    bool
	}{
		{
			name:       "Restore object not found",
			restore:    nil, // We can simulate a not found error by passing a nil Restore
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name:       "Restore without finalizer",
			restore:    createTestRestore(testRestoreName, testNamespace),
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "Restore with deletion timestamp",
			restore: func() *backupapi.Restore {
				r := createTestRestore(testRestoreName, testNamespace)
				now := metav1.Now()
				r.DeletionTimestamp = &now
				return r
			}(),
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTest(t)

			// Only create if the Restore object is not nil
			if tt.restore != nil {
				if err := mgr.Client.Create(context.Background(), tt.restore); err != nil {
					t.Fatalf("Failed to create test restore: %v", err)
				}
			}

			ctx := context.TODO()
			req := createTestReconcileRequest(tt.restore)

			gotResult, gotErr := mgr.Reconcile(ctx, req)
			assert.Equal(t, tt.wantResult, gotResult)
			if tt.wantErr {
				assert.NotNil(t, gotErr)
			} else {
				assert.Nil(t, gotErr)
			}
		})
	}
}

func TestReconcileDeleteRestore(t *testing.T) {
	tests := []struct {
		name          string
		restore       *backupapi.Restore
		wantErr       bool
		wantFinalizer bool
	}{
		{
			name: "Successful deletion",
			restore: func() *backupapi.Restore {
				r := createTestRestore(testRestoreName, testNamespace)
				controllerutil.AddFinalizer(r, RestoreFinalizer)
				return r
			}(),
			wantErr:       false,
			wantFinalizer: false,
		},
		{
			name: "Failed deletion due to fetch error",
			restore: func() *backupapi.Restore {
				r := createTestRestore("non-existent", "non-existent")
				controllerutil.AddFinalizer(r, RestoreFinalizer)
				return r
			}(),
			wantErr:       true,
			wantFinalizer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTest(t)
			fleetObj := createTestFleet(testFleetName, testNamespace)
			if err := mgr.Client.Create(context.Background(), fleetObj); err != nil {
				t.Fatalf("Failed to create test fleet: %v", err)
			}

			if err := mgr.Client.Create(context.Background(), tt.restore); err != nil {
				t.Fatalf("Failed to create test restore: %v", err)
			}

			_, err := mgr.reconcileDeleteRestore(context.TODO(), tt.restore)

			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			if tt.wantFinalizer {
				assert.Contains(t, tt.restore.Finalizers, RestoreFinalizer)
			} else {
				assert.NotContains(t, tt.restore.Finalizers, RestoreFinalizer)
			}
		})
	}
}
