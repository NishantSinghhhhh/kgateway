# AI Extension E2E Tests

The AI extension end-to-end (e2e) test uses **Python** to validate its functionality against various Large Language Model (LLM) providers. This document describes the workflow for running these tests directly using Python, which is ideal for development and debugging.

---

## Prerequisites

Before you begin, ensure you have the following installed:

- **python3** (3.11+) and **virtualenv**
- **kubectl** and **kind**
- **Docker**
- A local system that supports **MetalLB** for exposing Kubernetes services  

---

## Initial Setup

These steps prepare your local environment by creating a Kubernetes cluster and setting up the Python virtual environment.  
All commands should be run from the **root of the repository**.

---

### 1. Set up Python virtual environment

```bash
python3 -m venv .venv
source .venv/bin/activate
python3 -m ensurepip --upgrade
python3 -m pip install -r test/kubernetes/e2e/features/aiextension/tests/requirements.txt
export PYTHON=$(which python)
```

---

### 2. Create the E2E Cluster

This script spins up a **KIND** cluster configured with **MetalLB**, making it ready for the e2e tests.

```bash
CONFORMANCE=true ./hack/kind/setup-kind.sh
```

---

### 3. Deploy test resources

```bash
kubectl apply -f test/kubernetes/e2e/features/aiextension/testdata/
```

Wait for the `ai-gateway` service to get an external IP/hostname. You can check the status with:

```bash
kubectl get svc -n ai-test ai-gateway
```

---

### 4. Set environment variables

```bash
export INGRESS_GW_ADDRESS=$(kubectl get svc -n ai-test ai-gateway -o jsonpath="{.status.loadBalancer.ingress[0].hostname}{.status.loadBalancer.ingress[0].ip}")
export TEST_OPENAI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/openai"
export TEST_AZURE_OPENAI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/azure"
export TEST_GEMINI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/gemini"
export TEST_VERTEX_AI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/vertex-ai"
```

**Note:** The `jsonpath` above will output either the hostname or IP, whichever is available. If both are empty, wait a bit longer for the service to be assigned an address.

---

### 5. Run the Python tests

```bash
cd test/kubernetes/e2e/features/aiextension/tests/
python3 -m pytest -vvv --log-cli-level=DEBUG
```

To run only a subset of tests (e.g., only OpenAI streaming tests):

```bash
python3 -m pytest -vvv --log-cli-level=DEBUG streaming.py -k openai
```

Or for Vertex AI:

```bash
python3 -m pytest -vvv --log-cli-level=DEBUG -k vertex_ai
```

**Tip:** The `-k` flag in pytest lets you filter tests by name. You can use it to run only tests matching a substring.

---

### Troubleshooting

- If you get connection errors, ensure the gateway service has an external IP/hostname and your environment variables are set correctly.
- If dependencies are missing, re-run the `pip install` command in your virtual environment.
- If you want to reset the environment, you can delete and recreate the virtualenv.

---

## Cleanup

Once you are finished, you can delete the KIND cluster to clean up all resources.

```bash
kind delete cluster --name kgateway-e2ea
```
