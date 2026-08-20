rosactl cluster kubeconfig 8372824b-1131-4c26-91fc-9736694dcd19
```bash
apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: https://api.cdoan-hcp100.8372.0.us-east-1-eph-4b8b842d.dev0.rosa.devshift.net:443
    name: cdoan-hcp100
users:
  - name: cdoan-hcp100-iam
    user:
      exec:
        apiVersion: client.authentication.k8s.io/v1
        interactiveMode: Never
        command: /Users/cdoan/.local/bin/rosactl
        args:
          - cluster
          - get-token
          - --cluster-id
          - 8372824b-1131-4c26-91fc-9736694dcd19
          - --region
          - us-east-1
contexts:
  - context:
      cluster: cdoan-hcp100
      user: cdoan-hcp100-iam
    name: cdoan-hcp100
current-context: cdoan-hcp100
```

# set the kubeconfig
export KUBECONFIG=/tmp/cdoan-hcp100.kube

# switch to the aws account
export AWS_PROFILE=rrp-chris-regional_cluster

# verify we can create pods
kubectl create deployment demo-nginx --image=nginx --replicas=3