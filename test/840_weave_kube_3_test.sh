#! /bin/bash

. ./config.sh

start_suite "Test weave-kube image"

TOKEN=112233.445566778899000
HOST1IP=$($SSH $HOST1 "getent hosts $HOST1 | cut -f 1 -d ' '")
SUCCESS="6 established"

run_on $HOST1 "sudo systemctl start kubelet && sudo kubeadm init --token=$TOKEN"
run_on $HOST2 "sudo systemctl start kubelet && sudo kubeadm join --token=$TOKEN $HOST1IP"
run_on $HOST3 "sudo systemctl start kubelet && sudo kubeadm join --token=$TOKEN $HOST1IP"

cat ../prog/weave-kube/weave-daemonset.yaml | run_on $HOST1 "sudo kubectl apply -f -"

sleep 5

wait_for_connections() {
    for i in $(seq 1 30); do
        if run_on $HOST1 "curl -sS http://127.0.0.1:6784/status | grep \"$SUCCESS\"" ; then
            return
        fi
        echo "Waiting for connections"
        sleep 1
    done
    echo "Timed out waiting for connections to establish" >&2
    exit 1
}

wait_for_connections

assert_raises "run_on $HOST1 curl -sS http://127.0.0.1:6784/status | grep \"$SUCCESS\""

end_suite
