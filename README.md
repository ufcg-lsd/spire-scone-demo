# SPIRE and SGX-SCONE: Issuing SPIFFE IDs to SGX Confidential Workloads

<!-- # The Services

todo:

- figure with components. -->

# Preparing the K8s cluster

Before starting,

> In this demo, we will be using **Minikube**, on top of **Ubuntu 20.04**, to easily set up a Kubernetes cluster. Also, an **SGX-capable** machine is needed to execute all the steps described in this readme.
> Machine specs: 8GB RAM, 2 CPUs.

To prepare the environment:

- Install SGX Driver following [installation guide](https://sconedocs.github.io/sgxinstall/). 

> Starting with Linux kernel 5.11, you do not need to install an SGX driver anymore.

- Refer to the [Docker installation guide](https://docs.docker.com/engine/install/ubuntu/) to install Docker. It will be the backend driver to setup the K8s cluster with Minikube.
- Add the user to the Docker group (or use the root user to run minikube) with `sudo usermod -aG docker $USER`.
- Install Minikube and kubectl.

```bash
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube

curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

minikube start --cpus=2 --memory=6g
eval $(minikube docker-env)
```

> To ensure your Docker environment variables are pointing to your cluster, use `eval $(minikube docker-env)` in every terminal you open.


# Deploy SCONE and SPIRE Components

Clone the this repo into your machine.

```bash
git clone https://github.com/ufcg-lsd/spire-scone-demo && cd spire-scone-demo
```

Deploy the SCONE components under the `scone` namespace: the Kubernetes-SGX device plugin, the Local Attestation Service (LAS), and the `scone-cli` container that will be used to remotely attest the Configuration and Attestation Service (CAS).

```bash
# In the repository main directory
kubectl apply -f scone/sgx-dev-plugin.yaml
kubectl apply -f scone/scone-namespace.yaml 
kubectl apply -f scone/las-daemonset.yaml
kubectl apply -f scone/cli-scone.yaml
kubectl get pods -n scone
```

In this demonstration, we will use a public CAS at `5-4-0.scone-cas.cf`. In order to use this CAS, we need to create a namepace to post the sessions (configuration and attestation info for confidential workloads) in a way to not overlap with other users.

To do this, we can create a random string to be our namespace name.

```bash
export MY_NAMESPACE=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 15 ; echo '')

# Make $MY_NAMESPACE available inside the scone-cli container
kubectl exec -it -n scone $(kubectl get pods -n scone -o=jsonpath='{.items[0].metadata.name}' \
    -l app=scone-cli)  -- /bin/bash -c 'echo "export MY_NAMESPACE='$MY_NAMESPACE'" > /root/.bashrc'

kubectl cp scone/scone-session-namespace.yaml \
    scone/$(kubectl get pods -n scone -o=jsonpath='{.items[0].metadata.name}' \
    -l app=scone-cli):scone-session-namespace.yaml

kubectl exec -it -n scone $(kubectl get pods -n scone -o=jsonpath='{.items[0].metadata.name}' \
    -l app=scone-cli)  -- /bin/bash

# Now, inside the scone-cli container, you can use `env` to make sure $MY_NAMESPACE is set
# Notice the prompt `bash-5.0#` indicating you're inside the container

# Attest the CAS using 
scone cas attest 5-4-0.scone-cas.cf	 -C -G --only_for_testing-trust-any \
    --only_for_testing-debug --only_for_testing-ignore-signer

# Create the namespace into CAS
scone session create scone-session-namespace.yaml --use-env
exit
```

Use the following commands to deploy the SPIRE Server.

```bash
kubectl apply -f spire/spire-namespace.yaml
kubectl apply -f spire/server-account.yaml
kubectl apply -f spire/server-cluster-role.yaml
kubectl apply -f spire/server-configmap.yaml
kubectl apply -f spire/spire-bundle-configmap.yaml
kubectl apply -f spire/server-statefulset.yaml
kubectl apply -f spire/server-service.yaml
```

Now, deploy an SGX-enabled SPIRE Agent.

```bash
kubectl apply -f spire/agent-account.yaml
kubectl apply -f spire/agent-cluster-role.yaml
envsubst < spire/agent-configmap.yaml | kubectl apply -f -
kubectl apply -f spire/agent-daemonset.yaml

kubectl get pods -n spire
```

Wait for the Agent to present the message `msg="Starting Workload and SDS APIs"`, like below.

```bash
time="2021-07-07T14:00:40Z" level=info msg="Starting Workload and SDS APIs" subsystem_name=endpoints
```
You can use `kubectl` to watch the Agent logs.

```bash
kubectl logs -f -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-agent)
# press Ctrl+C to detach the logs
```


# Deploy Services

Now that we have SCONE and SPIRE components deployed, we can deploy the services, one by one.

## Service 1 - SPIFFE aware client

`Service 1` is a golang client that tries to get a secret from `Service 2`. It trusts the `Service 2` only if the SPIFFE ID `spiffe://example.org/scone-service` is presented by `Service 2`. The env vars in `services/1_spiffe_aware_client/deploy.yaml` configure this SPIFFE ID and the Agent socket path.

To deploy the `Service 1` execute:

```bash
kubectl apply -f services/1_spiffe_aware_client/deploy.yaml
```

Once you deploy `Service 1`, it will not get an SVID because we did not create an entry in SPIRE Server yet. You can see that the attestation process is failing looking at the SPIRE Agent logs.

```bash
kubectl logs -f -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-agent)
# press Ctrl+C to detach the logs
```

To solve this, we will create the registration entry for `Service 1`, with the following commands.

```bash
# Set Agent ID for the next commands
AGENT_ID=$(kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server) -- /bin/sh -c './bin/spire-server agent list' | grep "Spiffe ID" | awk '{print $4}' | tr -d '\r')

kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server)  -- /bin/sh -c 'echo "export AGENT_ID='$AGENT_ID'" >> /root/.shrc'

# Enter the spire-server container to register Service 1
kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server) -- /bin/sh -c 'ENV=/root/.shrc /bin/sh'

./bin/spire-server entry create -parentID $AGENT_ID \
        -registrationUDSPath /tmp/spire-registration.sock \
        -spiffeID spiffe://example.org/client \
        -selector k8s:container-name:spiffe-aware-client
exit
```

With the entry created, `Service 1` gets an SVID and starts to requesting `https://spiffe-service:5000"`, which is the address of Service 2. Once we did not deploy the `Service 2` yet, the requests will fail.

```bash
kubectl logs -f $(kubectl get pods -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spiffe-aware-client)
# press Ctrl+C to detach the logs
```

You should see an error caused by the kube-system DNS service like this:

```
2021/07/06 18:45:03 Requesting counter...
2021/07/06 18:45:03 Error connecting to "https://spiffe-service:5000": Get "https://spiffe-service:5000": dial tcp: lookup spiffe-service on 10.96.0.10:53: server misbehaving
```

## Service 2 - SPIFFE aware SGX-SCONE-enabled service

`Service 2` is an SGX-SCONE-enabled workload. The configuration and constraints for the attestation process are described in `services/2_spiffe_aware_service/session.yaml`. The deployment file for this workload, `services/2_spiffe_aware_service/deploy.yaml`, is configured with the CAS address and other information needed by the workload to be attested and receive the configurations.

To post the session into CAS, we will use the `scone-cli` container again.

```bash
kubectl cp services/2_spiffe_aware_service/session.yaml \
    scone/$(kubectl get pods -n scone -o=jsonpath='{.items[0].metadata.name}' \
    -l app=scone-cli):2_spiffe_aware_service-session.yaml

kubectl exec -it -n scone $(kubectl get pods -n scone -o=jsonpath='{.items[0].metadata.name}' \
    -l app=scone-cli)  -- /bin/bash

# Inside the scone-cli container
HASH_SERVICE_2=$(scone session create 2_spiffe_aware_service-session.yaml --use-env)
# Get the session hash for Service 2
echo $HASH_SERVICE_2
exit
```

Copy the content of `$HASH_SERVICE_2`.

```bash
export HASH_SERVICE_2=<content copied from the scone-cli container>
```

Now, let's create a registration entry for `Service 2` using the SPIRE Server registration API.

```bash
# Inject $MY_NAMESPACE and $HASH_SERVICE_2 into spire-server container
kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server)  -- /bin/sh -c 'echo "export MY_NAMESPACE='$MY_NAMESPACE'" >> /root/.shrc'

kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server)  -- /bin/sh -c 'echo "export HASH_SERVICE_2='$HASH_SERVICE_2'" >> /root/.shrc'

# Enter the spire-server container to register Service 2
kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server) -- /bin/sh -c 'ENV=/root/.shrc /bin/sh'

./bin/spire-server entry create -parentID $AGENT_ID \
        -registrationUDSPath /tmp/spire-registration.sock \
        -spiffeID spiffe://example.org/scone-service \
        -selector svidstore:type:scone_cas_secretsmanager \
        -selector cas_session_hash:$HASH_SERVICE_2 \
        -selector cas_session_name:$MY_NAMESPACE/svid-session
exit
```

At this moment, the SPIRE Agent will fetch the entry for `Service 2` and will start to push the SVID into the SCONE CAS. You can see the Agent logs using `kubectl`. The PoC is configured to be verbose to facilitate the demonstration.

```bash
kubectl logs -f -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-agent)
# press Ctrl+C to detach the logs
```

Now, we can deploy `Service 2`.

```bash
envsubst < services/2_spiffe_aware_service/deploy.yaml | kubectl apply -f -
```

Looking at `Service 2` logs, we can see that it received the SVID. However, it is returning a 500 status code to the `Service 1` because the `Service 3` (Secrets service) is unavailable (`"error": "HTTPSConnectionPool(host='secret-service', port=5000): Max retries exceeded`). We can use `kubectl` one more time to see the logs.

```bash
# Service 2 - spiffe-service
kubectl logs -f $(kubectl get pods -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spiffe-service)
# press Ctrl+C to detach the logs

# Service 1 - spiffe-aware-client
kubectl logs -f $(kubectl get pods -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spiffe-aware-client)
# press Ctrl+C to detach the logs
```

## Service 3 - Secret Service (HTTP Service protected with SCONE network shielding)

`Service 3` provides a JSON response to `Service 2`. Besides, the code only talks HTTP. We leverage the SCONE network shielding to create a TLS secure channel wrapping the TCP connection. The network shielding is also SPIFFE compliant. It can use SVIDs in the TLS connections.

Let's post a session for `Service 3` into CAS.

```bash
kubectl cp services/3_secret_service/network-shielding-session.yaml \
    scone/$(kubectl get pods -n scone -o=jsonpath='{.items[0].metadata.name}' \
    -l app=scone-cli):3_network-shielding-session.yaml

kubectl exec -it -n scone $(kubectl get pods -n scone -o=jsonpath='{.items[0].metadata.name}' \
    -l app=scone-cli)  -- /bin/bash

# Inside the scone-cli container
HASH_SERVICE_3=$(scone session create 3_network-shielding-session.yaml --use-env)
# Get the session hash for Service 2
echo $HASH_SERVICE_3
exit
```

Copy the content of `$HASH_SERVICE_3`.

```bash
export HASH_SERVICE_3=<content copied from the scone-cli container>
```

Also, we need to create an registration entry for `Service 3`.

```bash
# Inject $HASH_SERVICE_3 into spire-server container
kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server)  -- /bin/sh -c 'echo "export HASH_SERVICE_3='$HASH_SERVICE_3'" >> /root/.shrc'

# Enter the spire-server container to register Service 3
kubectl exec -it -n spire $(kubectl get pods -n spire -o=jsonpath='{.items[0].metadata.name}' \
    -l app=spire-server) -- /bin/sh -c 'ENV=/root/.shrc /bin/sh'

./bin/spire-server entry create -parentID $AGENT_ID \
        -registrationUDSPath /tmp/spire-registration.sock \
        -spiffeID spiffe://example.org/scone-legacy-service \
        -selector svidstore:type:scone_cas_secretsmanager \
        -selector cas_session_hash:$HASH_SERVICE_3 \
        -selector cas_session_name:$MY_NAMESPACE/netshield-session \
        -dns secret-service
exit
```

Now, we can deploy `Service 3`.

```bash
envsubst < services/3_secret_service/deploy.yaml | kubectl apply -f -
```

The `Service 3` will be protected by the network shielding, gaining an SPIFFE Identity from SPIRE. This identity is given after a successful attestation process.


# Clean up resources

```bash
minikube delete
```
