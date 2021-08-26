# quick walk through

1. get Zane's BMO pr 936
```
oc apply -f config/crd/bases/metal3.io_preprovisioningimages.yaml
```
2. get the rhcos iso
```
curl https://releases-art-rhcos.svc.ci.openshift.org/art/storage/releases/rhcos-4.9/49.84.202107010027-0/x86_64/rhcos-49.84.202107010027-0-live.x86_64.iso --output ~/rhcos-49.84.202107010027-0-live.x86_64.iso
```
3. run the controller
```
export DEPLOY_ISO=$HOME/rhcos-49.84.202107010027-0-live.x86_64.iso
go1.16 run .
```

5. in a new shell
```
oc create ns insta-cow
oc create -f example.yaml

oc get -n insta-cow PreprovisioningImage host-it-34 -o yaml
apiVersion: metal3.io/v1alpha1
kind: PreprovisioningImage
metadata:
  creationTimestamp: "2021-08-25T04:30:00Z"
  generation: 1
  name: host-it-34
  namespace: insta-cow
  resourceVersion: "6355704"
  uid: ea752fba-b6e9-4eca-915a-300a31e4f574
spec:
  networkDataName: mysecret
status:
  conditions:
  - lastTransitionTime: "2021-08-25T04:36:20Z"
    message: Set default image
    observedGeneration: 1
    reason: ImageSuccess
    status: "True"
    type: Ready
  - lastTransitionTime: "2021-08-25T04:36:20Z"
    message: ""
    observedGeneration: 1
    reason: ImageSuccess
    status: "False"
    type: Error
  format: iso
  imageUrl: http://localhost:8083/host-it-34.qcow
  networkData:
    name: mysecret
    version: "6349423"


curl http://localhost:8083/host-it-34.qcow --output host-it-34.qcow
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
 93 1058M   93  985M    0     0   738M      0  0:00:01  0:00:01 --:--:--  737M
curl: (18) transfer closed with 76677120 bytes remaining to read

ls -la host-it-34.qcow
-rw-rw-r--. 1 angus angus 1032847360 Aug 25 00:49 host-it-34.qcow
```
