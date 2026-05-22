package webutil

import (
	"io"
	"net/http"

	"google.golang.org/protobuf/proto"
)

type ValidatedProtoMessage interface {
	proto.Message
	ValidateAll() error
}

type ValidatedNoAuthFN[T any] func(w http.ResponseWriter, r *http.Request, req *T)

func ValidateNoAuth[T any](fn ValidatedNoAuthFN[T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req T

		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteAPIError(w, r, http.StatusBadRequest, "read request body failed", nil)
			return
		}

		pm, ok := any(&req).(ValidatedProtoMessage)
		if !ok {
			panic("internal error: type parameter T must implement proto.Message")
		}

		if err := proto.Unmarshal(body, pm); err != nil {
			WriteAPIError(w, r, http.StatusBadRequest, "invalid request body", nil)
			return
		}

		if err := pm.ValidateAll(); err != nil {
			WriteAPIError(w, r, http.StatusBadRequest, "invalid request parameters", err)
			return
		}

		fn(w, r, &req)
	}
}
