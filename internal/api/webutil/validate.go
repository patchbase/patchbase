package webutil

import (
	"errors"
	"io"
	"net/http"

	"go.patchbase.net/server/internal/apperr"
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
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				WriteError(w, r, apperr.ErrBodyTooLarge)
				return
			}
			WriteError(w, r, apperr.ErrBodyReadFailed)
			return
		}

		pm, ok := any(&req).(ValidatedProtoMessage)
		if !ok {
			panic("internal error: type parameter T must implement proto.Message")
		}

		if err := proto.Unmarshal(body, pm); err != nil {
			WriteError(w, r, apperr.ErrInvalidBody)
			return
		}

		if err := pm.ValidateAll(); err != nil {
			WriteError(w, r, apperr.WithDetails(apperr.ErrInvalidParams, err))
			return
		}

		fn(w, r, &req)
	}
}
