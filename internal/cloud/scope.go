/*
Copyright 2026.

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

// Package cloud wraps the STACKIT IaaS SDK (services/iaas/v2api) with the
// operations needed by the StackitCluster/StackitMachine controllers.
package cloud

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stackitcloud/stackit-sdk-go/core/config"
	iaas "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"

	infrav1 "github.com/bartvanbenthem/cluster-api-provider-stackit/api/v1alpha1"
)

// Scope bundles an authenticated STACKIT IaaS API client with the project/region a
// StackitCluster's resources live in. It is built fresh at the start of every
// reconcile so that credential rotation (a new Secret) takes effect immediately.
type Scope struct {
	// API is the STACKIT IaaS API surface used by every operation in this package.
	// It is declared as the SDK's DefaultAPI interface (not the concrete client) so
	// tests can supply a fake implementation.
	API iaas.DefaultAPI

	ProjectID string
	Region    string
}

// NewScope builds a Scope for the given StackitCluster. If the cluster sets
// spec.identityRef, credentials are read from the referenced StackitClusterIdentity's
// Secret; otherwise the manager's ambient STACKIT credentials (environment variables
// or the default credentials file) are used, matching the SDK's default discovery.
func NewScope(ctx context.Context, c client.Client, cluster *infrav1.StackitCluster) (*Scope, error) {
	opts := []config.ConfigurationOption{config.WithRegion(cluster.Spec.Region)}

	if cluster.Spec.IdentityRef != nil {
		identity := &infrav1.StackitClusterIdentity{}
		identityKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.IdentityRef.Name}
		if err := c.Get(ctx, identityKey, identity); err != nil {
			return nil, fmt.Errorf("getting StackitClusterIdentity %q: %w", identityKey, err)
		}

		secret := &corev1.Secret{}
		secretKey := client.ObjectKey{Namespace: cluster.Namespace, Name: identity.Spec.SecretRef.Name}
		if err := c.Get(ctx, secretKey, secret); err != nil {
			return nil, fmt.Errorf("getting credentials secret %q: %w", secretKey, err)
		}

		credOpts, err := credentialOptions(secretKey.String(), secret)
		if err != nil {
			return nil, err
		}
		opts = append(opts, credOpts...)
	}

	apiClient, err := iaas.NewAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating STACKIT IaaS API client: %w", err)
	}

	return &Scope{
		API:       apiClient.DefaultAPI,
		ProjectID: cluster.Spec.ProjectID,
		Region:    cluster.Spec.Region,
	}, nil
}

func credentialOptions(secretName string, secret *corev1.Secret) ([]config.ConfigurationOption, error) {
	switch {
	case len(secret.Data[infrav1.StackitSecretKeyServiceAccountKey]) > 0:
		opts := []config.ConfigurationOption{
			config.WithServiceAccountKey(string(secret.Data[infrav1.StackitSecretKeyServiceAccountKey])),
		}
		if pk := secret.Data[infrav1.StackitSecretKeyPrivateKey]; len(pk) > 0 {
			opts = append(opts, config.WithPrivateKey(string(pk)))
		}
		return opts, nil
	case len(secret.Data[infrav1.StackitSecretKeyToken]) > 0:
		return []config.ConfigurationOption{
			config.WithToken(string(secret.Data[infrav1.StackitSecretKeyToken])),
		}, nil
	default:
		return nil, fmt.Errorf("credentials secret %q has neither %q nor %q data key set",
			secretName, infrav1.StackitSecretKeyServiceAccountKey, infrav1.StackitSecretKeyToken)
	}
}
