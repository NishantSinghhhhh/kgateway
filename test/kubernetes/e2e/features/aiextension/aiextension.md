
# AI Extension E2E Tests

The AI extension end-to-end (e2e) test uses a combination of **Golang** and **Python** to validate its functionality.  
The Go test suite acts as an orchestrator, setting up the environment and invoking the Python **pytest** suite to run tests against various Large Language Model (LLM) providers.

This document describes the two primary workflows for running these tests.

---

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go** and **Docker**
- **kubectl** and **kind**
- **python3** and **virtualenv**
- A local system that supports **MetalLB** for exposing Kubernetes services  
  _(The cluster setup script relies on this to assign an external IP to the gateway.)_

---

## Initial Setup

These steps prepare your local environment by creating a Kubernetes cluster and setting up the Python virtual environment.  
All commands should be run from the **root of the repository**.

---

### 1. Set Up the Python Virtual Environment

```
# Create and activate the virtual environment
python3 -m venv .venv
source .venv/bin/activate

# Install required packages
python3 -m ensurepip --upgrade
python3 -m pip install -r test/kubernetes/e2e/features/aiextension/tests/requirements.txt

# Set the PYTHON environment variable, required by the Go test runner
export PYTHON=$(which python)
```

> **Note:** Python **3.11** is the version currently used in CI.

---

### 2. Create the E2E Cluster

This script spins up a **KIND** cluster configured with **MetalLB**, making it ready for the e2e tests.

```
CONFORMANCE=true ./hack/kind/setup-kind.sh
```

---

## Workflow 1: Running the Full Suite (via Go)

This is the standard method for running the entire test suite.  
The Go test handles setting up all necessary resources before executing the Python tests.

```
go test ./test/kubernetes/e2e/tests/ -run AIExtension
```

To run a specific subset of tests, set the `TEST_PYTHON_STRING_MATCH` environment variable.  
For example, to run only the **vertex_ai** tests:

```
VERSION=1.0.0-ci1 TEST_PYTHON_STRING_MATCH=vertex_ai go test ./test/kubernetes/e2e/tests/ -run AIExtension
```

---

## Workflow 2: Running Python Tests Directly (for Development) 🐍

This workflow is ideal for debugging or developing a specific part of the Python test suite, as it bypasses the Go orchestrator for a faster feedback loop.

---

### Step A: Deploy Kubernetes Resources

Manually deploy the required Gateway, Routes, and test provider configurations.  
The manifests in the **testdata** directory include the `ai-test` namespace.

```
kubectl apply -f test/kubernetes/e2e/features/aiextension/testdata/
```

> Wait a few moments for the resources to be created and for the **ai-gateway** service to be assigned an external IP address.

---

### Step B: Configure Test Environment Variables

The Python tests connect to the gateway using environment variables.

Run these commands to discover the gateway's IP address and export the required variables:

```
# Get the external IP address of the gateway service
export INGRESS_GW_ADDRESS=$(kubectl get svc -n ai-test ai-gateway -o jsonpath="{.status.loadBalancer.ingress['hostname','ip']}")

# Export the base URLs for the different AI providers
export TEST_OPENAI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/openai"
export TEST_AZURE_OPENAI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/azure"
export TEST_GEMINI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/gemini"
export TEST_VERTEX_AI_BASE_URL="http://$INGRESS_GW_ADDRESS:8080/vertex-ai"
```

---

### Step C: Execute the Pytest Suite

Navigate to the directory containing the Python tests and run `pytest`.  
Use the `-k` flag to filter tests.

```
cd test/kubernetes/e2e/features/aiextension/tests/

# Run only the OpenAI streaming tests
python3 -m pytest -vvv --log-cli-level=DEBUG streaming.py -k=openai

# Run only the Vertex AI tests
python3 -m pytest -vvv --log-cli-level=DEBUG -k=vertex_ai
```

---

## Cleanup

Once you are finished, you can delete the KIND cluster to clean up all resources.

```
kind delete cluster --name kgateway-e2ea
