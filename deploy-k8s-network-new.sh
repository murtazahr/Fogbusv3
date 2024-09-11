#!/bin/bash

# Function to generate PBFT keys
generate_pbft_keys() {
    local num_fog_nodes=$1
    local keys=""
    for ((i=0; i<num_fog_nodes; i++)); do
        priv_key=$(openssl ecparam -name secp256k1 -genkey | openssl ec -text -noout | grep priv -A 3 | tail -n +2 | tr -d '\n[:space:]:' | sed 's/^00//')
        pub_key=$(openssl ecparam -name secp256k1 -genkey | openssl ec -text -noout | grep pub -A 5 | tail -n +2 | tr -d '\n[:space:]:' | sed 's/^04//')
        keys+="      pbft${i}priv: $priv_key"$'\n'
        keys+="      pbft${i}pub: $pub_key"$'\n'
    done
    echo "$keys"
}

# Function to check if a node exists in the cluster
check_node_exists() {
    local node_name=$1
    kubectl get nodes | grep -q "$node_name"
    return $?
}

# Function to generate CouchDB cluster deployment YAML
generate_couchdb_yaml() {
    local num_fog_nodes=$1
    local yaml_content="apiVersion: v1
kind: List

items:"

    # Generate PVCs
    for ((i=0; i<num_fog_nodes; i++)); do
        yaml_content+="
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: couchdb${i}-data
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi"
    done

    # Generate Deployments
    for ((i=0; i<num_fog_nodes; i++)); do
        yaml_content+="
  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: couchdb-${i}
    spec:
      selector:
        matchLabels:
          app: couchdb-${i}
      replicas: 1
      template:
        metadata:
          labels:
            app: couchdb-${i}
        spec:
          nodeSelector:
            kubernetes.io/hostname: fog-node-$((i+1))
          containers:
            - name: couchdb
              image: couchdb:3
              ports:
                - containerPort: 5984
              env:
                - name: COUCHDB_USER
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_USER
                - name: COUCHDB_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_PASSWORD
                - name: COUCHDB_SECRET
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_SECRET
                - name: ERL_FLAGS
                  value: \"-setcookie \\\"\${ERLANG_COOKIE}\\\" -kernel inet_dist_listen_min 9100 -kernel inet_dist_listen_max 9200\"
                - name: NODENAME
                  value: \"couchdb-${i}.default.svc.cluster.local\"
              volumeMounts:
                - name: couchdb-data
                  mountPath: /opt/couchdb/data
              readinessProbe:
                httpGet:
                  path: /
                  port: 5984
                initialDelaySeconds: 5
                periodSeconds: 10
          volumes:
            - name: couchdb-data
              persistentVolumeClaim:
                claimName: couchdb${i}-data"
    done

    # Generate Services
    for ((i=0; i<num_fog_nodes; i++)); do
        yaml_content+="
  - apiVersion: v1
    kind: Service
    metadata:
      name: couchdb-${i}
    spec:
      clusterIP: None
      selector:
        app: couchdb-${i}
      ports:
        - port: 5984
          targetPort: 5984"
    done

    # Generate CouchDB Cluster Setup Job
    yaml_content+="
  - apiVersion: batch/v1
    kind: Job
    metadata:
      name: couchdb-setup
    spec:
      template:
        metadata:
          name: couchdb-setup
        spec:
          restartPolicy: OnFailure
          containers:
            - name: couchdb-setup
              image: curlimages/curl:latest
              command:
                - /bin/sh
              args:
                - -c
                - |
                  DB_NAME=\"resource_registry\" &&
                  echo \"Starting CouchDB cluster setup\" &&
                  for i in \$(seq 0 $((num_fog_nodes-1))); do
                    echo \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-\${i}.default.svc.cluster.local:5984\"
                    until curl -s \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-\${i}.default.svc.cluster.local:5984\" > /dev/null; do
                      echo \"Waiting for CouchDB on couchdb-\${i} to be ready...\"
                      sleep 5
                    done
                    echo \"CouchDB on couchdb-\${i} is ready\"
                  done &&
                  echo \"Adding nodes to the cluster\" &&
                  for num in \$(seq 1 $((num_fog_nodes-1))); do
                    response=\$(curl -X POST -H 'Content-Type: application/json' \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-0.default.svc.cluster.local:5984/_cluster_setup\" -d \"{\\\"action\\\": \\\"enable_cluster\\\", \\\"bind_address\\\":\\\"0.0.0.0\\\", \\\"username\\\": \\\"\${COUCHDB_USER}\\\", \\\"password\\\":\\\"\${COUCHDB_PASSWORD}\\\", \\\"port\\\": 5984, \\\"node_count\\\": \\\"$num_fog_nodes\\\", \\\"remote_node\\\": \\\"couchdb-\${num}.default.svc.cluster.local\\\", \\\"remote_current_user\\\": \\\"\${COUCHDB_USER}\\\", \\\"remote_current_password\\\": \\\"\${COUCHDB_PASSWORD}\\\" }\")
                    echo \"Enable cluster on couchdb-\${num} response: \${response}\"
                    response=\$(curl -s -X POST -H 'Content-Type: application/json' \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-0.default.svc.cluster.local:5984/_cluster_setup\" -d \"{\\\"action\\\": \\\"add_node\\\", \\\"host\\\":\\\"couchdb-\${num}.default.svc.cluster.local\\\", \\\"port\\\": 5984, \\\"username\\\": \\\"\${COUCHDB_USER}\\\", \\\"password\\\":\\\"\${COUCHDB_PASSWORD}\\\"}\")
                    echo \"Adding node couchdb-\${num} response: \${response}\"
                  done &&
                  echo \"Finishing cluster setup\" &&
                  response=\$(curl -s -X POST -H 'Content-Type: application/json' \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-0.default.svc.cluster.local:5984/_cluster_setup\" -d \"{\\\"action\\\": \\\"finish_cluster\\\"}\") &&
                  echo \"Finish cluster response: \${response}\" &&
                  echo \"Checking cluster membership\" &&
                  membership=\$(curl -s -X GET \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-0.default.svc.cluster.local:5984/_membership\") &&
                  echo \"Cluster membership: \${membership}\" &&
                  echo \"Creating \${RESOURCE_REGISTRY_DB}, \${TASK_DATA_DB} and \${SCHEDULES_DB} database on all nodes\" &&
                  response=\$(curl -s -X PUT \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-0.default.svc.cluster.local:5984/\${RESOURCE_REGISTRY_DB}\") &&
                  echo \"Creating \${RESOURCE_REGISTRY_DB} on couchdb-0 response: \${response}\" &&
                  response=\$(curl -s -X PUT \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-0.default.svc.cluster.local:5984/\${SCHEDULES_DB}\") &&
                  echo \"Creating \${SCHEDULES_DB} on couchdb-0 response: \${response}\" &&
                  response=\$(curl -s -X PUT \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-0.default.svc.cluster.local:5984/\${TASK_DATA_DB}\") &&
                  echo \"Creating \${TASK_DATA_DB} on couchdb-0 response: \${response}\" &&
                  echo \"Waiting for \${RESOURCE_REGISTRY_DB}, \${TASK_DATA_DB} and \${SCHEDULES_DB} to be available on all nodes\" &&
                  for db in \${RESOURCE_REGISTRY_DB} \${SCHEDULES_DB} \${TASK_DATA_DB}; do
                    for i in \$(seq 0 $((num_fog_nodes-1))); do
                      until curl -s \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-\${i}.default.svc.cluster.local:5984/\${db}\" | grep -q \"\${db}\"; do
                        echo \"Waiting for \${db} on couchdb-\${i}...\"
                        sleep 5
                      done
                      echo \"\${db} is available on couchdb-\${i}\"
                    done
                  done &&
                  echo \"CouchDB cluster setup completed and \${RESOURCE_REGISTRY_DB}, \${SCHEDULES_DB} & \${TASK_DATA_DB} is available on all nodes\"
              env:
                - name: RESOURCE_REGISTRY_DB
                  value: \"resource_registry\"
                - name: SCHEDULES_DB
                  value: \"schedules\"
                - name: TASK_DATA_DB
                  value: \"task_data\"
                - name: COUCHDB_USER
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_USER
                - name: COUCHDB_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_PASSWORD"

    echo "$yaml_content"
}

# Function to generate blockchain network deployment YAML
generate_blockchain_network_yaml() {
    local num_fog_nodes=$1
    local num_iot_nodes=$2
    local yaml_content="apiVersion: v1
kind: List

items:"

    # Generate Fog Node Deployments and Services
    for ((i=0; i<num_fog_nodes; i++)); do
        local hostname="fog-node-$((i+1))"
        local deployment_name="pbft-$i"
        local service_name="sawtooth-$i"

        yaml_content+="
  # --------------------------=== $hostname ===--------------------------

  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: $deployment_name
    spec:
      selector:
        matchLabels:
          name: $deployment_name
      replicas: 1
      template:
        metadata:
          labels:
            name: $deployment_name
        spec:
          nodeSelector:
            kubernetes.io/hostname: $hostname
          volumes:
            - name: proc
              hostPath:
                path: /proc
            - name: sys
              hostPath:
                path: /sys
          initContainers:
            - name: wait-for-registry
              image: busybox
              command: [ 'sh', '-c', 'until nc -z sawtooth-registry 5000; do echo waiting for sawtooth-registry; sleep 2; done;' ]
            - name: wait-for-couchdb-setup
              image: curlimages/curl:latest
              command:
                - 'sh'
                - '-c'
                - |
                  for db in \${RESOURCE_REGISTRY_DB} \${SCHEDULES_DB} \${TASK_DATA_DB}; do
                    for i in \$(seq 0 $((num_fog_nodes-1))); do
                      until curl -s \"http://\${COUCHDB_USER}:\${COUCHDB_PASSWORD}@couchdb-\${i}.default.svc.cluster.local:5984/\${db}\" | grep -q \"\${db}\"; do
                        echo \"Waiting for \${db} on couchdb-\${i}...\"
                        sleep 5
                      done
                      echo \"\${db} is available on couchdb-\${i}\"
                    done
                  done &&
                  echo \"CouchDB cluster setup completed and \${RESOURCE_REGISTRY_DB}, \${SCHEDULES_DB} & \${TASK_DATA_DB} is available on all nodes\"
              env:
                - name: RESOURCE_REGISTRY_DB
                  value: \"resource_registry\"
                - name: SCHEDULES_DB
                  value: \"schedules\"
                - name: TASK_DATA_DB
                  value: \"task_data\"
                - name: COUCHDB_USER
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_USER
                - name: COUCHDB_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_PASSWORD
          containers:
            - name: peer-registry-tp
              image: murtazahr/peer-registry-tp:latest
              env:
                - name: MAX_UPDATES_PER_NODE
                  value: \"100\"
                - name: VALIDATOR_URL
                  value: \"tcp://$service_name:4004\"

            - name: docker-image-tp
              image: murtazahr/docker-image-tp:latest
              env:
                - name: VALIDATOR_URL
                  value: \"tcp://$service_name:4004\"

            - name: dependency-management-tp
              image: murtazahr/dependency-management-tp:latest
              env:
                - name: VALIDATOR_URL
                  value: \"tcp://$service_name:4004\"

            - name: scheduling-tp
              image: murtazahr/scheduling-tp:latest
              env:
                - name: VALIDATOR_URL
                  value: \"tcp://$service_name:4004\"
                - name: COUCHDB_HOST
                  value: \"couchdb-$i.default.svc.cluster.local:5984\"
                - name: COUCHDB_USER
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_USER
                - name: COUCHDB_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_PASSWORD

            - name: sawtooth-pbft-engine
              image: hyperledger/sawtooth-pbft-engine:chime
              command:
                - bash
              args:
                - -c
                - \"pbft-engine -vv --connect tcp://\$HOSTNAME:5050\"

            - name: sawtooth-settings-tp
              image: hyperledger/sawtooth-settings-tp:chime
              command:
                - bash
              args:
                - -c
                - \"settings-tp -vv -C tcp://\$HOSTNAME:4004\"

            - name: fog-node
              image: murtazahr/fog-node:latest
              securityContext:
                privileged: true
              volumeMounts:
                - name: proc
                  mountPath: /host/proc
                  readOnly: true
                - name: sys
                  mountPath: /host/sys
                  readOnly: true
              env:
                - name: REGISTRY_URL
                  value: \"sawtooth-registry:5000\"
                - name: VALIDATOR_URL
                  value: \"tcp://$service_name:4004\"
                - name: NODE_ID
                  value: \"sawtooth-fog-node-$i\"
                - name: COUCHDB_HOST
                  value: \"couchdb-$i.default.svc.cluster.local:5984\"
                - name: RESOURCE_UPDATE_INTERVAL
                  value: \"300\"
                - name: RESOURCE_UPDATE_BATCH_SIZE
                  value: \"10\"
                - name: COUCHDB_USER
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_USER
                - name: COUCHDB_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: couchdb-secrets
                      key: COUCHDB_PASSWORD

            - name: sawtooth-validator
              image: hyperledger/sawtooth-validator:chime
              ports:
                - name: tp
                  containerPort: 4004
                - name: consensus
                  containerPort: 5050
                - name: validators
                  containerPort: 8800
              env:
                - name: pbft${i}priv
                  valueFrom:
                    configMapKeyRef:
                      name: keys-config
                      key: pbft${i}priv
                - name: pbft${i}pub
                  valueFrom:
                    configMapKeyRef:
                      name: keys-config
                      key: pbft${i}pub"

        if [ "$i" -eq 0 ]; then
            yaml_content+="
              command:
                - bash
              args:
                - -c
                - |
                  if [ ! -e /etc/sawtooth/keys/validator.priv ]; then
                    echo \$pbft${i}priv > /etc/sawtooth/keys/validator.priv
                    echo \$pbft${i}pub > /etc/sawtooth/keys/validator.pub
                  fi &&
                  if [ ! -e /root/.sawtooth/keys/my_key.priv ]; then
                    sawtooth keygen my_key
                  fi &&
                  if [ ! -e config-genesis.batch ]; then
                    sawset genesis -k /root/.sawtooth/keys/my_key.priv -o config-genesis.batch
                  fi &&
                  sleep 30 &&
                  echo sawtooth.consensus.pbft.members=[$(for ((j=0; j<num_fog_nodes; j++)); do echo -n "\"$\{pbft${j}pub\}\""; if [ $j -lt $((num_fog_nodes-1)) ]; then echo -n ","; fi; done)] &&
                  if [ ! -e config.batch ]; then
                    sawset proposal create \
                      -k /root/.sawtooth/keys/my_key.priv \
                      sawtooth.consensus.algorithm.name=pbft \
                      sawtooth.consensus.algorithm.version=1.0 \
                      sawtooth.consensus.pbft.members=[$(for ((j=0; j<num_fog_nodes; j++)); do echo -n "\"$\{pbft${j}pub\}\""; if [ $j -lt $((num_fog_nodes-1)) ]; then echo -n ","; fi; done)] \
                      sawtooth.publisher.max_batches_per_block=1200 \
                      -o config.batch
                  fi && \
                  if [ ! -e /var/lib/sawtooth/genesis.batch ]; then
                    sawadm genesis config-genesis.batch config.batch
                  fi &&
                  sawtooth-validator -vv \
                    --endpoint tcp://\$SAWTOOTH_0_SERVICE_HOST:8800 \
                    --bind component:tcp://eth0:4004 \
                    --bind consensus:tcp://eth0:5050 \
                    --bind network:tcp://eth0:8800 \
                    --scheduler parallel \
                    --peering static \
                    --maximum-peer-connectivity 10000"
        else
            yaml_content+="
              command:
                - bash
              args:
                - -c
                - |
                  if [ ! -e /etc/sawtooth/keys/validator.priv ]; then
                    echo \$pbft${i}priv > /etc/sawtooth/keys/validator.priv
                    echo \$pbft${i}pub > /etc/sawtooth/keys/validator.pub
                  fi &&
                  sawtooth keygen my_key &&
                  sawtooth-validator -vv \
                    --endpoint tcp://\$SAWTOOTH_${i}_SERVICE_HOST:8800 \
                    --bind component:tcp://eth0:4004 \
                    --bind consensus:tcp://eth0:5050 \
                    --bind network:tcp://eth0:8800 \
                    --scheduler parallel \
                    --peering static \
                    --maximum-peer-connectivity 10000 \
                    $(for ((j=0; j<i; j++)); do echo -n "--peers tcp://\$SAWTOOTH_${j}_SERVICE_HOST:8800 "; done)"
        fi

        yaml_content+="

  - apiVersion: v1
    kind: Service
    metadata:
      name: $service_name
    spec:
      type: ClusterIP
      selector:
        name: $deployment_name
      ports:
        - name: \"4004\"
          protocol: TCP
          port: 4004
          targetPort: 4004
        - name: \"5050\"
          protocol: TCP
          port: 5050
          targetPort: 5050
        - name: \"8080\"
          protocol: TCP
          port: 8080
          targetPort: 8080
        - name: \"8800\"
          protocol: TCP
          port: 8800
          targetPort: 8800"
    done

    # Generate IoT Node Deployments
    for ((i=0; i<num_iot_nodes; i++)); do
        yaml_content+="

  # -------------------------=== iot-node-$((i+1)) ===------------------

  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: iot-$i
    spec:
      selector:
        matchLabels:
          name: iot-$i
      replicas: 1
      template:
        metadata:
          labels:
            name: iot-$i
        spec:
          nodeSelector:
            kubernetes.io/hostname: iot-node-$((i+1))
          containers:
            - name: scheduling-client
              image: murtazahr/scheduling-client:latest
              env:
                - name: VALIDATOR_URL
                  value: \"tcp://sawtooth-0:4004\""
    done

    # Add Client Console Deployment
    yaml_content+="

  # -------------------------=== client-console ===------------------

  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: network-management-console
    spec:
      selector:
        matchLabels:
          name: network-management-console
      replicas: 1
      template:
        metadata:
          labels:
            name: network-management-console
        spec:
          nodeSelector:
            kubernetes.io/hostname: client-console
          containers:
            - name: application-deployment-client
              image: murtazahr/docker-image-client:latest
              securityContext:
                privileged: true
              env:
                - name: REGISTRY_URL
                  value: \"sawtooth-registry:5000\"
                - name: VALIDATOR_URL
                  value: \"tcp://sawtooth-0:4004\"

            - name: dependency-management-client
              image: murtazahr/dependency-management-client:latest
              env:
                - name: VALIDATOR_URL
                  value: \"tcp://sawtooth-0:4004\"

            - name: scheduling-client
              image: murtazahr/scheduling-client:latest
              env:
                - name: VALIDATOR_URL
                  value: \"tcp://sawtooth-0:4004\""

    echo "$yaml_content"
}

# Main script starts here
echo "Enter the number of fog nodes:"
read num_fog_nodes
echo "Enter the number of IoT nodes:"
read num_iot_nodes

# Part 1: Verify inputs and check node existence
if [ "$num_fog_nodes" -lt 3 ]; then
    echo "Error: The number of fog nodes must be at least 3."
    exit 1
fi

echo "Checking for fog nodes..."
for ((i=1; i<=num_fog_nodes; i++)); do
    if ! check_node_exists "fog-node-$i"; then
        echo "Error: fog-node-$i does not exist in the cluster."
        exit 1
    fi
done

echo "Checking for IoT nodes..."
for ((i=1; i<=num_iot_nodes; i++)); do
    if ! check_node_exists "iot-node-$i"; then
        echo "Error: iot-node-$i does not exist in the cluster."
        exit 1
    fi
done

echo "All required nodes are present in the cluster."

# Part 2: Generate YAML file for config and secrets
generated_keys=$(generate_pbft_keys "$num_fog_nodes")

mkdir -p kubernetes-manifests/generated

cat << EOF > kubernetes-manifests/generated/config-and-secrets.yaml
apiVersion: v1
kind: List

items:
  # --------------------------=== Blockchain Setup Keys ===----------------------
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: keys-config
    data:
$generated_keys
  # --------------------------=== CouchDB Secrets ===---------------------------
  - apiVersion: v1
    kind: Secret
    metadata:
      name: couchdb-secrets
    type: Opaque
    stringData:
      COUCHDB_USER: fogbus
      COUCHDB_PASSWORD: mwg478jR04vAonMu2QnFYF3sVyVKUujYrGrzVsrq3I
      COUCHDB_SECRET: LEv+K7x24ITqcAYp0R0e1GzBqiE98oSSarPD1sdeOyM=
      ERLANG_COOKIE: jT7egojgnPLzOncq9MQU/zqwqHm6ZiPUU7xJfFLA8MA=

  # --------------------------=== Docker Registry Secret ===----------------------
  - apiVersion: v1
    kind: Secret
    metadata:
      name: registry-secret
    type: Opaque
    stringData:
      http-secret: Y74bs7QpaHmI1NKDGO8I3JdquvVxL+5K15NupwxhSbc=
EOF

echo "Generated YAML file for config and secrets has been saved to kubernetes-manifests/generated/config-and-secrets.yaml"

# Part 3: Generate CouchDB cluster deployment YAML
couchdb_yaml=$(generate_couchdb_yaml "$num_fog_nodes")

# Save the generated CouchDB YAML to a file
echo "$couchdb_yaml" > kubernetes-manifests/generated/couchdb-cluster-deployment.yaml

echo "Generated CouchDB cluster deployment YAML has been saved to kubernetes-manifests/generated/couchdb-cluster-deployment.yaml"

# Part 4: Generate blockchain network deployment YAML
blockchain_network_yaml=$(generate_blockchain_network_yaml "$num_fog_nodes" "$num_iot_nodes")

# Save the generated blockchain network YAML to a file
echo "$blockchain_network_yaml" > kubernetes-manifests/generated/blockchain-network-deployment.yaml

echo "Generated blockchain network deployment YAML has been saved to kubernetes-manifests/generated/blockchain-network-deployment.yaml"

echo "Script execution completed successfully."