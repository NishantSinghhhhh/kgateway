package tests_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/crds"
	"github.com/kgateway-dev/kgateway/v2/pkg/schemes"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e"
	. "github.com/kgateway-dev/kgateway/v2/test/kubernetes/e2e/tests"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/testutils/cluster"
	"github.com/kgateway-dev/kgateway/v2/test/kubernetes/testutils/install"
	testruntime "github.com/kgateway-dev/kgateway/v2/test/kubernetes/testutils/runtime"
)

var (
	// poolCrdManifest defines the manifest file containing Inference Extension CRDs.
	// Created using command:
	//   kubectl kustomize "https://github.com/kubernetes-sigs/gateway-api-inference-extension/config/crd/?ref=$COMMIT_SHA" \
	//   > internal/kgateway/crds/inference-crds.yaml
	poolCrdManifest = filepath.Join(crds.AbsPathToCrd("inference-crds.yaml"))
	// BEGIN: Updated to use XListenerSet CRD
	// xListenerSetCrdManifest defines the manifest for the XListenerSet CRD.
	// This is required for the e2e tests to pass as it's a dependency.
	// Note: The stable ListenerSet CRD is not available yet, so we use the experimental XListenerSet.
	xListenerSetCrdManifest = filepath.Join(crds.AbsPathToCrd("gateway-crds.yaml"))
	// END: Updated to use XListenerSet CRD
	// infExtNs is the namespace to install kgateway
	infExtNs = "inf-ext-e2e"
)

// TestInferenceExtension tests Inference Extension functionality
func TestInferenceExtension(t *testing.T) {
	ctx := context.Background()

	runtimeContext := testruntime.NewContext()
	clusterContext := cluster.MustKindContextWithScheme(runtimeContext.ClusterName, schemes.InferExtScheme())

	installContext := &install.Context{
		InstallNamespace:          infExtNs,
		ProfileValuesManifestFile: e2e.ManifestPath("inference-extension-helm.yaml"),
		ValuesManifestFile:        e2e.EmptyValuesManifestPath,
	}

	testInstallation := e2e.CreateTestInstallationForCluster(
		t,
		runtimeContext,
		clusterContext,
		installContext,
	)

	// We register the cleanup function _before_ we actually perform the installation.
	// This allows us to uninstall kgateway, in case the original installation only completed partially
	t.Cleanup(func() {
		if t.Failed() {
			testInstallation.PreFailHandler(ctx)
		}

		testInstallation.UninstallKgateway(ctx)

		// Uninstall InferencePool v1 CRD
		err := testInstallation.Actions.Kubectl().DeleteFile(ctx, poolCrdManifest)
		testInstallation.Assertions.Require.NoError(err, "can delete manifest %s", poolCrdManifest)
		// BEGIN: Updated to use XListenerSet CRD
		// Note: We don't need to uninstall the XListenerSet CRD as it's part of the gateway-crds.yaml
		// which is managed by the main kgateway installation
		// END: Updated to use XListenerSet CRD
	})

	// Install InferencePool v1 CRD
	err := testInstallation.Actions.Kubectl().ApplyFile(ctx, poolCrdManifest)
	testInstallation.Assertions.Require.NoError(err, "can apply manifest %s", poolCrdManifest)

	// BEGIN: Updated to use XListenerSet CRD
	// Note: The XListenerSet CRD is already included in the gateway-crds.yaml
	// which is installed by the main kgateway installation, so we don't need to install it separately
	// END: Updated to use XListenerSet CRD

	// Install kgateway
	testInstallation.InstallKgatewayFromLocalChart(ctx)
	testInstallation.Assertions.EventuallyNamespaceExists(ctx, infExtNs)

	// Run the e2e tests
	InferenceExtensionSuiteRunner().Run(ctx, t, testInstallation)
}
