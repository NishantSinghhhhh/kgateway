package auto_host_rewrite

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kgateway-dev/kgateway/v2/pkg/utils/fsutils"

	"github.com/kgateway-dev/kgateway/v2/pkg/utils/kubeutils"
	"github.com/kgateway-dev/kgateway/v2/pkg/utils/requestutils/curl"
	testmatchers "github.com/kgateway-dev/kgateway/v2/test/gomega/matchers"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e"
	testdefaults "github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e/defaults"
)

const namespace = "auto-host-rewrite"

var _ e2e.NewSuiteFunc = NewTestingSuite // makes the suite discoverable

type testingSuite struct {
	suite.Suite

	ctx              context.Context
	testInstallation *e2e.TestInstallation

	commonManifests []string
	commonResources []client.Object
}

func NewTestingSuite(ctx context.Context, ti *e2e.TestInstallation) suite.TestingSuite {
	return &testingSuite{ctx: ctx, testInstallation: ti}
}

/* ───────────────────────── Set-up / Tear-down ───────────────────────── */

func (s *testingSuite) SetupSuite() {
	// 1) first create the namespace itself
	nsManifest := filepath.Join(fsutils.MustGetThisDir(), "testdata", "namespace.yaml")

	s.commonManifests = []string{
		nsManifest,
		testdefaults.CurlPodManifest,
		backendManifest,
		httprouteManifest,
		trafficPolicyManifest,
	}
	s.commonResources = []client.Object{
		testdefaults.CurlPod,
		echoDeployment, echoService,
		proxyDeployment, proxyService,
		route, trafficPolicy,
	}

	for _, mf := range s.commonManifests {
		s.Require().NoError(
			s.testInstallation.Actions.Kubectl().ApplyFile(s.ctx, mf),
			"apply "+mf,
		)
	}
	s.testInstallation.Assertions.EventuallyObjectsExist(s.ctx, s.commonResources...)

	// wait for all pods to actually come up
	s.testInstallation.Assertions.EventuallyPodsRunning(s.ctx,
		testdefaults.CurlPod.GetNamespace(),
		metav1.ListOptions{LabelSelector: testdefaults.CurlPodLabelSelector},
	)
	s.testInstallation.Assertions.EventuallyPodsRunning(s.ctx,
		echoDeployment.GetNamespace(),
		metav1.ListOptions{LabelSelector: "app=echo"},
	)
	s.testInstallation.Assertions.EventuallyPodsRunning(s.ctx,
		proxyObjectMeta.GetNamespace(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", proxyObjectMeta.GetName()),
		},
	)
}
func (s *testingSuite) TearDownSuite() {
	for _, mf := range s.commonManifests {
		_ = s.testInstallation.Actions.Kubectl().DeleteFileSafe(s.ctx, mf)
	}
	s.testInstallation.Assertions.EventuallyObjectsNotExist(s.ctx, s.commonResources...)
}

/* ──────────────────────────── Test Cases ──────────────────────────── */

func (s *testingSuite) TestHostHeaderIsRewritten() {
	s.assertResponse("/", http.StatusOK)
}

func (s *testingSuite) TestInvalidCombinationWebhookRejects() {
	manifest := invalidTrafficPolicyManifest

	s.T().Cleanup(func() {
		_ = s.testInstallation.Actions.Kubectl().DeleteFileSafe(s.ctx, manifest)
	})

	err := s.testInstallation.Actions.Kubectl().ApplyFile(s.ctx, manifest)
	s.Require().Error(err)
	s.Contains(err.Error(), "hostRewrite and autoHostRewrite are mutually exclusive")
}

func (s *testingSuite) assertResponse(path string, expectedStatus int) {
	s.testInstallation.Assertions.AssertEventuallyConsistentCurlResponse(
		s.ctx,
		testdefaults.CurlPodExecOpt,
		[]curl.Option{
			curl.WithPath(path),
			curl.WithHost(kubeutils.ServiceFQDN(proxyObjectMeta)),
			curl.WithHostHeader("foo.local"),
			curl.WithPort(8080),
		},
		&testmatchers.HttpResponse{
			StatusCode: expectedStatus,
			Body:       gomega.ContainSubstring("Host: echo.default.svc.cluster.local"),
		},
	)
}
