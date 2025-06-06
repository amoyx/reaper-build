/*
Copyright 2021 The KodeRover Authors.

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

package gerrit

import (
	"reaper-build/tool/httpclient"
)

type HTTPClient struct {
	*httpclient.Client

	host  string
	token string
}

func NewHTTPClient(host, token string) *HTTPClient {
	c := httpclient.New(
		httpclient.SetAuthScheme("Basic"),
		httpclient.SetAuthToken(token),
		httpclient.SetHostURL(host),
	)

	return &HTTPClient{
		Client: c,
		host:   host,
		token:  token,
	}
}
