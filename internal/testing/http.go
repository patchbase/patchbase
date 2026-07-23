// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package testing

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/samber/do/v2"
)

type HTTPRequestOption func(r *http.Request)

func WithHeader(key string, value string) HTTPRequestOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

func WithBearerToken(token string) HTTPRequestOption {
	return WithHeader("Authorization", "Bearer "+token)
}

func (b *Backend) HTTPGet(path string, opts ...HTTPRequestOption) *httptest.ResponseRecorder {
	return b.doHTTPRequest(http.MethodGet, path, nil, opts...)
}

func (b *Backend) HTTPPost(path string, body string, opts ...HTTPRequestOption) *httptest.ResponseRecorder {
	return b.doHTTPRequest(http.MethodPost, path, strings.NewReader(body), opts...)
}

func (b *Backend) HTTPPostBytes(path string, body []byte, opts ...HTTPRequestOption) *httptest.ResponseRecorder {
	return b.doHTTPRequest(http.MethodPost, path, bytes.NewReader(body), opts...)
}

func (b *Backend) HTTPPut(path string, body string, opts ...HTTPRequestOption) *httptest.ResponseRecorder {
	return b.doHTTPRequest(http.MethodPut, path, strings.NewReader(body), opts...)
}

func (b *Backend) HTTPPatch(path string, body string, opts ...HTTPRequestOption) *httptest.ResponseRecorder {
	return b.doHTTPRequest(http.MethodPatch, path, strings.NewReader(body), opts...)
}

func (b *Backend) HTTPDelete(path string, opts ...HTTPRequestOption) *httptest.ResponseRecorder {
	return b.doHTTPRequest(http.MethodDelete, path, nil, opts...)
}

func (b *Backend) doHTTPRequest(method string, path string, body io.Reader, opts ...HTTPRequestOption) *httptest.ResponseRecorder {
	mux := do.MustInvoke[*http.ServeMux](b.injector)
	request := httptest.NewRequest(method, path, body)
	for _, opt := range opts {
		opt(request)
	}

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}
