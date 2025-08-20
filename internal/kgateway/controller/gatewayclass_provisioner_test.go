package controller_test

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/controller"
	"github.com/kgateway-dev/kgateway/v2/test/gomega/assertions"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

func TestGatewayClassProvisioner(t *testing.T) {
	t.Run("no GatewayClasses exist on the cluster", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			if cancel != nil {
				cancel()
			}
			// ensure goroutines cleanup
			time.Sleep(3 * time.Second)
		}()

		managerCancel, err := createManager(t, ctx, nil, nil)
		assertNoError(t, err)
		defer managerCancel()

		// should create the default GCs
		assertEventually(t, func() bool {
			gcs := &apiv1.GatewayClassList{}
			err := k8sClient.List(ctx, gcs)
			if err != nil {
				return false
			}
			if len(gcs.Items) != gwClasses.Len() {
				return false
			}
			for _, gc := range gcs.Items {
				if !gwClasses.Has(gc.Name) {
					return false
				}
			}
			return true
		}, timeout, interval, "expected default GatewayClasses to be created")
	})

	t.Run("existing GatewayClasses from other controllers exist on the cluster", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			if cancel != nil {
				cancel()
			}
			// ensure goroutines cleanup
			time.Sleep(3 * time.Second)
		}()

		// Create GatewayClass owned by another controller
		otherGC := &apiv1.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-controller",
			},
			Spec: apiv1.GatewayClassSpec{
				ControllerName: "other.controller/name",
			},
		}
		assertNoError(t, k8sClient.Create(ctx, otherGC))
		defer func() {
			k8sClient.Delete(ctx, otherGC)
		}()

		// Create our GatewayClass but with wrong controller
		wrongControllerGC := &apiv1.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "wrong-controller",
			},
			Spec: apiv1.GatewayClassSpec{
				ControllerName: "wrong.controller/name",
			},
		}
		assertNoError(t, k8sClient.Create(ctx, wrongControllerGC))
		defer func() {
			k8sClient.Delete(ctx, wrongControllerGC)
		}()

		managerCancel, err := createManager(t, ctx, nil, nil)
		assertNoError(t, err)
		defer managerCancel()

		// should create our GCs and not affect others
		// verifying our GatewayClasses are created with correct controller
		assertEventually(t, func() bool {
			for className := range gwClasses {
				gc := &apiv1.GatewayClass{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: className}, gc); err != nil {
					return false
				}
				if gc.Spec.ControllerName != apiv1.GatewayController(gatewayControllerName) {
					return false
				}
			}
			return true
		}, timeout, interval, "expected our GatewayClasses to be created with correct controller")
	})

	t.Run("the default GCs are deleted", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			if cancel != nil {
				cancel()
			}
			// ensure goroutines cleanup
			time.Sleep(3 * time.Second)
		}()

		managerCancel, err := createManager(t, ctx, nil, nil)
		assertNoError(t, err)
		defer managerCancel()

		// wait for the default GCs to be created, especially needed if this is the first test to run
		gc := &apiv1.GatewayClass{}
		assertEventually(t, func() bool {
			return k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc) == nil
		}, timeout, interval, "expected default GatewayClass to be created initially")

		// deleting the default GCs
		for name := range gwClasses {
			err := k8sClient.Delete(ctx, &apiv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: name}})
			assertNoError(t, err)
		}

		// waiting for the GCs to be recreated - should be recreated by the provisioner
		assertEventually(t, func() bool {
			gcs := &apiv1.GatewayClassList{}
			err := k8sClient.List(ctx, gcs)
			if err != nil {
				return false
			}
			return len(gcs.Items) == gwClasses.Len()
		}, timeout, interval, "expected GatewayClasses to be recreated after deletion")

		// Cleanup verification
		assertEventually(t, func() bool {
			gcs := &apiv1.GatewayClassList{}
			err := k8sClient.List(ctx, gcs)
			return err == nil && len(gcs.Items) == gwClasses.Len()
		}, timeout, interval, "expected final GatewayClass count to be correct")
	})

	t.Run("a default GC is updated", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			if cancel != nil {
				cancel()
			}
			// ensure goroutines cleanup
			time.Sleep(3 * time.Second)
		}()

		managerCancel, err := createManager(t, ctx, nil, nil)
		assertNoError(t, err)
		defer managerCancel()

		// getting the default GC
		gc := &apiv1.GatewayClass{}
		assertEventually(t, func() bool {
			return k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc) == nil
		}, timeout, interval, "expected to get default GatewayClass")

		var description string
		if gc.Spec.Description != nil {
			description = *gc.Spec.Description
		}

		defer func() {
			// restoring the default GC value
			gc := &apiv1.GatewayClass{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc)
			if err == nil {
				gc.Spec.Description = ptr.To(description)
				k8sClient.Update(ctx, gc)
			}
		}()

		// should not be overwritten by the provisioner
		// updating a default GC
		assertEventually(t, func() bool {
			gc = &apiv1.GatewayClass{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc)
			return err == nil
		}, timeout, interval, "expected to get GatewayClass for update")

		// updating the GC
		gc.Spec.Description = ptr.To("updated")
		err = k8sClient.Update(ctx, gc)
		assertNoError(t, err)

		// waiting for the GC to be updated
		assertEventually(t, func() bool {
			gc = &apiv1.GatewayClass{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc)
			if err != nil {
				return false
			}
			if gc.Spec.Description == nil {
				return false
			}
			return *gc.Spec.Description == "updated"
		}, timeout, interval, "expected GatewayClass description to remain updated")
	})

	t.Run("custom GatewayClass configurations are provided", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			if cancel != nil {
				cancel()
			}
			// ensure goroutines cleanup
			time.Sleep(3 * time.Second)
		}()

		customClassConfigs := map[string]*controller.ClassInfo{
			"custom-class": {
				Description: "custom gateway class",
				Labels: map[string]string{
					"custom": "true",
				},
				Annotations: map[string]string{
					"custom.annotation": "value",
				},
			},
		}

		managerCancel, err := createManager(t, ctx, nil, customClassConfigs)
		assertNoError(t, err)
		defer managerCancel()

		// should create GatewayClasses with custom configurations
		assertEventually(t, func() bool {
			gc := &apiv1.GatewayClass{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "custom-class"}, gc); err != nil {
				return false
			}
			return gc.Spec.ControllerName == apiv1.GatewayController(gatewayControllerName) &&
				gc.Spec.Description != nil &&
				*gc.Spec.Description == "custom gateway class" &&
				gc.Labels["custom"] == "true" &&
				gc.Annotations["custom.annotation"] == "value"
		}, timeout, interval, "expected custom GatewayClass to be created with correct configuration")
	})
}

// assertEventually polls a condition function until it returns true or times out
func assertEventually(t *testing.T, condition func() bool, timeout, interval time.Duration, msgAndArgs ...interface{}) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}

	// Build error message
	msg := "condition was not met within timeout"
	if len(msgAndArgs) > 0 {
		if str, ok := msgAndArgs[0].(string); ok {
			msg = str
		}
	}

	t.Fatalf("assertEventually failed: %s (timeout: %v)", msg, timeout)
}
