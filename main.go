/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metal3iocontroller "github.com/asalkeld/image-customization-controller/controllers/metal3.io"
	"github.com/asalkeld/image-customization-controller/pkg/imagehandler"
	metal3iov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/metal3-io/baremetal-operator/pkg/version"
	// +kubebuilder:scaffold:imports
)

var (
	scheme     = k8sruntime.NewScheme()
	setupLog   = ctrl.Log.WithName("setup")
	healthAddr string
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = metal3iov1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func printVersion() {
	setupLog.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	setupLog.Info(fmt.Sprintf("Git commit: %s", version.Commit))
	setupLog.Info(fmt.Sprintf("Build time: %s", version.BuildTime))
	setupLog.Info(fmt.Sprintf("Component: %s", version.String))
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func main() {
	var watchNamespace string
	var devLogging bool
	var imagesBindAddr string
	var imagesPublishAddr string

	// From CAPI point of view, BMO should be able to watch all namespaces
	// in case of a deployment that is not multi-tenant. If the deployment
	// is for multi-tenancy, then the BMO should watch only the provided
	// namespace.
	flag.StringVar(&watchNamespace, "namespace", os.Getenv("WATCH_NAMESPACE"),
		"Namespace that the controller watches to reconcile host resources.")
	flag.StringVar(&imagesBindAddr, "images-bind-addr", ":8084",
		"The address the images endpoint binds to.")
	flag.StringVar(&imagesPublishAddr, "images-publish-addr", "127.0.0.1:8084",
		"The address clients would access the images endpoint from.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(devLogging)))

	printVersion()

	iso := os.Getenv("DEPLOY_ISO")
	if iso == "" {
		setupLog.Info("No DEPLOY_ISO specified")
		os.Exit(1)
	}

	imageServer := imagehandler.NewImageFileServer(ctrl.Log.WithName("ImageFileServer"), iso, imagesPublishAddr)
	// why use a FileServer?
	// 1. it streams files efficiently
	// 2. if we cache these images, then that will be an easy change.
	http.Handle("/", http.FileServer(imageServer.FileSystem()))
	go func() {
		log.Fatal(http.ListenAndServe(imagesBindAddr, nil))
	}()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:    scheme,
		Port:      0, // Add flag with default of 9443 when adding webhooks
		Namespace: watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	imgReconciler := metal3iocontroller.PreprovisioningImageReconciler{
		Client:          mgr.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("PreprovisioningImage"),
		APIReader:       mgr.GetAPIReader(),
		Scheme:          mgr.GetScheme(),
		ImageFileServer: imageServer,
	}
	if err = (&imgReconciler).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PreprovisioningImage")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupChecks(mgr)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
