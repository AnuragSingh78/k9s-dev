// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// doctorOptions contains only local invocation parameters. No credentials are logged.
type doctorOptions struct {
	kubeconfig string
	kubeContext string
	timeout time.Duration
}

func doctorCmd() *cobra.Command {
	opts := doctorOptions{timeout: 8 * time.Second}
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect context, API connectivity, and Kubernetes permissions",
		Long:  "Run non-destructive diagnostics of your kubeconfig, active cluster, and effective permissions.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}

	command.Flags().StringVar(&opts.kubeconfig, "kubeconfig", "", "Kubeconfig file (default: KUBECONFIG or ~/.kube/config)")
	command.Flags().StringVar(&opts.kubeContext, "context", "", "Context to inspect (default: current context)")
	command.Flags().DurationVar(&opts.timeout, "timeout", opts.timeout, "Maximum timeout for API calls")

	return command
}

func runDoctor(ctx context.Context, out io.Writer, opts doctorOptions) error {
	if opts.timeout <= 0 {
		return errors.New("timeout must be greater than zero")
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.kubeconfig != "" {
		rules.ExplicitPath = opts.kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if opts.kubeContext != "" {
		overrides.CurrentContext = opts.kubeContext
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	raw, err := clientConfig.RawConfig()
	if err != nil {
		return fmt.Errorf("cannot read kubeconfig: %w", err)
	}

	contextName := opts.kubeContext
	if contextName == "" {
		contextName = raw.CurrentContext
	}
	if contextName == "" {
		return errors.New("no current Kubernetes context; set one using kubectl config use-context or pass --context")
	}
	selected, found := raw.Contexts[contextName]
	if !found {
		return fmt.Errorf("Kubernetes context %q was not found in kubeconfig", contextName)
	}

	namespace := selected.Namespace
	if namespace == "" {
		namespace = "default"
	}
	fmt.Fprintln(out, "K9s Dev · Cluster Doctor")
	fmt.Fprintf(out, "Context:   %s\nCluster:   %s\nNamespace: %s\n", contextName, selected.Cluster, namespace)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("cannot prepare Kubernetes API connection: %w", err)
	}
	restConfig.Timeout = opts.timeout
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("cannot initialize Kubernetes client: %w", err)
	}

	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("Kubernetes API connection failed: %w", err)
	}
	fmt.Fprintf(out, "API server: reachable (%s)\n", version.GitVersion)
	fmt.Fprintln(out, "\nEffective access (RBAC checks):")

	type accessCheck struct {
		label string
		verb string
		resource string
		subresource string
		namespace string
	}
	checks := []accessCheck{
		{label: "List pods", verb: "list", resource: "pods", namespace: namespace},
		{label: "Read pod logs", verb: "get", resource: "pods", subresource: "log", namespace: namespace},
		{label: "Exec into pods", verb: "create", resource: "pods", subresource: "exec", namespace: namespace},
		{label: "List nodes", verb: "list", resource: "nodes"},
		{label: "Delete pods", verb: "delete", resource: "pods", namespace: namespace},
	}
	for _, check := range checks {
		reqCtx, cancel := context.WithTimeout(ctx, opts.timeout)
		response, checkErr := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(
			reqCtx,
			&authorizationv1.SelfSubjectAccessReview{
				Spec: authorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Namespace: check.namespace,
						Verb: check.verb,
						Resource: check.resource,
						Subresource: check.subresource,
					},
				},
			},
			metav1.CreateOptions{},
		)
		cancel()
		status := "UNKNOWN"
		switch {
		case checkErr != nil:
			// Some clusters disallow SelfSubjectAccessReview. Report uncertainty,
			// never confuse it with an explicit RBAC denial.
			status = "UNKNOWN (permission probe unavailable)"
		case response.Status.Allowed:
			status = "ALLOWED"
		case response.Status.Denied:
			status = "DENIED"
		default:
			status = "NOT ALLOWED"
		}
		fmt.Fprintf(out, "  %-18s %s\n", check.label+":", status)
	}
	fmt.Fprintln(out, "\nNo Kubernetes workloads were changed. Permissions reflect the active credentials.")
	return nil
}

