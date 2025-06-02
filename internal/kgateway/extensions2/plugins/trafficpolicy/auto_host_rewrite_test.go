//go:build unit
// +build unit

package trafficpolicy

import (
	"context"
	"testing"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/ir"
)

func TestTranslate_SetsAutoHostRewrite(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name       string
		input      *bool // value in TrafficPolicy.Spec.autoHostRewrite
		shouldHave bool  // expect non-nil in IR?
		wantValue  bool  // if shouldHave, what value?
	}{
		{"true", boolPtr(true), true, true},
		{"false", boolPtr(false), true, false},
		{"nil", nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// minimal CR
			cr := minimalTrafficPolicyCR(tt.input)

			b := NewTrafficPolicyBuilder(context.Background(), nil) // nil common collections because we don't hit deps
			irPolicy, errs := b.Translate(nil, cr)
			require.Empty(t, errs)

			got := irPolicy.spec.autoHostRewrite

			if !tt.shouldHave {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantValue, got.Value)
		})
	}
}

func TestApplyForRoute_SetsRouteActionFlag(t *testing.T) {
	ctx := context.Background()
	plugin := &trafficPolicyPluginGwPass{}

	t.Run("autoHostRewrite true → RouteAction flag set", func(t *testing.T) {
		policy := &TrafficPolicy{
			spec: trafficPolicySpecIr{
				autoHostRewrite: wrapperspb.Bool(true),
			},
		}

		pCtx := &ir.RouteContext{Policy: policy}
		out := &routev3.Route{
			Action: &routev3.Route_Route{
				Route: &routev3.RouteAction{},
			},
		}

		require.NoError(t, plugin.ApplyForRoute(ctx, pCtx, out))

		ra := out.GetRoute()
		require.NotNil(t, ra)
		assert.NotNil(t, ra.GetAutoHostRewrite())
		assert.True(t, ra.GetAutoHostRewrite().GetValue())
	})

	t.Run("autoHostRewrite nil → RouteAction untouched", func(t *testing.T) {
		policy := &TrafficPolicy{
			spec: trafficPolicySpecIr{autoHostRewrite: nil},
		}
		pCtx := &ir.RouteContext{Policy: policy}
		out := &routev3.Route{
			Action: &routev3.Route_Route{Route: &routev3.RouteAction{}},
		}

		require.NoError(t, plugin.ApplyForRoute(ctx, pCtx, out))

		ra := out.GetRoute()
		require.NotNil(t, ra)
		assert.Nil(t, ra.HostRewriteSpecifier) // nothing written
	})
}

func minimalTrafficPolicyCR(val *bool) *v1alpha1.TrafficPolicy {
	var ahr *bool
	if val != nil {
		ahr = val
	}
	return &v1alpha1.TrafficPolicy{
		Spec: v1alpha1.TrafficPolicySpec{
			AutoHostRewrite: ahr,
		},
	}
}
