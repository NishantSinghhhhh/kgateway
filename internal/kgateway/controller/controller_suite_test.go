package controller_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"istio.io/istio/pkg/kube"
	istiosets "istio.io/istio/pkg/util/sets"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	infextv1a2 "sigs.k8s.io/gateway-api-inference-extension/api/v1alpha2"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kgateway-dev/kgateway/v2/api/v1alpha1"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/controller"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/extensions2/registry"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/krtcollections"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/setup"
	"github.com/kgateway-dev/kgateway/v2/internal/kgateway/wellknown"
	"github.com/kgateway-dev/kgateway/v2/pkg/client/clientset/versioned"
	"github.com/kgateway-dev/kgateway/v2/pkg/deployer"
	"github.com/kgateway-dev/kgateway/v2/pkg/pluginsdk/collections"
	"github.com/kgateway-dev/kgateway/v2/pkg/pluginsdk/krtutil"
	"github.com/kgateway-dev/kgateway/v2/pkg/schemes"
	"github.com/kgateway-dev/kgateway/v2/pkg/settings"
)

const (
	gatewayClassName            = "clsname"
	altGatewayClassName         = "clsname-alt"
	selfManagedGatewayClassName = "clsname-selfmanaged"
	gatewayControllerName       = "kgateway.dev/kgateway"
	defaultNamespace            = "default"
)

var (
	cfg          *rest.Config
	k8sClient    client.Client
	testEnv      *envtest.Environment
	ctx          context.Context
	cancel       context.CancelFunc
	kubeconfig   string
	gwClasses    = sets.New(gatewayClassName, altGatewayClassName, selfManagedGatewayClassName)
	scheme       *runtime.Scheme
	inferenceExt *deployer.InferenceExtInfo
)

func TestMain(m *testing.M) {
	// Setup logger for tests
	log.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	// Setup test environment
	if err := setupTestEnvironment(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup test environment: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	teardownTestEnvironment()

	os.Exit(code)
}

func setupTestEnvironment() error {
	fmt.Println("Bootstrapping test environment")

	// Create a scheme and add both Gateway and InferencePool types.
	scheme = schemes.GatewayScheme()
	if err := infextv1a2.Install(scheme); err != nil {
		return fmt.Errorf("failed to install inference extension scheme: %w", err)
	}

	// Required to deploy endpoint picker RBAC resources.
	if err := rbacv1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add rbac to scheme: %w", err)
	}

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "crds"),
			filepath.Join("..", "..", "..", "install", "helm", "kgateway-crds", "templates"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: getAssetsDir(testing.TB(nil)),
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		return fmt.Errorf("failed to start test environment: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("test environment config is nil")
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	if k8sClient == nil {
		return fmt.Errorf("k8s client is nil")
	}

	return nil
}

func teardownTestEnvironment() {
	if cancel != nil {
		cancel()
	}

	fmt.Println("Tearing down the test environment")
	if testEnv != nil {
		if err := testEnv.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop test environment: %v\n", err)
		}
	}

	if kubeconfig != "" {
		os.Remove(kubeconfig)
	}
}

func getAssetsDir(t testing.TB) string {
	var assets string
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		out, err := exec.Command("sh", "-c", "make -sC $(dirname $(go env GOMOD)) envtest-path").CombinedOutput()
		fmt.Printf("envtest assets output: %s\n", string(out))
		assertNoError(t, err)
		assets = strings.TrimSpace(string(out))
	}
	return assets
}

func generateKubeConfiguration(t *testing.T, restconfig *rest.Config) string {
	clusters := make(map[string]*clientcmdapi.Cluster)
	authinfos := make(map[string]*clientcmdapi.AuthInfo)
	contexts := make(map[string]*clientcmdapi.Context)

	clusterName := "cluster"
	clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   restconfig.Host,
		CertificateAuthorityData: restconfig.CAData,
	}
	authinfos[clusterName] = &clientcmdapi.AuthInfo{
		ClientKeyData:         restconfig.KeyData,
		ClientCertificateData: restconfig.CertData,
	}
	contexts[clusterName] = &clientcmdapi.Context{
		Cluster:   clusterName,
		Namespace: "default",
		AuthInfo:  clusterName,
	}

	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "cluster",
		AuthInfos:      authinfos,
	}

	tmpfile, err := os.CreateTemp("", "ggii_envtest_*.kubeconfig")
	assertNoError(t, err)
	tmpfile.Close()

	err = clientcmd.WriteToFile(clientConfig, tmpfile.Name())
	assertNoError(t, err)

	return tmpfile.Name()
}

