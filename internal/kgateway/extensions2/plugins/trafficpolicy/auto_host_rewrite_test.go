//go:build unit
// +build unit

package trafficpolicy_test

import (
	"context"
	"testing"

	trafficpolicyv1 "github.com/kgateway-dev/kgateway/v2/apis/trafficpolicy/v1alpha1"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/extensions2/plugins/trafficpolicy"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/ir"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// --- bootstraps the suite ----------------------------------------------------

func TestAutoHostRewritePlugin(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "autoHostRewrite translator-plugin suite")
}

// --- helpers -----------------------------------------------------------------

func boolPtr(b bool) *bool { return &b }

// buildMinimalTP returns *trafficpolicyv1.TrafficPolicy with a single rule
// that points at some dummy backend and the supplied autoHostRewrite value.
func buildMinimalTP(ahr *bool) *trafficpolicyv1.TrafficPolicy {
	return &trafficpolicyv1.TrafficPolicy{
		Spec: trafficpolicyv1.TrafficPolicySpec{
			// …other fields elided; only ahr matters for these tests
			AutoHostRewrite: ahr,
		},
	}
}

// extractAutoHostRewrite digs the generated IR to the single RouteAction that
// the plugin creates and returns RouteAction.AutoHostRewrite (may be nil).
func extractAutoHostRewrite(out *ir.Resources) *wrapperspb.BoolValue {
	// We assume a single virtual-host → single route for these unit tests.
	for _, r := range out.Routes { // map[string]*ir.Route
		if ra := r.Action.GetRoute(); ra != nil { // *routev3.Route
			return ra.AutoHostRewrite
		}
	}
	return nil
}



var _ = ginkgo.Describe("AutoHostRewrite translation", func() {
	ctx := context.Background()
	builder := trafficpolicy.NewTrafficPolicyBuilder() // zero-dep constructor

	type testCase struct {
		name   string
		input  *bool  // TrafficPolicy.Spec.autoHostRewrite
		expect *bool  // value we expect in RouteAction.AutoHostRewrite
	}

	table := []testCase{
		{name: "explicit true", input: boolPtr(true), expect: boolPtr(true)},
		{name: "explicit false", input: boolPtr(false), expect: boolPtr(false)},
		{name: "unset / nil", input: nil, expect: nil},
	}

	for _, tc := range table {
		tc := tc // capture
		ginkgo.It(tc.name, func() {
			tp := buildMinimalTP(tc.input)

			out, err := builder.Translate(ctx, tp)
			Expect(err).NotTo(HaveOccurred())

			got := extractAutoHostRewrite(out)
			switch {
			case tc.expect == nil:
				Expect(got).To(BeNil())
			default:
				Expect(got).NotTo(BeNil())
				Expect(got.Value).To(Equal(*tc.expect))
			}
		})
	}
})
