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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"

	driver "github.com/arangodb/go-driver"
)

// httpResponse implements driver.Response for standard golang http responses.
type httpResponse struct {
	resp        *http.Response
	rawResponse *[]byte
	body        map[string]*json.RawMessage
}

// StatusCode returns an HTTP compatible status code of the response.
func (r *httpResponse) StatusCode() int {
	return r.resp.StatusCode
}

// CheckStatus checks if the status of the response equals to one of the given status codes.
// If so, nil is returned.
// If not, an attempt is made to parse an error response in the body and an error is returned.
func (r *httpResponse) CheckStatus(validStatusCodes ...int) error {
	for _, x := range validStatusCodes {
		if x == r.resp.StatusCode {
			// Found valid status code
			return nil
		}
	}
	// Invalid status code, try to parse arango error response.
	var aerr driver.ArangoError
	if err := r.ParseBody("", &aerr); err == nil {
		// Found correct arango error.
		return aerr
	}

	// We do not have a valid error code, so we can only create one based on the HTTP status code.
	return driver.ArangoError{
		HasError:     true,
		Code:         r.resp.StatusCode,
		ErrorMessage: fmt.Sprintf("Unexpected status code %d", r.resp.StatusCode),
	}
}

// ParseBody performs protocol specific unmarshalling of the response data into the given result.
// If the given field is non-empty, the contents of that field will be parsed into the given result.
func (r *httpResponse) ParseBody(field string, result interface{}) error {
	if r.body == nil {
		body := r.resp.Body
		defer body.Close()
		content, err := ioutil.ReadAll(body)
		if err != nil {
			return driver.WithStack(err)
		}
		if r.rawResponse != nil {
			*r.rawResponse = content
		}
		bodyMap := make(map[string]*json.RawMessage)
		if err := json.Unmarshal(content, &bodyMap); err != nil {
			return driver.WithStack(err)
		}
		r.body = bodyMap
	}
	if field != "" {
		// Unmarshal only a specific field
		raw, ok := r.body[field]
		if !ok || raw == nil {
			// Field not found, silently ignored
			return nil
		}
		// Unmarshal field
		if err := json.Unmarshal(*raw, result); err != nil {
			return driver.WithStack(err)
		}
		return nil
	}
	// Unmarshal entire body
	rv := reflect.ValueOf(result)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(result)}
	}
	objValue := rv.Elem()
	switch objValue.Kind() {
	case reflect.Struct:
		if err := decodeObjectFields(objValue, r.body); err != nil {
			return driver.WithStack(err)
		}
	case reflect.Map:
		if err := decodeMapFields(objValue, r.body); err != nil {
			return driver.WithStack(err)
		}
	default:
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(result)}
	}
	return nil
}

// decodeObjectFields decodes fields from the given body into a objValue of kind struct.
func decodeObjectFields(objValue reflect.Value, body map[string]*json.RawMessage) error {
	objValueType := objValue.Type()
	for i := 0; i != objValue.NumField(); i++ {
		f := objValueType.Field(i)
		if f.Anonymous {
			// Recurse into fields of anonymous field
			if err := decodeObjectFields(objValue.Field(i), body); err != nil {
				return driver.WithStack(err)
			}
		} else {
			// Decode individual field
			jsonName := strings.Split(f.Tag.Get("json"), ",")[0]
			if jsonName == "" {
				jsonName = f.Name
			} else if jsonName == "-" {
				continue
			}
			raw, ok := body[jsonName]
			if ok && raw != nil {
				field := objValue.Field(i)
				if err := json.Unmarshal(*raw, field.Addr().Interface()); err != nil {
					return driver.WithStack(err)
				}
			}
		}
	}
	return nil
}

// decodeMapFields decodes fields from the given body into a mapValue of kind map.
func decodeMapFields(val reflect.Value, body map[string]*json.RawMessage) error {
	mapVal := val
	if mapVal.IsNil() {
		valType := val.Type()
		mapType := reflect.MapOf(valType.Key(), valType.Elem())
		mapVal = reflect.MakeMap(mapType)
	}
	for jsonName, raw := range body {
		var value interface{}
		if raw != nil {
			if err := json.Unmarshal(*raw, &value); err != nil {
				return driver.WithStack(err)
			}
		}
		mapVal.SetMapIndex(reflect.ValueOf(jsonName), reflect.ValueOf(value))
	}
	val.Set(mapVal)
	return nil
}
