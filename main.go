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
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clusterv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	topologyv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	// +kubebuilder:scaffold:imports
)

const queueControllerName = "queue-controller"
const exchangeControllerName = "exchange-controller"

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1beta1.AddToScheme(scheme)

	_ = topologyv1beta1.AddToScheme(scheme)
	_ = rabbitmqcomv1beta1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     true,
		LeaderElectionID:   "messaging-topology-operator-leader-election",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.QueueReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("Queue"),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(queueControllerName),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", queueControllerName)
		os.Exit(1)
	}
	if err = (&controllers.ExchangeReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("Exchange"),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(exchangeControllerName),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", exchangeControllerName)
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
