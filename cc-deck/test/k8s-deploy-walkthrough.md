# K8s Deploy Manual Walkthrough

Manual test plan for verifying the k8s-deploy environment type against a kind cluster.
Run each section in order.
Every step includes the expected outcome so you can confirm correct behavior.

## Prerequisites

- `kind` installed (`brew install kind`)
- `kubectl` installed
- `podman` installed (for building the stub image)
- cc-deck CLI built (`make install` from project root, or `cd cc-deck && go build -o /tmp/cc-deck-test ./cmd/cc-deck`)

## Setup

### 1. Create a kind cluster

```bash
kind create cluster --name cc-deck-test
```

**Expected**: Cluster created, kubectl context set to `kind-cc-deck-test`.

### 2. Build and load the stub image

```bash
# Build with podman
podman build -t localhost/cc-deck-stub:latest -f cc-deck/test/Containerfile.stub .

# Save and load into kind (kind needs a Docker/OCI archive)
podman save localhost/cc-deck-stub:latest -o /tmp/cc-deck-stub.tar
kind load image-archive /tmp/cc-deck-stub.tar --name cc-deck-test
```

**Expected**: Image available inside the kind cluster.

### 3. Create the test namespace

```bash
kubectl create namespace cc-deck-test
```

**Expected**: Namespace `cc-deck-test` created.

### 4. Set up a temp state directory

Use a temp directory to avoid polluting your real cc-deck state:

```bash
export CC_DECK_STATE_FILE=$(mktemp -d)/state.yaml
export CC_DECK_DEFINITIONS_FILE=$(mktemp -d)/definitions.yaml
```

---

## Test 1: Core Lifecycle (US1)

### 1a. Create environment

```bash
cc-deck env create test-basic \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --no-network-policy \
  --credential TEST_KEY=test-value \
  --timeout 3m
```

**Expected**: "Environment "test-basic" created (type: k8s-deploy)"

### 1b. Verify K8s resources

```bash
kubectl -n cc-deck-test get statefulset cc-deck-test-basic
kubectl -n cc-deck-test get service cc-deck-test-basic
kubectl -n cc-deck-test get configmap cc-deck-test-basic
kubectl -n cc-deck-test get secret cc-deck-test-basic-creds
kubectl -n cc-deck-test get pod cc-deck-test-basic-0
```

**Expected**: All resources exist. Pod is Running. StatefulSet has 1 replica ready.

### 1c. Verify labels

```bash
kubectl -n cc-deck-test get statefulset cc-deck-test-basic -o jsonpath='{.metadata.labels}' | jq .
```

**Expected**:
```json
{
  "app.kubernetes.io/component": "workspace",
  "app.kubernetes.io/instance": "test-basic",
  "app.kubernetes.io/managed-by": "cc-deck",
  "app.kubernetes.io/name": "cc-deck"
}
```

### 1d. Verify credential Secret

```bash
kubectl -n cc-deck-test get secret cc-deck-test-basic-creds -o jsonpath='{.data.TEST_KEY}' | base64 -d
```

**Expected**: `test-value`

### 1e. Verify credential mount

```bash
kubectl -n cc-deck-test exec cc-deck-test-basic-0 -- cat /run/secrets/cc-deck/TEST_KEY
```

**Expected**: `test-value`

### 1f. Verify PVC

```bash
kubectl -n cc-deck-test get pvc
```

**Expected**: A PVC named `data-cc-deck-test-basic-0` exists and is Bound.

### 1g. List environments

```bash
cc-deck env list
```

**Expected**: `test-basic` appears with type `k8s-deploy`, state `running`, storage `pvc`.

### 1h. Status

```bash
cc-deck env status test-basic
```

**Expected**: Shows type k8s-deploy, status running.

### 1i. Stop

```bash
cc-deck env stop test-basic
```

**Expected**: "Environment "test-basic" stopped". StatefulSet scaled to 0:

```bash
kubectl -n cc-deck-test get statefulset cc-deck-test-basic -o jsonpath='{.spec.replicas}'
# Expected: 0
```

### 1j. Verify PVC preserved after stop

```bash
kubectl -n cc-deck-test get pvc
```

**Expected**: PVC still exists and is Bound.

### 1k. Start

```bash
cc-deck env start test-basic
```

**Expected**: "Environment "test-basic" started". Pod comes back up:

```bash
kubectl -n cc-deck-test get pod cc-deck-test-basic-0
# Expected: Running
```

### 1l. Write a file, stop, start, verify persistence

```bash
kubectl -n cc-deck-test exec cc-deck-test-basic-0 -- sh -c 'echo "persistence-test" > /workspace/testfile.txt'
cc-deck env stop test-basic
cc-deck env start test-basic
kubectl -n cc-deck-test exec cc-deck-test-basic-0 -- cat /workspace/testfile.txt
```

**Expected**: `persistence-test` (file survived stop/start cycle).

### 1m. Delete with --keep-volumes

```bash
cc-deck env delete test-basic --force --keep-volumes
```

**Expected**: StatefulSet, Service, ConfigMap, Secret deleted. PVC preserved:

```bash
kubectl -n cc-deck-test get statefulset cc-deck-test-basic 2>&1
# Expected: "not found"
kubectl -n cc-deck-test get pvc
# Expected: PVC still exists
```

### 1n. Clean up preserved PVC

```bash
kubectl -n cc-deck-test delete pvc data-cc-deck-test-basic-0
```

---

## Test 2: Duplicate Name Conflict

```bash
cc-deck env create dup-test \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --no-network-policy \
  --timeout 3m

cc-deck env create dup-test \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --no-network-policy
```

