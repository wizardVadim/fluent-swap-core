package websocket

import "errors"

var (
	errRequestIDNotFound       = errors.New("request id not found")
	errTypeIsEmpty             = errors.New("type is empty")
	errValueIsEmpty            = errors.New("some value is empty")
	errCannotFindClientSession = errors.New("cannot find client's session")
)
