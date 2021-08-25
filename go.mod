module github.com/asalkeld/image-customization-controller

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/golangci/golangci-lint v1.32.0
	github.com/metal3-io/baremetal-operator v0.0.0-00010101000000-000000000000
	github.com/metal3-io/baremetal-operator/apis v0.0.0
	github.com/openshift/assisted-image-service v0.0.0-20210825003515-8675374a2fc2
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	sigs.k8s.io/controller-runtime v0.9.6
)

replace (
	github.com/metal3-io/baremetal-operator => github.com/zaneb/baremetal-operator v0.0.0-20210712202217-703cc8031a6e
	github.com/metal3-io/baremetal-operator/apis => github.com/zaneb/baremetal-operator/apis v0.0.0-20210712202217-703cc8031a6e
)
