package main

import (
	"context"
	"runtime"

	sdk "github.com/operator-framework/operator-sdk/pkg/sdk"
	k8sutil "github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	stub "github.com/srleyva/vault-operator/pkg/stub"

	"github.com/sirupsen/logrus"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func main() {
	printVersion()

	resource := "vault.sleyva.com/v1alpha1"
	kinds := []string{"Consul", "Vault"}
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("Failed to get watch namespace: %v", err)
	}
	resyncPeriod := 5
	for _, kind := range kinds {
		logrus.Infof("Watching %s, %s, %s, %d", resource, kind, namespace, resyncPeriod)
		sdk.Watch(resource, kind, namespace, resyncPeriod)
	}
	sdk.Handle(stub.NewHandler())
	sdk.Run(context.TODO())
}
