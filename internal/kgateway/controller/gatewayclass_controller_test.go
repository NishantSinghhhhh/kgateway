package controller_test

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestGatewayClassStatusController(t *testing.T) {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var cancel context.CancelFunc

	// Setup
	var err error
	cancel, err = createManager(ctx, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Cleanup
	defer func() {
		if cancel != nil {
			cancel()
		}
		// ensure goroutines cleanup
		time.Sleep(3 * time.Second)
	}()

	t.Run("GatewayClass reconciliation", func(t *testing.T) {
		var gc *apiv1.GatewayClass

		// Setup GatewayClass
		gc = &apiv1.GatewayClass{}
		if !eventually(func() bool {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc); err != nil {
				return false
			}
			return true
		}, timeout, interval) {
			t.Fatalf("GatewayClass %s not found", gatewayClassName)
		}

		t.Run("should set the Accepted=True condition type", func(t *testing.T) {
			if !eventuallyCondition(t, func() (*metav1.Condition, error) {
				gc := &apiv1.GatewayClass{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc); err != nil {
					return nil, err
				}
				return meta.FindStatusCondition(gc.Status.Conditions, string(apiv1.GatewayClassConditionStatusAccepted)), nil
			}, timeout, interval, func(c *metav1.Condition) bool {
				if c == nil {
					return false
				}
				if c.Type != string(apiv1.GatewayClassConditionStatusAccepted) {
					t.Errorf("Expected condition type %s, got %s", string(apiv1.GatewayClassConditionStatusAccepted), c.Type)
					return false
				}
				if c.Status != metav1.ConditionTrue {
					t.Errorf("Expected condition status %s, got %s", metav1.ConditionTrue, c.Status)
					return false
				}
				if c.Reason != string(apiv1.GatewayClassReasonAccepted) {
					t.Errorf("Expected condition reason %s, got %s", string(apiv1.GatewayClassReasonAccepted), c.Reason)
					return false
				}
				if !contains(c.Message, "accepted by kgateway controller") {
					t.Errorf("Expected condition message to contain 'accepted by kgateway controller', got %s", c.Message)
					return false
				}
				return true
			}) {
				t.Fatal("Condition validation failed")
			}
		})

		t.Run("should set the SupportedVersion=True condition type", func(t *testing.T) {
			if !eventuallyCondition(t, func() (*metav1.Condition, error) {
				gc := &apiv1.GatewayClass{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: gatewayClassName}, gc); err != nil {
					return nil, err
				}
				return meta.FindStatusCondition(gc.Status.Conditions, string(apiv1.GatewayClassConditionStatusSupportedVersion)), nil
			}, timeout, interval, func(c *metav1.Condition) bool {
				if c == nil {
					return false
				}
				if c.Type != string(apiv1.GatewayClassConditionStatusSupportedVersion) {
					t.Errorf("Expected condition type %s, got %s", string(apiv1.GatewayClassConditionStatusSupportedVersion), c.Type)
					return false
				}
				if c.Status != metav1.ConditionTrue {
					t.Errorf("Expected condition status %s, got %s", metav1.ConditionTrue, c.Status)
					return false
				}
				if c.Reason != string(apiv1.GatewayClassReasonSupportedVersion) {
					t.Errorf("Expected condition reason %s, got %s", string(apiv1.GatewayClassReasonSupportedVersion), c.Reason)
					return false
				}
				if !contains(c.Message, "supported by kgateway controller") {
					t.Errorf("Expected condition message to contain 'supported by kgateway controller', got %s", c.Message)
					return false
				}
				return true
			}) {
				t.Fatal("Condition validation failed")
			}
		})
	})
}

// Helper functions to replicate Ginkgo's Eventually behavior
func eventually(condition func() bool, timeout, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

func eventuallyCondition(t *testing.T, getCondition func() (*metav1.Condition, error), timeout, interval time.Duration, validate func(*metav1.Condition) bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		condition, err := getCondition()
		if err != nil {
			t.Logf("Error getting condition: %v", err)
			time.Sleep(interval)
			continue
		}
		if validate(condition) {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > len(substr) && s[len(s)-len(substr):] == substr) || s[:len(substr)] == substr || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}