package controller_test

import (
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	infextv1a2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kgateway-dev/kgateway/v2/pkg/deployer"
)

func TestInferencePoolController(t *testing.T) {
	t.Run("when Inference Extension deployer is enabled", func(t *testing.T) {
		t.Run("should reconcile an InferencePool referenced by a managed HTTPRoute and deploy the endpoint picker", func(t *testing.T) {
			// Setup
			var err error
			inferenceExt = new(deployer.InferenceExtInfo)
			cancel, err = createManager(ctx, inferenceExt, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}
			defer func() {
				if cancel != nil {
					cancel()
				}
				// ensure goroutines cleanup
				time.Sleep(3 * time.Second)
			}()

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
			if err != nil {
				t.Fatalf("Failed to create test gateway: %v", err)
			}

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
			if err != nil {
				t.Fatalf("Failed to create HTTPRoute: %v", err)
			}

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

			err = wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
				updateErr := k8sClient.Status().Update(ctx, httpRoute)
				return updateErr == nil, nil
			})
			if err != nil {
				t.Fatalf("Failed to update HTTPRoute status: %v", err)
			}

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
			if err != nil {
				t.Fatalf("Failed to create InferencePool: %v", err)
			}

			// The secondary watch on HTTPRoute should now trigger reconciliation of pool "pool1".
			// We expect the deployer to render and deploy an endpoint picker Deployment with name "pool1-endpoint-picker".
			expectedName := fmt.Sprintf("%s-endpoint-picker", pool.Name)
			var deploy appsv1.Deployment

			err = wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
				getErr := k8sClient.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: expectedName}, &deploy)
				return getErr == nil, nil
			})
			if err != nil {
				t.Fatalf("Expected deployment %s to be created, but it wasn't found: %v", expectedName, err)
			}
		})

		t.Run("should ignore an InferencePool not referenced by any HTTPRoute and not deploy the endpoint picker", func(t *testing.T) {
			// Setup
			var err error
			inferenceExt = new(deployer.InferenceExtInfo)
			cancel, err = createManager(ctx, inferenceExt, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}
			defer func() {
				if cancel != nil {
					cancel()
				}
				// ensure goroutines cleanup
				time.Sleep(3 * time.Second)
			}()

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
			if err != nil {
				t.Fatalf("Failed to create InferencePool: %v", err)
			}

			// Consistently check that no endpoint picker deployment is created.
			// We'll check multiple times over a 5-second period
			expectedName := fmt.Sprintf("%s-endpoint-picker", pool.Name)
			for i := 0; i < 5; i++ {
				var dep appsv1.Deployment
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: expectedName}, &dep)
				if err == nil {
					t.Fatalf("Expected deployment %s to NOT be created, but it was found", expectedName)
				}
				time.Sleep(1 * time.Second)
			}
		})
	})

	t.Run("when Inference Extension deployer is disabled", func(t *testing.T) {
		t.Run("should not deploy endpoint picker resources", func(t *testing.T) {
			// Setup
			var err error
			inferenceExt = nil
			cancel, err = createManager(ctx, inferenceExt, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}
			defer func() {
				if cancel != nil {
					cancel()
				}
				// ensure goroutines cleanup
				time.Sleep(3 * time.Second)
			}()

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
			if err := k8sClient.Create(ctx, httpRoute); err != nil {
				t.Fatalf("Failed to create HTTPRoute: %v", err)
			}

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
			if err := k8sClient.Create(ctx, pool); err != nil {
				t.Fatalf("Failed to create InferencePool: %v", err)
			}

			expectedName := fmt.Sprintf("%s-endpoint-picker", pool.Name)
			// Consistently check that no endpoint picker deployment is created.
			for i := 0; i < 5; i++ {
				var dep appsv1.Deployment
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: defaultNamespace, Name: expectedName}, &dep)
				if err == nil {
					t.Fatalf("Expected deployment %s to NOT be created when deployer is disabled, but it was found", expectedName)
				}
				time.Sleep(1 * time.Second)
			}
		})
	})
}
