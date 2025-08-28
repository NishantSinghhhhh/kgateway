# AI Extension E2E Tests

The AI extension end-to-end (e2e) test uses **Python** to validate its functionality against various Large Language Model (LLM) providers. This document describes the workflow for running these tests directly using Python, which is ideal for development and debugging.

-----

## Prerequisites

Before you begin, ensure you have the following installed:

  - **python3** (3.11+) 
  - **kubectl** and **kind**
  - **Docker**

-----

## Initial Setup

These steps prepare your local environment by creating a Kubernetes cluster and setting up the Python virtual environment.  
All commands should be run from the **root of the repository**.

-----

### 1. Set up Python virtual environment

```
python3 -m venv .venv
source .venv/bin/activate
python3 -m ensurepip --upgrade
python3 -m pip install -r test/kubernetes/e2e/features/aiextension/tests/requirements.txt
```

-----

### 2. Define Cluster Name

Export the cluster name as an environment variable to ensure it's used consistently for creation and deletion.

```
export KIND_CLUSTER_NAME="kgateway-e2e"
```

-----

### 3. Create the E2E Cluster

This script spins up a **KIND** cluster, builds the local images, and packages the local helm charts:

```
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME} ./hack/kind/setup-kind.sh
```

-----

### 4. Install CRDs and Controller (via Helm)

Before deploying test resources, install all required CRDs and the controller using Helm.

```
# Install CRDs
helm install kgateway-crds install/helm/kgateway-crds --namespace ai-test --create-namespace

# Install Controller
helm install kgateway-controller install/helm/kgateway --namespace ai-test
```
-----

### 5. Deploy test resources

```
kubectl apply -f test/kubernetes/e2e/features/aiextension/testdata/
```

Wait for the `ai-gateway` service to get an external IP/hostname. You can check the status with:

```
kubectl get svc -n ai-test ai-gateway
```

### 6. Set environment variables

```
export INGRESS_GW_ADDRESS=$(kubectl get svc -n ai-test ai-gateway -o jsonpath="{.status.loadBalancer.ingress.hostname}{.status.loadBalancer.ingress.ip}")
export TEST_OPENAI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/openai"
export TEST_AZURE_OPENAI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/azure"
export TEST_GEMINI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/gemini"
export TEST_VERTEX_AI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/vertex-ai"
```

**Note:** The `jsonpath` above will output either the hostname or IP, whichever is available. If both are empty, wait a bit longer for the service to be assigned an address.

-----

### 7. Run the Python tests

# To run all test files
cd test/kubernetes/e2e/features/aiextension/tests/
python3 -m pytest -vvv --log-cli-level=DEBUG

# To run only streaming.py
python3 -m pytest -vvv --log-cli-level=DEBUG streaming.py

# To run only routing.py
python3 -m pytest -vvv --log-cli-level=DEBUG routing.py

# To run only test(s) matching 'vertex_ai' in routing.py, e.g.
python3 -m pytest -vvv --log-cli-level=DEBUG routing.py -k vertex_ai


### Troubleshooting

  - If you get connection errors, ensure the gateway service has an external IP/hostname and your environment variables are set correctly.
  - If dependencies are missing, re-run the `pip install` command in your virtual environment.
  - If you want to reset the environment, you can delete and recreate the virtualenv.

-----

## Cleanup

Once you are finished, you can delete the KIND cluster to clean up all resources.

```
kind delete cluster --name ${KIND_CLUSTER_NAME}
```