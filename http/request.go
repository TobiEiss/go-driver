//
// DISCLAIMER
//
// Copyright 2017 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//
// Author Ewout Prangsma
//

package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	driver "github.com/arangodb/go-driver"
)

// httpRequest implements driver.Request using standard golang http requests.
type httpRequest struct {
	method string
	path   string
	q      url.Values
	hdr    map[string]string
	body   []byte
}

// SetQuery sets a single query argument of the request.
// Any existing query argument with the same key is overwritten.
func (r *httpRequest) SetQuery(key, value string) driver.Request {
	if r.q == nil {
		r.q = url.Values{}
	}
	r.q.Set(key, value)
	return r
}

// SetBody sets the content of the request.
// The protocol of the connection determines what kinds of marshalling is taking place.
func (r *httpRequest) SetBody(body interface{}) (driver.Request, error) {
	if data, err := json.Marshal(body); err != nil {
		return r, driver.WithStack(err)
	} else {
		r.body = data
	}
	return r, nil
}

// SetHeader sets a single header arguments of the request.
// Any existing header argument with the same key is overwritten.
func (r *httpRequest) SetHeader(key, value string) driver.Request {
	if r.hdr == nil {
		r.hdr = make(map[string]string)
	}
	r.hdr[key] = value
	return r
}

// createHTTPRequest creates a golang http.Request based on the configured arguments.
func (r *httpRequest) createHTTPRequest(endpoint url.URL) (*http.Request, error) {
	u := endpoint
	u.Path = r.path
	if r.q != nil {
		u.RawQuery = r.q.Encode()
	}
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequest(r.method, u.String(), body)
	if err != nil {
		return nil, driver.WithStack(err)
	}

	if r.hdr != nil {
		for k, v := range r.hdr {
			req.Header.Set(k, v)
		}
	}

	if r.body != nil {
		req.Header.Set("Content-Length", strconv.Itoa(len(r.body)))
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}
