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

package cloud

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	infrav1 "github.com/bartvanbenthem/cluster-api-provider-stackit/api/v1alpha1"
)

func TestCredentialOptions(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string][]byte
		wantErr   bool
		wantCount int
	}{
		{
			name: "service account key only",
			data: map[string][]byte{
				infrav1.StackitSecretKeyServiceAccountKey: []byte(`{"some":"key"}`),
			},
			wantCount: 1,
		},
		{
			name: "service account key with private key",
			data: map[string][]byte{
				infrav1.StackitSecretKeyServiceAccountKey: []byte(`{"some":"key"}`),
				infrav1.StackitSecretKeyPrivateKey:        []byte("-----BEGIN PRIVATE KEY-----"),
			},
			wantCount: 2,
		},
		{
			name: "token only",
			data: map[string][]byte{
				infrav1.StackitSecretKeyToken: []byte("some-token"),
			},
			wantCount: 1,
		},
		{
			name:    "missing both keys",
			data:    map[string][]byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &corev1.Secret{Data: tt.data}
			opts, err := credentialOptions("test-namespace/test-secret", secret)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("credentialOptions() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("credentialOptions() unexpected error: %v", err)
			}
			if len(opts) != tt.wantCount {
				t.Errorf("credentialOptions() returned %d options, want %d", len(opts), tt.wantCount)
			}
		})
	}
}
