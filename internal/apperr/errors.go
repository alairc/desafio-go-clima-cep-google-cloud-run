// Package apperr concentra os erros de domínio que o handler HTTP traduz
// para os status codes definidos no contrato da API.
package apperr

import "errors"

var (
	// ErrInvalidZipcode indica CEP fora do formato de 8 dígitos numéricos (422).
	ErrInvalidZipcode = errors.New("invalid zipcode")

	// ErrZipcodeNotFound indica CEP bem formatado mas inexistente (404).
	ErrZipcodeNotFound = errors.New("can not find zipcode")
)
