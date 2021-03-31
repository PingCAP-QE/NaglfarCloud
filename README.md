# NaglfarCloud

The cloud native naglfar.

## Prepare images

IMG_PREFIX=hub.pingcap.net/qa/ make upload-image

## Install

### Install naglfar-manager

```bash
# install the naglfar-manager
make deploy-manager
```

### Config the kube-scheduler

1. Apply naglfar-scheduler-rbac.yaml

    ```bash
    $ kubectl apply -f deploy/scheduler/naglfar-scheduler-rbac.yaml
    clusterrole.rbac.authorization.k8s.io/system:kube-scheduler:plugins created
    clusterrolebinding.rbac.authorization.k8s.io/system:kube-scheduler:plugins created
    ```

2. Login control plane nodes
3. Backup kube-scheduler.yaml

    ```bash
    cp /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes/kube-scheduler.yaml
    ```

4. Config a kubeSchedulerConfiguration

    ```bash
    cat > /etc/kubernetes/scheduler-config.yaml <<EOF
    apiVersion: kubescheduler.config.k8s.io/v1beta1
    clientConnection:
      kubeconfig: "/etc/kubernetes/scheduler.conf"
    kind: KubeSchedulerConfiguration
    profiles:
    - schedulerName: default-scheduler
      plugins:
        queueSort:
          enabled:
            - name: naglfar-scheduler
          disabled:
            - name: "*"
        preFilter:
          enabled:
            - name: naglfar-scheduler
        filter:
          enabled:
            - name: naglfar-scheduler
        postFilter:
          enabled:
            - name: naglfar-scheduler
        score:
          enabled:
            - name: naglfar-scheduler
              weight: 10000
        permit:
          enabled:
            - name: naglfar-scheduler
        reserve:
          enabled:
            - name: naglfar-scheduler
        postBind:
          enabled:
            - name: naglfar-scheduler
      pluginConfig:
      - name: naglfar-scheduler
        args:
          scheduleTimeout: 60s
          rescheduleDelayOffset: 20s
          kubeconfig: /etc/kubernetes/scheduler.conf
    EOF
    ```

5. Update the /etc/kubernetes/manifests/kube-scheduler.yaml

    patch.diff:

    ```diff
    --- /etc/kubernetes/kube-scheduler.yaml
    +++ /etc/kubernetes/manifests/kube-scheduler.yaml
    @@ -13,12 +13,14 @@
         - kube-scheduler
         - --authentication-kubeconfig=/etc/kubernetes/scheduler.conf
         - --authorization-kubeconfig=/etc/kubernetes/scheduler.conf
    +    - --config=/etc/kubernetes/scheduler-config.yaml
         - --bind-address=127.0.0.1
         - --kubeconfig=/etc/kubernetes/scheduler.conf
         - --leader-elect=false
         - --port=0
    -    image: k8s.gcr.io/kube-scheduler:v1.19.8
    -    imagePullPolicy: IfNotPresent
    +    - --v=3
    +    image: hub.pingcap.net/qa/naglfar-scheduler:v1.19.8
    +    imagePullPolicy: Always
         livenessProbe:
           failureThreshold: 8
           httpGet:
    @@ -47,6 +49,9 @@
         - mountPath: /etc/kubernetes/scheduler.conf
           name: kubeconfig
           readOnly: true
    +    - mountPath: /etc/kubernetes/scheduler-config.yaml
    +      name: scheduler-config
    +      readOnly: true
       hostNetwork: true
       priorityClassName: system-node-critical
       volumes:
    @@ -54,4 +59,8 @@
           path: /etc/kubernetes/scheduler.conf
           type: FileOrCreate
         name: kubeconfig
    +  - hostPath:
    +      path: /etc/kubernetes/scheduler-config.yaml
    +      type: FileOrCreate
    +    name: scheduler-config
     status: {}
    ```

    ```bash
    patch /etc/kubernetes/kube-scheduler.yaml < patch.diff
    ```
