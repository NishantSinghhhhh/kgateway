package auto_host_rewrite

import (
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	v1alpha1 "github.com/kgateway-dev/kgateway/v2/api/v1alpha1"
	"github.com/kgateway-dev/kgateway/v2/pkg/utils/fsutils"
)

const namespace = "auto-host-rewrite"

var (
	backendManifest              = get("backend.yaml")
	httprouteManifest            = get("httproute.yaml")
	trafficPolicyManifest        = get("trafficpolicy.yaml")
	invalidTrafficPolicyManifest = get("invalid_trafficpolicy.yaml")
)

var (
	proxyObjectMeta = metav1.ObjectMeta{Name: "gw", Namespace: namespace}

	proxyDeployment = &appsv1.Deployment{ObjectMeta: proxyObjectMeta}
	proxyService    = &corev1.Service{ObjectMeta: proxyObjectMeta}
)

var (
	echoDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: namespace},
	}
	echoService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "echo", Namespace: namespace},
	}
)

var (
	route = &gwv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "echo-route", Namespace: namespace},
	}
	trafficPolicy = &v1alpha1.TrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "auto-host-rewrite", Namespace: namespace},
	}
)

func get(file string) string {
	return filepath.Join(fsutils.MustGetThisDir(), "input", file)
}
