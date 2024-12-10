//go:build js && wasm

// Since net/http is currently not supported in TinyGo, we have
// to implement our own Fetch operation.
// Since Go's net/http already has a perfectly functioning WASM implementation,
// we can reuse the code and remove everything we don't need

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the GO_LICENSE file.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"syscall/js"
)

var uint8Array = js.Global().Get("Uint8Array")

// Implements fetch for WebAssembly using Fetch API
func Fetch(ctx context.Context, url string) (*FetchResponse, error) {
	ac := js.Global().Get("AbortController")
	if !ac.IsUndefined() {
		// Some browsers that support WASM don't necessarily support
		// the AbortController. See
		// https://developer.mozilla.org/en-US/docs/Web/API/AbortController#Browser_compatibility.
		ac = ac.New()
	}

	opt := js.Global().Get("Object").New()

	fetchPromise := js.Global().Call("fetch", url, opt)
	var (
		respCh           = make(chan *FetchResponse, 1)
		errCh            = make(chan error, 1)
		success, failure js.Func
	)
	success = js.FuncOf(func(this js.Value, args []js.Value) any {
		success.Release()
		failure.Release()

		result := args[0]

		b := result.Get("body")
		var body io.ReadCloser
		// The body is undefined when the browser does not support streaming response bodies (Firefox),
		// and null in certain error cases, i.e. when the request is blocked because of CORS settings.
		if !b.IsUndefined() && !b.IsNull() {
			body = &streamReader{stream: b.Call("getReader")}
		} else {
			// Fall back to using ArrayBuffer
			// https://developer.mozilla.org/en-US/docs/Web/API/Body/arrayBuffer
			body = &arrayReader{arrayPromise: result.Call("arrayBuffer")}
		}

		code := result.Get("status").Int()

		respCh <- &FetchResponse{
			StatusCode: code,
			Body:       body,
		}

		return nil
	})
	failure = js.FuncOf(func(this js.Value, args []js.Value) any {
		success.Release()
		failure.Release()

		err := args[0]
		// The error is a JS Error type
		// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Error
		// We can use the toString() method to get a string representation of the error.
		errMsg := err.Call("toString").String()
		// Errors can optionally contain a cause.
		if cause := err.Get("cause"); !cause.IsUndefined() {
			// The exact type of the cause is not defined,
			// but if it's another error, we can call toString() on it too.
			if !cause.Get("toString").IsUndefined() {
				errMsg += ": " + cause.Call("toString").String()
			} else if cause.Type() == js.TypeString {
				errMsg += ": " + cause.String()
			}
		}
		errCh <- fmt.Errorf("net/http: fetch() failed: %s", errMsg)
		return nil
	})

	fetchPromise.Call("then", success, failure)
	select {
	case <-ctx.Done():
		if !ac.IsUndefined() {
			// Abort the Fetch request.
			ac.Call("abort")
		}
		return nil, ctx.Err()
	case resp := <-respCh:
		return resp, nil
	case err := <-errCh:
		return nil, err
	}
}

var errClosed = errors.New("net/http: reader is closed")

// streamReader implements an io.ReadCloser wrapper for ReadableStream.
// See https://fetch.spec.whatwg.org/#readablestream for more information.
type streamReader struct {
	pending []byte
	stream  js.Value
	err     error // sticky read error
}

func (r *streamReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	if len(r.pending) == 0 {
		var (
			bCh   = make(chan []byte, 1)
			errCh = make(chan error, 1)
		)
		success := js.FuncOf(func(this js.Value, args []js.Value) any {
			result := args[0]
			if result.Get("done").Bool() {
				errCh <- io.EOF
				return nil
			}
			value := make([]byte, result.Get("value").Get("byteLength").Int())
			js.CopyBytesToGo(value, result.Get("value"))
			bCh <- value
			return nil
		})
		defer success.Release()
		failure := js.FuncOf(func(this js.Value, args []js.Value) any {
			// Assumes it's a TypeError. See
			// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/TypeError
			// for more information on this type. See
			// https://streams.spec.whatwg.org/#byob-reader-read for the spec on
			// the read method.
			errCh <- errors.New(args[0].Get("message").String())
			return nil
		})
		defer failure.Release()
		r.stream.Call("read").Call("then", success, failure)
		select {
		case b := <-bCh:
			r.pending = b
		case err := <-errCh:
			r.err = err
			return 0, err
		}
	}
	n = copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *streamReader) Close() error {
	// This ignores any error returned from cancel method. So far, I did not encounter any concrete
	// situation where reporting the error is meaningful. Most users ignore error from resp.Body.Close().
	// If there's a need to report error here, it can be implemented and tested when that need comes up.
	r.stream.Call("cancel")
	if r.err == nil {
		r.err = errClosed
	}
	return nil
}

// arrayReader implements an io.ReadCloser wrapper for ArrayBuffer.
// https://developer.mozilla.org/en-US/docs/Web/API/Body/arrayBuffer.
type arrayReader struct {
	arrayPromise js.Value
	pending      []byte
	read         bool
	err          error // sticky read error
}

func (r *arrayReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	if !r.read {
		r.read = true
		var (
			bCh   = make(chan []byte, 1)
			errCh = make(chan error, 1)
		)
		success := js.FuncOf(func(this js.Value, args []js.Value) any {
			// Wrap the input ArrayBuffer with a Uint8Array
			uint8arrayWrapper := uint8Array.New(args[0])
			value := make([]byte, uint8arrayWrapper.Get("byteLength").Int())
			js.CopyBytesToGo(value, uint8arrayWrapper)
			bCh <- value
			return nil
		})
		defer success.Release()
		failure := js.FuncOf(func(this js.Value, args []js.Value) any {
			// Assumes it's a TypeError. See
			// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/TypeError
			// for more information on this type.
			// See https://fetch.spec.whatwg.org/#concept-body-consume-body for reasons this might error.
			errCh <- errors.New(args[0].Get("message").String())
			return nil
		})
		defer failure.Release()
		r.arrayPromise.Call("then", success, failure)
		select {
		case b := <-bCh:
			r.pending = b
		case err := <-errCh:
			return 0, err
		}
	}
	if len(r.pending) == 0 {
		return 0, io.EOF
	}
	n = copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *arrayReader) Close() error {
	if r.err == nil {
		r.err = errClosed
	}
	return nil
}
