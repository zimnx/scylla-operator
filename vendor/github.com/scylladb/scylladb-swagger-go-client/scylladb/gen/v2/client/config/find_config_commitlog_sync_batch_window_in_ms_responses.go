// Code generated by go-swagger; DO NOT EDIT.

package config

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/scylladb/scylladb-swagger-go-client/scylladb/gen/v2/models"
)

// FindConfigCommitlogSyncBatchWindowInMsReader is a Reader for the FindConfigCommitlogSyncBatchWindowInMs structure.
type FindConfigCommitlogSyncBatchWindowInMsReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *FindConfigCommitlogSyncBatchWindowInMsReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewFindConfigCommitlogSyncBatchWindowInMsOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewFindConfigCommitlogSyncBatchWindowInMsDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewFindConfigCommitlogSyncBatchWindowInMsOK creates a FindConfigCommitlogSyncBatchWindowInMsOK with default headers values
func NewFindConfigCommitlogSyncBatchWindowInMsOK() *FindConfigCommitlogSyncBatchWindowInMsOK {
	return &FindConfigCommitlogSyncBatchWindowInMsOK{}
}

/*
FindConfigCommitlogSyncBatchWindowInMsOK handles this case with default header values.

Config value
*/
type FindConfigCommitlogSyncBatchWindowInMsOK struct {
	Payload int64
}

func (o *FindConfigCommitlogSyncBatchWindowInMsOK) GetPayload() int64 {
	return o.Payload
}

func (o *FindConfigCommitlogSyncBatchWindowInMsOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewFindConfigCommitlogSyncBatchWindowInMsDefault creates a FindConfigCommitlogSyncBatchWindowInMsDefault with default headers values
func NewFindConfigCommitlogSyncBatchWindowInMsDefault(code int) *FindConfigCommitlogSyncBatchWindowInMsDefault {
	return &FindConfigCommitlogSyncBatchWindowInMsDefault{
		_statusCode: code,
	}
}

/*
FindConfigCommitlogSyncBatchWindowInMsDefault handles this case with default header values.

unexpected error
*/
type FindConfigCommitlogSyncBatchWindowInMsDefault struct {
	_statusCode int

	Payload *models.ErrorModel
}

// Code gets the status code for the find config commitlog sync batch window in ms default response
func (o *FindConfigCommitlogSyncBatchWindowInMsDefault) Code() int {
	return o._statusCode
}

func (o *FindConfigCommitlogSyncBatchWindowInMsDefault) GetPayload() *models.ErrorModel {
	return o.Payload
}

func (o *FindConfigCommitlogSyncBatchWindowInMsDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorModel)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (o *FindConfigCommitlogSyncBatchWindowInMsDefault) Error() string {
	return fmt.Sprintf("agent [HTTP %d] %s", o._statusCode, strings.TrimRight(o.Payload.Message, "."))
}