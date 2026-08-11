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
	"errors"

	"github.com/stackitcloud/stackit-sdk-go/core/oapierror"
)

// IsNotFound reports whether err is a STACKIT API error with HTTP status 404,
// meaning the resource referenced in the request no longer (or does not yet) exist.
func IsNotFound(err error) bool {
	var apiErr *oapierror.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		return apiErr.GetStatusCode() == 404
	}
	return false
}
