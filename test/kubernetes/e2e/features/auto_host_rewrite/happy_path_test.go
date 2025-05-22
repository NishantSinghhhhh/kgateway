package autohostrewrite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kgateway-dev/kgateway/v2/pkg/utils/fsutils"
	"github.com/kgateway-dev/kgateway/v2/pkg/utils/kubeutils"
	"github.com/kgateway-dev/kgateway/v2/pkg/utils/requestutils/curl"
	"github.com/kgateway-dev/kgateway/v2/test/gomega/matchers"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e/defaults"
)

// TestAutoHostRewrite runs the AutoHostRewrite E2E suite.
func TestAutoHostRewrite(t *testing.T) {
	e2e.RunSuite(t, NewTestingSuite)
}

// Ensure NewTestingSuite matches the harness signature.
var _ e2e.NewSuiteFunc = NewTestingSuite

// NewTestingSuite constructs the testify suite for AutoHostRewrite.
func NewTestingSuite(ctx context.Context, ti *e2e.TestInstallation) suite.TestingSuite {
	return &happySuite{ctx: ctx, ti: ti}
}

// happySuite holds test context and installation.
type happySuite struct {
	suite.Suite
	ctx context.Context
	ti  *e2e.TestInstallation
}

const ns = "auto-host-rewrite"

var (
	dir = fsutils.MustGetThisDir()

	manifests = []string{
		filepath.Join(dir, "input/backend.yaml"),
		filepath.Join(dir, "input/httproute.yaml"),
		filepath.Join(dir, "input/trafficpolicy.yaml"),
		defaults.CurlPodManifest,
	}

	gwSvc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-proxy", Namespace: ns},
	}
)

// TestAutoHostRewrite_HappyPath verifies the Host header is rewritten.
func (s *happySuite) TestAutoHostRewrite_HappyPath() {
	// Clean up resources when done.
	s.T().Cleanup(func() {
		for _, m := range manifests {
			_ = s.ti.Actions.Kubectl().DeleteFileSafe(s.ctx, m)
		}
	})

	// Apply YAML manifests
	for _, m := range manifests {
		s.Require().NoError(s.ti.Actions.Kubectl().ApplyFile(s.ctx, m))
	}

	// Wait for pods: echo and curl
	s.ti.Assertions.EventuallyPodsRunning(s.ctx, ns,
		metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=echo"},
	)
	s.ti.Assertions.EventuallyPodsRunning(s.ctx, defaults.CurlPod.GetNamespace(),
		metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=curl"},
	)

	// Perform curl and check rewritten Host
	s.ti.Assertions.AssertEventualCurlResponse(
		s.ctx,
		defaults.CurlPodExecOpt,
		[]curl.Option{
			curl.WithHost(kubeutils.ServiceFQDN(gwSvc.ObjectMeta)),
			curl.WithHostHeader("foo.local"),
			curl.WithPath("/"),
		},
		&matchers.HttpResponse{
			StatusCode: http.StatusOK,
			Body:       gomega.ContainSubstring("host=echo." + ns + ".svc.cluster.local"),
		},
	)
}
