package controller_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	infextv1a2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kgateway-dev/kgateway/v2/pkg/deployer"
	"github.com/kgateway-dev/kgateway/v2/test/gomega/assertions"
)

func TestInferencePoolController(t *testing.T) {
	ctx := context.Background()
	
	t.Run("when Inference Extension deployer is enabled", func(t *testing.T) {
		t.Run("should reconcile an InferencePool referenced by a managed HTTPRoute and deploy the endpoint picker", func(t *testing.T) {
			goroutineMonitor := assertions.NewGoRoutineMonitor()
			var cancel context.CancelFunc
			defer func() {
				if cancel != nil {
					cancel()
				}
				waitForGoroutinesToFinish(goroutineMonitor)
			}()

			inferenceExt := new(deployer.InferenceExtInfo)
			var err error
			cancel, err = createManager(ctx, inferenceExt, nil)
			require.NoError(t, err)

			// Create a test Gateway that will be referenced by the HTTPRoute.
			testGw := &apiv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: defaultNamespace,
				},
				Spec: apiv1.GatewaySpec{
					GatewayClassName: gatewayClassName,
					Listeners: []apiv1.Listener{
						{
							Name:     "listener-1",
							Protocol: apiv1.HTTPProtocolType,
							Port:     80,
						},
					},
				},
			}
			err = k8sClient.Create(ctx, testGw)
			assert.NoError(t, err)

			// Create an HTTPRoute without a status.
			httpRoute := &apiv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: defaultNamespace,
				},
				Spec: apiv1.HTTPRouteSpec{
					Rules: []apiv1.HTTPRouteRule{
						{
							BackendRefs: []apiv1.HTTPBackendRef{
								{
									BackendRef: apiv1.BackendRef{
										BackendObjectReference: apiv1.BackendObjectReference{
											Group: ptr.To(apiv1.Group(infextv1a2.GroupVersion.Group)),
											Kind:  ptr.To(apiv1.Kind("InferencePool")),
											Name:  "pool1",
										},
									},
								},
							},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, httpRoute)
			assert.NoError(t, err)

			// Now update the status to include a valid Parents field.
			httpRoute.Status = apiv1.HTTPRouteStatus{
				RouteStatus: apiv1.RouteStatus{
					Parents: []apiv1.RouteParentStatus{
						{
							ParentRef: apiv1.ParentReference{
								Group:     ptr.To(apiv1.Group("gateway.networking.k8s.io")),
								Kind:      ptr.To(apiv1.Kind("Gateway")),
								Name:      apiv1.ObjectName(testGw.Name),
								Namespace: ptr.To(apiv1.Namespace(defaultNamespace)),
							},
							ControllerName: gatewayControllerName,
						},
					},
				},
			}

			// Retry updating status until successful
			err = retryWithTimeout(10*time.Second, 1*time.Second, func() error {
				return k8sClient.Status().Update(ctx, httpRoute)
			})
			assert.NoError(t, err)

			// Create an InferencePool resource that is referenced by the HTTPRoute.
			pool := &infextv1a2.InferencePool{
				TypeMeta: metav1.TypeMeta{
					Kind:       "InferencePool",
					APIVersion: infextv1a2.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pool1",
					Namespace: defaultNamespace,
					UID:       "pool-uid",
				},
				Spec: infextv1a2.InferencePoolSpec{
					Selector:         map[infextv1a2.LabelKey]infextv1a2.LabelValue{},
					TargetPortNumber: 1234,
					EndpointPickerConfig: infextv1a2.EndpointPickerConfig{
						ExtensionRef: &infextv1a2.Extension{
							ExtensionReference: infextv1a2.ExtensionReference{
								Name: "doesnt-matter",
							},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, pool)
			assert.NoError(t, err)

			// The secondary watch on HTTPRoute should now trigger reconciliation of pool "pool1".
			// We expect the deployer to render and deploy an endpoint picker Deployment with name "pool1-endpoint-picker".
			expectedName := fmt.Sprintf("%s-endpoint-picker", pool.Name)
			var deploy appsv1.Deployment
			
			err = retryWithTimeout(10*time.Second, 1*time.Second, func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: expectedName}, &deploy)
			})
			assert.NoError(t, err, "Expected deployment %s to be created", expectedName)
		})

		t.Run("should ignore an InferencePool not referenced by any HTTPRoute and not deploy the endpoint picker", func(t *testing.T) {
			goroutineMonitor := assertions.NewGoRoutineMonitor()
			var cancel context.CancelFunc
			defer func() {
				if cancel != nil {
					cancel()
				}
				waitForGoroutinesToFinish(goroutineMonitor)
			}()

			inferenceExt := new(deployer.InferenceExtInfo)
			var err error
			cancel, err = createManager(ctx, inferenceExt, nil)
			require.NoError(t, err)

			// Create an InferencePool that is not referenced by any HTTPRoute.
			pool := &infextv1a2.InferencePool{
				TypeMeta: metav1.TypeMeta{
					Kind:       "InferencePool",
					APIVersion: infextv1a2.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pool2",
					Namespace: defaultNamespace,
					UID:       "pool2-uid",
				},
				Spec: infextv1a2.InferencePoolSpec{
					Selector:         map[infextv1a2.LabelKey]infextv1a2.LabelValue{},
					TargetPortNumber: 1234,
					EndpointPickerConfig: infextv1a2.EndpointPickerConfig{
						ExtensionRef: &infextv1a2.Extension{
							ExtensionReference: infextv1a2.ExtensionReference{
								Name: "doesnt-matter",
							},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, pool)
			assert.NoError(t, err)

			// Consistently check that no endpoint picker deployment is created.
			expectedName := fmt.Sprintf("%s-endpoint-picker", pool.Name)
			
			// Check multiple times over 5 seconds that deployment is NOT created
			for i := 0; i < 5; i++ {
				var dep appsv1.Deployment
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: expectedName}, &dep)
				assert.Error(t, err, "Deployment %s should not exist", expectedName)
				time.Sleep(1 * time.Second)
			}
		})
	})

	t.Run("when Inference Extension deployer is disabled", func(t *testing.T) {
		t.Run("should not deploy endpoint picker resources", func(t *testing.T) {
			goroutineMonitor := assertions.NewGoRoutineMonitor()
			var cancel context.CancelFunc
			defer func() {
				if cancel != nil {
					cancel()
				}
				waitForGoroutinesToFinish(goroutineMonitor)
			}()

			var err error
			cancel, err = createManager(ctx, nil, nil)
			require.NoError(t, err)

			httpRoute := &apiv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route-disabled",
					Namespace: defaultNamespace,
				},
				Spec: apiv1.HTTPRouteSpec{
					Rules: []apiv1.HTTPRouteRule{{
						BackendRefs: []apiv1.HTTPBackendRef{{
							BackendRef: apiv1.BackendRef{
								BackendObjectReference: apiv1.BackendObjectReference{
									Group: ptr.To(apiv1.Group(infextv1a2.GroupVersion.Group)),
									Kind:  ptr.To(apiv1.Kind("InferencePool")),
									Name:  "pool-disabled",
								},
							},
						}},
					}},
				},
			}
			err = k8sClient.Create(ctx, httpRoute)
			assert.NoError(t, err)

			pool := &infextv1a2.InferencePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pool-disabled",
					Namespace: defaultNamespace,
				},
				Spec: infextv1a2.InferencePoolSpec{
					Selector:         map[infextv1a2.LabelKey]infextv1a2.LabelValue{},
					TargetPortNumber: 1234,
					EndpointPickerConfig: infextv1a2.EndpointPickerConfig{
						ExtensionRef: &infextv1a2.Extension{
							ExtensionReference: infextv1a2.ExtensionReference{Name: "doesnt-matter"},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, pool)
			assert.NoError(t, err)

			expectedName := fmt.Sprintf("%s-endpoint-picker", pool.Name)
			
			// Check multiple times over 5 seconds that deployment is NOT created
			for i := 0; i < 5; i++ {
				var dep appsv1.Deployment
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: expectedName}, &dep)
				assert.Error(t, err, "Deployment %s should not exist when deployer is disabled", expectedName)
				time.Sleep(1 * time.Second)
			}
		})
	})
}

// Helper function to retry operations with timeout
func retryWithTimeout(timeout, interval time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := fn(); err == nil {
			return nil
		}
		time.Sleep(interval)
	}
	return fn() // Final attempt
}