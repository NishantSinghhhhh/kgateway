package autohostrewrite

import (
	"context"
	"path/filepath"

	"github.com/stretchr/testify/suite"

	"github.com/kgateway-dev/kgateway/v2/pkg/utils/fsutils"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e"
)

var _ e2e.NewSuiteFunc = NewInvalidSuite

type invalidSuite struct {
	suite.Suite
	ctx              context.Context
	testInstallation *e2e.TestInstallation
}

func NewInvalidSuite(ctx context.Context, inst *e2e.TestInstallation) suite.TestingSuite {
	return &invalidSuite{ctx: ctx, testInstallation: inst}
}

func (s *invalidSuite) TestWebhookRejects_InvalidCombination() {
	badTP := filepath.Join(
		fsutils.MustGetThisDir(),
		"input/invalid_trafficpolicy.yaml",
	)

	err := s.testInstallation.Actions.Kubectl().
		ApplyFile(s.ctx, badTP) // should fail admission-webhook
	s.Require().Error(err, "expected webhook rejection when both hostRewrite and autoHostRewrite are set")
}