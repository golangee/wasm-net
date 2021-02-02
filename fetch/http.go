// Copyright 2020 Torben Schinke
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fetch

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"runtime/debug"
)

// GlobalPanicHandler is introduced to remove a dependency to the dom package and avoids halting
// the entire wasm process if an internally spawned goroutine panics.
var GlobalPanicHandler = func() { //nolint:gochecknoglobals
	r := recover()
	if r == nil {
		return
	}

	log.Println(r, string(debug.Stack()))
}

// Get performs a simple http.Get (Fetch) and returns the response.
func Get(url string, f func(res *http.Response, err error)) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		f(nil, err)

		return
	}

	Request(http.DefaultClient, req, f)
}

// Request is the generic http client implementation which allows custom requests. The current implementation spawns a
// new goroutine for each request, but the callback is guaranteed not to race with the UI or DOM Thread. However,
// the only guarantee is, that it does not deadlock.
func Request(client *http.Client, request *http.Request, f func(res *http.Response, err error)) {
	go func() {
		defer GlobalPanicHandler()

		res, err := client.Do(request)
		if err == nil {
			defer res.Body.Close() //nolint:errcheck
		}

		// in a "normal" context, this would be a simple way to introduce data races, however the Go wasm
		// implementation is currently only single threaded and even if that would not be the case
		// in the future anymore, it is still unclear how we will evolve, perhaps directly using fetch
		// instead of doing this kind of complex (and broken) roundtrip.
		f(res, err)
	}()
}

// AsText is a middleware for an async http response.
func AsText(f func(res string, err error)) func(res *http.Response, err error) {
	return func(res *http.Response, err error) {
		if err != nil {
			f("", err)

			return
		}

		buf, err := ioutil.ReadAll(res.Body)
		if err != nil {
			f("", err)

			return
		}

		f(string(buf), nil)
	}
}

// AsJSON tries to unmarshal into given v and invokes the callback afterwards. The callback is always invoked
// and if the err is nil, the given interface has been populated successfully. Example:
//   type MyType struct{
//     SomeField string
//   }
//
//   var myType MyType
//   Get("http://...", AsJSON(&myType, func(err error)) {
//      if err != nil {
//         return
//      }
//
//      // do something with myType, but note that this is an async call
//	 })
func AsJSON(v interface{}, f func(err error)) func(res *http.Response, err error) {
	return func(res *http.Response, err error) {
		if err != nil {
			f(err)

			return
		}

		buf, err := ioutil.ReadAll(res.Body)
		if err != nil {
			f(err)

			return
		}

		if err := json.Unmarshal(buf, v); err != nil {
			f(err)

			return
		}

		f(nil) // success case
	}
}
