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

package controller

import "testing"

func TestServerIDFromProviderID(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		want       string
		wantErr    bool
	}{
		{
			name:       "valid provider ID",
			providerID: "stackit://11111111-1111-1111-1111-111111111111/eu01/22222222-2222-2222-2222-222222222222",
			want:       "22222222-2222-2222-2222-222222222222",
		},
		{
			name:       "missing scheme",
			providerID: "11111111-1111-1111-1111-111111111111/eu01/22222222-2222-2222-2222-222222222222",
			wantErr:    true,
		},
		{
			name:       "too few segments",
			providerID: "stackit://eu01/22222222-2222-2222-2222-222222222222",
			wantErr:    true,
		},
		{
			name:       "empty server id",
			providerID: "stackit://11111111-1111-1111-1111-111111111111/eu01/",
			wantErr:    true,
		},
		{
			name:       "empty string",
			providerID: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serverIDFromProviderID(tt.providerID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("serverIDFromProviderID(%q) error = nil, want error", tt.providerID)
				}
				return
			}
			if err != nil {
				t.Fatalf("serverIDFromProviderID(%q) unexpected error: %v", tt.providerID, err)
			}
			if got != tt.want {
				t.Errorf("serverIDFromProviderID(%q) = %q, want %q", tt.providerID, got, tt.want)
			}
		})
	}
}
