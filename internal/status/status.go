/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package status

import (
	"errors"
	"fmt"
)

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	perrors "github.com/pkg/errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	spb "google.golang.org/genproto/googleapis/rpc/status"
)

import (
	"github.com/dubbogo/triple/internal/codes"
)

// TripleError wraps a pointer of a status proto. It implements error and Status,
// and a nil *Error should never be returned by this package.
type TripleError struct {
	e *Status
}

func (e *TripleError) Error() string {
	return fmt.Sprintf("rpc error: codes = %+v desc = %v", codes.Code(e.e.s.GetCode()), e.e.s.GetMessage())
}

func (e *TripleError) Status() *Status {
	return e.e
}
func IsTripleError(err error) bool {
	_, ok := err.(*TripleError)
	return ok
}

// Errorf returns New Error with stackTraces from now
func Errorf(c codes.Code, format string, a ...interface{}) *TripleError {
	// create trace
	newErrorf := perrors.Errorf(format, a...)
	return FromError(c, newErrorf)
}

// FromError deal with user level error, which have recorded user codes' trace detail
func FromError(code codes.Code, err error) *TripleError {
	newStatus := NewStatus(code, err.Error())
	newStatusWithDetail, err := newStatus.WithDetails(&errdetails.DebugInfo{
		StackEntries: []string{
			fmt.Sprintf("%+v", err),
		},
	})
	if err != nil {
		return &TripleError{
			e: newStatus,
		}
	}
	return &TripleError{
		e: newStatusWithDetail,
	}
}

// Status represents an RPC status codes, message, and details.  It is immutable
// and should be created with New, Newf, or FromProto.
type Status struct {
	s *spb.Status
}

// NewStatus returns a Status representing c and msg.
func NewStatus(c codes.Code, msg string) *Status {
	return &Status{s: &spb.Status{Code: int32(c), Message: msg}}
}

// FromProto returns a Status representing s.
func FromProto(s *spb.Status) *Status {
	return &Status{s: proto.Clone(s).(*spb.Status)}
}

// Code returns the status codes contained in s.
func (s *Status) Code() codes.Code {
	if s == nil || s.s == nil {
		return codes.OK
	}
	return codes.Code(s.s.Code)
}

// Message returns the message contained in s.
func (s *Status) Message() string {
	if s == nil || s.s == nil {
		return ""
	}
	return s.s.Message
}

// Proto returns s's status as an spb.Status proto message.
func (s *Status) Proto() *spb.Status {
	if s == nil {
		return nil
	}
	return proto.Clone(s.s).(*spb.Status)
}

// WithDetails returns a new status with the provided details messages appended to the status.
// If any errors are encountered, it returns nil and the first error encountered.
func (s *Status) WithDetails(details ...proto.Message) (*Status, error) {
	if s.Code() == codes.OK {
		return nil, errors.New("no error details for status with codes OK")
	}
	// s.Code() != OK implies that s.Proto() != nil.
	p := s.Proto()
	for _, detail := range details {
		any, err := ptypes.MarshalAny(detail)
		if err != nil {
			return nil, err
		}
		p.Details = append(p.Details, any)
	}
	return &Status{s: p}, nil
}

// Details returns a slice of details messages attached to the status.
// If a detail cannot be decoded, the error is returned in place of the detail.
func (s *Status) Details() []interface{} {
	if s == nil || s.s == nil {
		return nil
	}
	details := make([]interface{}, 0, len(s.s.Details))
	for _, any := range s.s.Details {
		detail := &ptypes.DynamicAny{}
		if err := ptypes.UnmarshalAny(any, detail); err != nil {
			details = append(details, err)
			continue
		}
		details = append(details, detail.Message)
	}
	return details
}