type fakeDiscoveryNamespaceFilter struct{}

func (f fakeDiscoveryNamespaceFilter) Filter(obj any) bool {
	return true
}

func (f fakeDiscoveryNamespaceFilter) AddHandler(func(selected, deselected istiosets.String)) {}

func createManager(
	t *testing.T,
	parentCtx context.Context,
	inferenceExt *deployer.InferenceExtInfo,
	classConfigs map[string]*controller.ClassInfo,
) (context.CancelFunc, error) {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
		Controller: config.Controller{
			SkipNameValidation: ptr.To(true),
		},
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	assertNoError(t, err)

	ctx, cancel := context.WithCancel(parentCtx)
	kubeClient, _ := setup.CreateKubeClient(cfg)
	gwCfg := controller.GatewayConfig{
		Mgr:            mgr,
		ControllerName: gatewayControllerName,
		AutoProvision:  true,
		ImageInfo: &deployer.ImageInfo{
			Registry: "ghcr.io/kgateway-dev",
			Tag:      "latest",
		},
		DiscoveryNamespaceFilter: fakeDiscoveryNamespaceFilter{},
		CommonCollections:        newCommonCols(t, ctx, kubeClient),
	}
	err = controller.NewBaseGatewayController(parentCtx, gwCfg, nil)
	assertNoError(t, err)

	err = mgr.GetClient().Create(ctx, &v1alpha1.GatewayParameters{
		ObjectMeta: metav1.ObjectMeta{
			Name:      selfManagedGatewayClassName,
			Namespace: "default",
		},
		Spec: v1alpha1.GatewayParametersSpec{
			SelfManaged: &v1alpha1.SelfManagedGateway{},
		},
	})
	if client.IgnoreAlreadyExists(err) != nil {
		cancel()
		assertNoError(t, err)
	}

	if classConfigs == nil {
		classConfigs = map[string]*controller.ClassInfo{}
		classConfigs[altGatewayClassName] = &controller.ClassInfo{
			Description: "alt gateway class",
		}
		classConfigs[gatewayClassName] = &controller.ClassInfo{
			Description: "default gateway class",
		}
		classConfigs[selfManagedGatewayClassName] = &controller.ClassInfo{
			Description: "self managed gw",
			ParametersRef: &apiv1.ParametersReference{
				Group:     apiv1.Group(wellknown.GatewayParametersGVK.Group),
				Kind:      apiv1.Kind(wellknown.GatewayParametersGVK.Kind),
				Name:      selfManagedGatewayClassName,
				Namespace: ptr.To(apiv1.Namespace("default")),
			},
		}
	}

	err = controller.NewGatewayClassProvisioner(mgr, gatewayControllerName, classConfigs)
	assertNoError(t, err)

	poolCfg := &controller.InferencePoolConfig{
		Mgr:            mgr,
		ControllerName: gatewayControllerName,
		InferenceExt:   inferenceExt,
	}
	err = controller.NewBaseInferencePoolController(parentCtx, poolCfg, &gwCfg, nil)
	assertNoError(t, err)

	go func() {
		kubeconfig = generateKubeConfiguration(t, cfg)
		mgr.GetLogger().Info("starting manager", "kubeconfig", kubeconfig)
		err := mgr.Start(ctx)
		assertNoError(t, err)
	}()

	return func() {
		cancel()
		kubeClient.Shutdown()
	}, nil
}

func newCommonCols(t *testing.T, ctx context.Context, kubeClient kube.Client) *collections.CommonCollections {
	krtopts := krtutil.NewKrtOptions(ctx.Done(), nil)
	cli, err := versioned.NewForConfig(cfg)
	assertNoError(t, err)

	settings, err := settings.BuildSettings()
	assertNoError(t, err)

	commoncol, err := collections.NewCommonCollections(ctx, krtopts, kubeClient, cli, nil, gatewayControllerName, *settings)
	assertNoError(t, err)

	plugins := registry.Plugins(ctx, commoncol, wellknown.DefaultWaypointClassName)
	plugins = append(plugins, krtcollections.NewBuiltinPlugin(ctx))
	extensions := registry.MergePlugins(plugins...)

	commoncol.InitPlugins(ctx, extensions, *settings)
	kubeClient.RunAndWait(ctx.Done())
	return commoncol
}

// Helper functions for tests to use instead of Ginkgo's Expect
func assertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}