**Expected**: First create succeeds. Second create fails with "environment with this name already exists".

```bash
cc-deck env delete dup-test --force
```

---

## Test 3: Existing Secret (US2)

### 3a. Create a pre-existing Secret

```bash
kubectl -n cc-deck-test create secret generic my-api-keys \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-existing-test
```

### 3b. Create environment with existing secret

```bash
cc-deck env create test-existing-secret \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --existing-secret my-api-keys \
  --no-network-policy \
  --timeout 3m
```

**Expected**: Environment created. No `cc-deck-test-existing-secret-creds` Secret created.

### 3c. Verify mount

```bash
kubectl -n cc-deck-test exec cc-deck-test-existing-secret-0 -- cat /run/secrets/cc-deck/ANTHROPIC_API_KEY
```

**Expected**: `sk-ant-existing-test`

### 3d. Delete and verify Secret preserved

```bash
cc-deck env delete test-existing-secret --force
kubectl -n cc-deck-test get secret my-api-keys
```

**Expected**: Environment deleted. `my-api-keys` Secret still exists (user-managed, not deleted).

```bash
kubectl -n cc-deck-test delete secret my-api-keys
```

---

## Test 4: NetworkPolicy (US3)

### 4a. Create with default NetworkPolicy

```bash
cc-deck env create test-netpol \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --credential TEST=val \
  --timeout 3m
```

### 4b. Verify NetworkPolicy exists

```bash
kubectl -n cc-deck-test get networkpolicy cc-deck-test-netpol
kubectl -n cc-deck-test get networkpolicy cc-deck-test-netpol -o yaml | head -30
```

**Expected**: NetworkPolicy exists with deny-all egress base and DNS egress rule (port 53).

### 4c. Create with --no-network-policy

```bash
cc-deck env delete test-netpol --force

cc-deck env create test-no-netpol \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --no-network-policy \
  --credential TEST=val \
  --timeout 3m

kubectl -n cc-deck-test get networkpolicy cc-deck-test-no-netpol 2>&1
```

**Expected**: "not found" (no NetworkPolicy created).

```bash
cc-deck env delete test-no-netpol --force
```

---

## Test 5: File Sync (US6)

### 5a. Create environment

```bash
cc-deck env create test-sync \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --no-network-policy \
  --credential TEST=val \
  --timeout 3m
```

### 5b. Push files

```bash
mkdir -p /tmp/cc-deck-push-test
echo "hello from local" > /tmp/cc-deck-push-test/greeting.txt

cc-deck env push test-sync /tmp/cc-deck-push-test
```

**Expected**: "Pushed to environment "test-sync""

### 5c. Verify files arrived

```bash
kubectl -n cc-deck-test exec cc-deck-test-sync-0 -- cat /workspace/greeting.txt
```

**Expected**: `hello from local`

### 5d. Pull files

```bash
mkdir -p /tmp/cc-deck-pull-test
cc-deck env pull test-sync /workspace /tmp/cc-deck-pull-test
cat /tmp/cc-deck-pull-test/greeting.txt
```

**Expected**: `hello from local`

### 5e. Exec

```bash
cc-deck env exec test-sync -- echo "exec works"
```

**Expected**: `exec works`

```bash
cc-deck env delete test-sync --force
```

---

## Test 6: Custom Storage (US1 variant)

```bash
cc-deck env create test-storage \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --storage-size 1Gi \
  --no-network-policy \
  --credential TEST=val \
  --timeout 3m

kubectl -n cc-deck-test get pvc -o jsonpath='{.items[0].spec.resources.requests.storage}'
```

**Expected**: `1Gi`

```bash
cc-deck env delete test-storage --force
```

---

## Test 7: Invalid Inputs (Edge Cases)

### 7a. Invalid namespace

```bash
cc-deck env create test-bad-ns \
  --type k8s-deploy \
  --namespace nonexistent-namespace \
  --image localhost/cc-deck-stub:latest \
  --no-network-policy 2>&1
```

**Expected**: Error mentioning namespace does not exist. No resources created.

### 7b. Invalid name

```bash
cc-deck env create INVALID_NAME \
  --type k8s-deploy \
  --namespace cc-deck-test 2>&1
```

**Expected**: Error about invalid environment name.

### 7c. Delete non-running without --force

```bash
cc-deck env create test-force \
  --type k8s-deploy \
  --namespace cc-deck-test \
  --image localhost/cc-deck-stub:latest \
  --no-network-policy \
  --credential TEST=val \
  --timeout 3m

cc-deck env delete test-force 2>&1
```

**Expected**: Error "environment is running; use --force to delete"

```bash
cc-deck env delete test-force --force
```

---

## Teardown

```bash
kind delete cluster --name cc-deck-test
unset CC_DECK_STATE_FILE CC_DECK_DEFINITIONS_FILE
rm -rf /tmp/cc-deck-push-test /tmp/cc-deck-pull-test /tmp/cc-deck-stub.tar
```

---

## Summary Checklist

| Test | Covers | Pass? |
|------|--------|-------|
| 1a-1n | US1: Create, list, status, stop, start, delete, persistence, keep-volumes | |
| 2 | Edge: duplicate name conflict | |
| 3a-3d | US2: Existing secret mount, preservation on delete | |
| 4a-4c | US3: NetworkPolicy default, --no-network-policy | |
| 5a-5e | US6: Push, pull, exec | |
| 6 | US1: Custom storage size | |
| 7a-7c | Edge: invalid namespace, invalid name, delete without force | |
