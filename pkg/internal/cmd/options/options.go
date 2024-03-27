/*
Copyright 2021 The cert-manager Authors.

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

package options

import (
	"flag"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/cert-manager/approver-policy/pkg/approver"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Options are the main options for the approver-policy. Populated via
// processing command line flags.
type Options struct {
	// logLevel is the verbosity level the driver will write logs at.
	logLevel string

	// kubeConfigFlags is used for generating a Kubernetes rest config via CLI
	// flags.
	kubeConfigFlags *genericclioptions.ConfigFlags

	// MetricsAddress is the TCP address for exposing HTTP Prometheus metrics
	// which will be served on the HTTP path '/metrics'. The value "0" will
	// disable exposing metrics.
	MetricsAddress string

	// LeaderElectionNamespace is the Namespace to lease the controller replica
	// leadership election.
	LeaderElectionNamespace string

	// ReadyzAddress is the TCP address for exposing the HTTP readiness probe
	// which will be served on the HTTP path '/readyz'.
	ReadyzAddress string

	// RestConfig is the shared base rest config to connect to the Kubernetes
	// API.
	RestConfig *rest.Config

	// Webhook are options specific to the Kubernetes Webhook.
	Webhook

	// Logr is the shared base logger.
	Logr logr.Logger
}

// Webhook holds options specific to running the approver-policy Webhook
// service.
type Webhook struct {
	// Host is the host that the Webhook will be served on.
	Host string

	// Port is the TCP port that the Webhook will be served on.
	Port int

	// ServiceName is the service that exposes the Webhook server.
	ServiceName string

	// CASecretNamespace is the namespace that the
	// cert-manager-approver-policy-tls Secret is stored.
	CASecretNamespace string
}

func New() *Options {
	return new(Options)
}

func (o *Options) Prepare(cmd *cobra.Command, approvers ...approver.Interface) *Options {
	o.addFlags(cmd, approvers...)
	return o
}

func (o *Options) Complete() error {
	klog.InitFlags(nil)
	log := klogr.New()
	flag.Set("v", o.logLevel)
	o.Logr = log

	var err error
	o.RestConfig, err = o.kubeConfigFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to build kubernetes rest config: %s", err)
	}

	return nil
}

func (o *Options) addFlags(cmd *cobra.Command, approvers ...approver.Interface) {
	var nfs cliflag.NamedFlagSets

	o.addAppFlags(nfs.FlagSet("App"))
	o.addWebhookFlags(nfs.FlagSet("Webhook"))
	o.kubeConfigFlags = genericclioptions.NewConfigFlags(true)
	o.kubeConfigFlags.AddFlags(nfs.FlagSet("Kubernetes"))

	for _, approver := range approvers {
		approver.RegisterFlags(nfs.FlagSet(approver.Name()))
	}

	usageFmt := "Usage:\n  %s\n"
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), nfs, 0)
		return nil
	})

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), nfs, 0)
	})

	fs := cmd.Flags()
	for _, f := range nfs.FlagSets {
		fs.AddFlagSet(f)
	}
}

func (o *Options) addAppFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.logLevel, "log-level", "v", "1",
		"Log level (1-5).")

	fs.StringVar(&o.LeaderElectionNamespace, "leader-election-namespace", "",
		"Namespace to lease leader election for controller replica set.")

	fs.StringVar(&o.MetricsAddress, "metrics-bind-address", ":9402",
		`TCP address for exposing HTTP Prometheus metrics which will be served on the HTTP path '/metrics'. The value "0" will
	 disable exposing metrics.`)

	fs.StringVar(&o.ReadyzAddress, "readiness-probe-bind-address", ":6060",
		"TCP address for exposing the HTTP readiness probe which will be served on the HTTP path '/readyz'.")
}

func (o *Options) addWebhookFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Webhook.Host,
		"webhook-host", "0.0.0.0",
		"Host to serve webhook.")

	fs.IntVar(&o.Webhook.Port,
		"webhook-port", 6443,
		"Port to serve webhook.")

	fs.StringVar(&o.Webhook.ServiceName,
		"webhook-service-name", "cert-manager-approver-policy",
		"Name of the Kubernetes Service that exposes the Webhook's server.")

	fs.StringVar(&o.Webhook.CASecretNamespace,
		"webhook-ca-secret-namespace", "cert-manager",
		"Namespace that the cert-manager-approver-policy-tls Secret is stored.")

	var deprecatedCertDir string
	fs.StringVar(&deprecatedCertDir,
		"webhook-certificate-dir", "/tmp",
		"Directory where the Webhook certificate and private key are located. "+
			"Certificate and private key must be named 'tls.crt' and 'tls.key' "+
			"respectively.")

	fs.MarkDeprecated("webhook-certificate-dir", "webhook-certificate-dir is deprecated")
}
