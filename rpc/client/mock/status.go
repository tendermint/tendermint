// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mock

import (
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// StatusMock returns the result specified by the Call
type StatusMock struct {
	Call
}

func (m *StatusMock) _assertStatusClient() client.StatusClient {
	return m
}

func (m *StatusMock) Status() (*ctypes.ResultStatus, error) {
	res, err := m.GetResponse(nil)
	if err != nil {
		return nil, err
	}
	return res.(*ctypes.ResultStatus), nil
}

// StatusRecorder can wrap another type (StatusMock, full client)
// and record the status calls
type StatusRecorder struct {
	Client client.StatusClient
	Calls  []Call
}

func NewStatusRecorder(client client.StatusClient) *StatusRecorder {
	return &StatusRecorder{
		Client: client,
		Calls:  []Call{},
	}
}

func (r *StatusRecorder) _assertStatusClient() client.StatusClient {
	return r
}

func (r *StatusRecorder) addCall(call Call) {
	r.Calls = append(r.Calls, call)
}

func (r *StatusRecorder) Status() (*ctypes.ResultStatus, error) {
	res, err := r.Client.Status()
	r.addCall(Call{
		Name:     "status",
		Response: res,
		Error:    err,
	})
	return res, err
}
