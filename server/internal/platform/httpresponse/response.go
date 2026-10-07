// Package httpresponse writes the GIM V1 response envelope.
package httpresponse

import (
	"encoding/json"
	"net/http"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
)

const successMessage = "成功"

// Body is compatible with the V1 web and admin response contract.
type Body struct {
	Code uint32 `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// Success writes a successful business response.
func Success(writer http.ResponseWriter, data any) error {
	return writeJSON(writer, http.StatusOK, Body{
		Code: apperror.CodeOK,
		Msg:  successMessage,
		Data: data,
	})
}

// Failure writes only the public fields of err. Unknown errors are converted
// to a generic HTTP 500 response and never expose err.Error().
func Failure(writer http.ResponseWriter, err error) error {
	status, code, message := apperror.PublicFields(err)
	return writeJSON(writer, status, Body{
		Code: code,
		Msg:  message,
		Data: nil,
	})
}

func writeJSON(writer http.ResponseWriter, status int, body Body) error {
	payload, err := json.Marshal(body)
	if err != nil {
		fallback, _ := json.Marshal(Body{
			Code: apperror.CodeInternal,
			Msg:  "服务器错误",
			Data: nil,
		})
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write(fallback)
		return err
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_, err = writer.Write(payload)
	return err
}
