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

var (
	backendManifest       = filepath.Join(fsutils.MustGetThisDir(), "testdata", "backend.yaml")
	httprouteManifest     = filepath.Join(fsutils.MustGetThisDir(), "testdata", "httproute.yaml")
	trafficPolicyManifest = filepath.Join(fsutils.MustGetThisDir(), "testdata", "trafficpolicy.yaml")

	invalidTrafficPolicyManifest = filepath.Join(
		fsutils.MustGetThisDir(), "testdata", "invalid_trafficpolicy.yaml")

	/* objects from gateway manifest (gw.yaml) */
	proxyObjectMeta = metav1.ObjectMeta{
		Name:      "gw",
		Namespace: "default",
	}
	proxyDeployment = &appsv1.Deployment{ObjectMeta: proxyObjectMeta}
	proxyService    = &corev1.Service{ObjectMeta: proxyObjectMeta}

	/* backend service + deployment */
	echoDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "echo",
			Namespace: "default",
		},
	}
	echoService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "echo",
			Namespace: "default",
		},
	}

	/* route + traffic-policy */
	route = &gwv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "echo-route",
			Namespace: "default",
		},
	}
	trafficPolicy = &v1alpha1.TrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auto-host-rewrite",
			Namespace: "default",
		},
	}
)
