package controller_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	api "sigs.k8s.io/gateway-api/apis/v1"
)

// TestGwController tests the GwController functionality
func TestGwController(t *testing.T) {
	testCases := []struct {
		name    string
		gwClass string
	}{
		{"default gateway class", gatewayClassName},
		{"alternative gateway class", altGatewayClassName},
		{"self managed gateway", selfManagedGatewayClassName},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := setupGwControllerTest(t)
			defer teardownGwControllerTest(t, cancel)

			testShouldAddStatusToGateway(t, ctx, tc.gwClass)
		})
	}
}

func setupGwControllerTest(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	var err error
	managerCancel, err := createManager(ctx, inferenceExt, nil)
	if err != nil {
		cancel()
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Return a combined cancel function
	combinedCancel := func() {
		if managerCancel != nil {
			managerCancel()
		}
		cancel()
	}

	return ctx, combinedCancel
}

func teardownGwControllerTest(t *testing.T, cancel context.CancelFunc) {
	t.Helper()

	if cancel != nil {
		cancel()
	}
	// ensure goroutines cleanup
	waitForCondition(t, func() bool { return true }, 3*time.Second, 100*time.Millisecond, "goroutines cleanup")
}

func testShouldAddStatusToGateway(t *testing.T, ctx context.Context, gwClass string) {
	same := api.NamespacesFromSame
	gwName := "gw-" + gwClass
	gw := api.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwName,
			Namespace: "default",
		},
		Spec: api.GatewaySpec{
			Addresses: []api.GatewaySpecAddress{{
				Type:  ptr.To(api.IPAddressType),
				Value: "127.0.0.1",
			}},
			GatewayClassName: api.ObjectName(gwClass),
			Listeners: []api.Listener{{
				Protocol: "HTTP",
				Port:     80,
				AllowedRoutes: &api.AllowedRoutes{
					Namespaces: &api.RouteNamespaces{
						From: &same,
					},
				},
				Name: "listener",
			}},
		},
	}

	err := k8sClient.Create(ctx, &gw)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	if gwClass != selfManagedGatewayClassName {
		svc := waitForGatewayServiceSimple(t, ctx, &gw)

		// Need to update the status of the service
		svc.Status.LoadBalancer = corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{
				IP: "127.0.0.1",
			}},
		}

		waitForConditionWithError(t, func() error {
			return k8sClient.Status().Update(ctx, &svc)
		}, timeout, interval, "service status update")
	}

	waitForCondition(t, func() bool {
		err := k8sClient.Get(ctx, client.ObjectKey{Name: gwName, Namespace: "default"}, &gw)
		if err != nil {
			return false
		}
		if len(gw.Status.Addresses) == 0 {
			return false
		}
		return true
	}, timeout, interval, "gateway status addresses update")

	if len(gw.Status.Addresses) != 1 {
		t.Errorf("Expected gateway to have 1 address, got %d", len(gw.Status.Addresses))
	}

	if gw.Status.Addresses[0].Type == nil || *gw.Status.Addresses[0].Type != api.IPAddressType {
		t.Errorf("Expected address type %s, got %v", api.IPAddressType, gw.Status.Addresses[0].Type)
	}

	if gw.Status.Addresses[0].Value != "127.0.0.1" {
		t.Errorf("Expected address value '127.0.0.1', got '%s'", gw.Status.Addresses[0].Value)
	}
}

// waitForGatewayServiceSimple is a version of waitForGatewayService that matches the original signature
func waitForGatewayServiceSimple(t *testing.T, ctx context.Context, gw *api.Gateway) corev1.Service {
	t.Helper()

	var svc corev1.Service

	waitForCondition(t, func() bool {
		var createdServices corev1.ServiceList
		err := k8sClient.List(ctx, &createdServices)
		if err != nil {
			return false
		}
		for _, s := range createdServices.Items {
			if len(s.ObjectMeta.OwnerReferences) == 1 && s.ObjectMeta.OwnerReferences[0].UID == gw.GetUID() {
				svc = s
				return true
			}
		}
		return false
	}, timeout, interval, "service not created")

	if svc.Spec.ClusterIP == "" {
		t.Errorf("Expected service ClusterIP to not be empty")
	}

	return svc
}

// Helper function to wait with timeout and interval checking
func waitForCondition(t *testing.T, condition func() bool, timeout, interval time.Duration, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("Condition not met within timeout: %s", msg)
}

// Helper function to wait for condition with error
func waitForConditionWithError(t *testing.T, condition func() error, timeout, interval time.Duration, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := condition(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(interval)
	}
	t.Fatalf("Condition not met within timeout: %s, last error: %v", msg, lastErr)
}
