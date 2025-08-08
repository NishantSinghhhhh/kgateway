package controller_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	api "sigs.k8s.io/gateway-api/apis/v1"

	. "github.com/kgateway-dev/kgateway/v2/internal/kgateway/controller"
	"github.com/kgateway-dev/kgateway/v2/pkg/metrics"
	"github.com/kgateway-dev/kgateway/v2/pkg/metrics/metricstest"
)

type TestReporter struct {
	t *testing.T
}

func (r TestReporter) Errorf(format string, args ...interface{}) {
	r.t.Errorf(format, args...)
}

func (r TestReporter) Fatalf(format string, args ...interface{}) {
	r.t.Fatalf(format, args...)
}

func (r TestReporter) FailNow() {
	r.t.FailNow()
}

// TestGwControllerMetrics tests the GwController metrics functionality
func TestGwControllerMetrics(t *testing.T) {
	t.Run("should generate gateway controller metrics", func(t *testing.T) {
		ctx, cancel := setupGwControllerMetricsTest(t)
		defer teardownGwControllerMetricsTest(t, cancel)

		setupGateway(t, ctx)
		defer deleteGateway(t, ctx)

		gathered := metricstest.MustGatherMetrics(TestReporter{t})
			
		gathered.AssertMetricsInclude("kgateway_controller_reconciliations_total", []metricstest.ExpectMetric{
			&metricstest.ExpectedMetricValueTest{
				Labels: []metrics.Label{
					{Name: "controller", Value: "gateway"},
					{Name: "namespace", Value: defaultNamespace},
					{Name: "name", Value: "gw-" + gatewayClassName + "-metrics"},
					{Name: "result", Value: "success"},
				},
				Test: metricstest.Between(1, 20),
			},
			&metricstest.ExpectedMetricValueTest{
				Labels: []metrics.Label{
					{Name: "controller", Value: "gatewayclass"},
					{Name: "namespace", Value: defaultNamespace},
					{Name: "name", Value: "gw-" + gatewayClassName + "-metrics"},
					{Name: "result", Value: "success"},
				},
				Test: metricstest.Between(1, 20),
			},
			&metricstest.ExpectedMetricValueTest{
				Labels: []metrics.Label{
					{Name: "controller", Value: "gatewayclass-provisioner"},
					{Name: "namespace", Value: defaultNamespace},
					{Name: "name", Value: "gw-" + gatewayClassName + "-metrics"},
					{Name: "result", Value: "success"},
				},
				Test: metricstest.Between(1, 10),
			},
		})

		gathered.AssertMetricsInclude("kgateway_controller_reconciliations_running", []metricstest.ExpectMetric{
			&metricstest.ExpectedMetricValueTest{
				Labels: []metrics.Label{
					{Name: "controller", Value: "gateway"},
					{Name: "name", Value: "gw-" + gatewayClassName + "-metrics"},
					{Name: "namespace", Value: defaultNamespace},
				},
				Test: metricstest.Between(0, 1),
			},
			&metricstest.ExpectedMetricValueTest{
				Labels: []metrics.Label{
					{Name: "controller", Value: "gatewayclass"},
					{Name: "name", Value: "gw-" + gatewayClassName + "-metrics"},
					{Name: "namespace", Value: defaultNamespace},
				},
				Test: metricstest.Between(0, 1),
			},
			&metricstest.ExpectedMetricValueTest{
				Labels: []metrics.Label{
					{Name: "controller", Value: "gatewayclass-provisioner"},
					{Name: "name", Value: "gw-" + gatewayClassName + "-metrics"},
					{Name: "namespace", Value: defaultNamespace},
				},
				Test: metricstest.Between(0, 1),
			},
		})

		gathered.AssertMetricsLabelsInclude("kgateway_controller_reconcile_duration_seconds", [][]metrics.Label{{
			{Name: "controller", Value: "gateway"},
			{Name: "name", Value: "gw-" + gatewayClassName + "-metrics"},
			{Name: "namespace", Value: defaultNamespace},
		}})
	})

	t.Run("when metrics are not active", func(t *testing.T) {
		testGwControllerMetricsInactive(t)
	})
}

func setupGwControllerMetricsTest(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	var err error
	managerCancel, err := createManager(t, ctx, inferenceExt, nil)
	if err != nil {
		cancel()
		t.Fatalf("Failed to create manager: %v", err)
	}

	ResetMetrics()

	// Return a combined cancel function
	combinedCancel := func() {
		if managerCancel != nil {
			managerCancel()
		}
		cancel()
	}

	return ctx, combinedCancel
}

func teardownGwControllerMetricsTest(t *testing.T, cancel context.CancelFunc) {
	t.Helper()

	if cancel != nil {
		cancel()
	}

	// ensure goroutines cleanup
	waitForCondition(t, func() bool { return true }, 3*time.Second, 100*time.Millisecond, "goroutines cleanup")
}

func testGwControllerMetricsInactive(t *testing.T) {
	var oldRegistry metrics.RegistererGatherer

	// Setup: disable metrics
	metrics.SetActive(false)
	oldRegistry = metrics.Registry()
	metrics.SetRegistry(false, metrics.NewRegistry())

	// Cleanup: restore metrics
	defer func() {
		metrics.SetActive(true)
		metrics.SetRegistry(false, oldRegistry)
	}()

	ctx, cancel := setupGwControllerMetricsTest(t)
	defer teardownGwControllerMetricsTest(t, cancel)

	t.Run("should not record metrics if metrics are not active", func(t *testing.T) {
		setupGateway(t, ctx)
		defer deleteGateway(t, ctx)

		gathered := metricstest.MustGatherMetrics(TestReporter{t})

		gathered.AssertMetricNotExists("kgateway_controller_reconciliations_total")
		gathered.AssertMetricNotExists("kgateway_controller_reconciliations_running")
		gathered.AssertMetricNotExists("kgateway_controller_reconcile_duration_seconds")
	})
}

func gateway() *api.Gateway {
	same := api.NamespacesFromSame
	gwName := "gw-" + gatewayClassName + "-metrics"
	gw := api.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwName,
			Namespace: defaultNamespace,
		},
		Spec: api.GatewaySpec{
			GatewayClassName: api.ObjectName(gatewayClassName),
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

	return &gw
}

func deleteGateway(t *testing.T, ctx context.Context) {
	t.Helper()

	gw := gateway()
	err := k8sClient.Delete(ctx, gw)
	if err != nil {
		t.Fatalf("Failed to delete gateway: %v", err)
	}

	// The tests in this suite don't do a good job of cleaning up after themselves, which is relevant because of the shared envtest environment
	// but we can at least that the gateway from this test is deleted
	waitForCondition(t, func() bool {
		var createdGateways api.GatewayList
		err := k8sClient.List(ctx, &createdGateways)
		found := false
		for _, foundGw := range createdGateways.Items {
			if foundGw.Name == gw.Name {
				found = true
				break
			}
		}
		return err == nil && !found
	}, timeout, interval, "gateway not deleted")
}

func setupGateway(t *testing.T, ctx context.Context) {
	t.Helper()

	gw := gateway()
	err := k8sClient.Create(ctx, gw)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	waitForGatewayService(t, ctx, gw)

	if probs, err := metricstest.GatherAndLint(); err != nil || len(probs) > 0 {
		t.Fatalf("metrics linter error: %v", err)
	}
}

func waitForGatewayService(t *testing.T, ctx context.Context, gw *api.Gateway) corev1.Service {
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
		t.Fatalf("Expected service ClusterIP to not be empty")
	}

	return svc
}
