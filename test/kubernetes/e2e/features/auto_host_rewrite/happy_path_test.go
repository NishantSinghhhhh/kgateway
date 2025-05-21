package autohostrewrite

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/onsi/ginkgo/v2"
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

func TestAutoHostRewrite(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "AutoHostRewrite E2E Suite")
}

// -----------------------------------------------------------------------------
// suite registration – this is how TestInstallation is injected
// -----------------------------------------------------------------------------

var _ e2e.NewSuiteFunc = NewTestingSuite

func NewTestingSuite(ctx context.Context, ti *e2e.TestInstallation) suite.TestingSuite {
	return &suiteImpl{ctx: ctx, ti: ti}
}

// -----------------------------------------------------------------------------
// suite definition
// -----------------------------------------------------------------------------

type suiteImpl struct {
	suite.Suite
	ctx context.Context
	ti  *e2e.TestInstallation
}

// -----------------------------------------------------------------------------
// constants & manifest paths
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// happy-path test
// -----------------------------------------------------------------------------

func (s *suiteImpl) TestAutoHostRewrite_HappyPath() {
	// clean-up after the test
	s.T().Cleanup(func() {
		for _, m := range manifests {
			_ = s.ti.Actions.Kubectl().DeleteFileSafe(s.ctx, m)
		}
	})

	// apply manifests
	for _, m := range manifests {
		s.Require().NoError(s.ti.Actions.Kubectl().ApplyFile(s.ctx, m))
	}

	// wait for echo & curl pods
	s.ti.Assertions.EventuallyPodsRunning(s.ctx, ns,
		metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=echo"},
	)
	s.ti.Assertions.EventuallyPodsRunning(s.ctx, defaults.CurlPod.GetNamespace(),
		metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=curl"},
	)

	// assert traffic
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