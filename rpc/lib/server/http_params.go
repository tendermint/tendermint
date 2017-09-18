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

package rpcserver

import (
	"encoding/hex"
	"net/http"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
)

var (
	// Parts of regular expressions
	atom    = "[A-Z0-9!#$%&'*+\\-/=?^_`{|}~]+"
	dotAtom = atom + `(?:\.` + atom + `)*`
	domain  = `[A-Z0-9.-]+\.[A-Z]{2,4}`

	RE_HEX     = regexp.MustCompile(`^(?i)[a-f0-9]+$`)
	RE_EMAIL   = regexp.MustCompile(`^(?i)(` + dotAtom + `)@(` + dotAtom + `)$`)
	RE_ADDRESS = regexp.MustCompile(`^(?i)[a-z0-9]{25,34}$`)
	RE_HOST    = regexp.MustCompile(`^(?i)(` + domain + `)$`)

	//RE_ID12       = regexp.MustCompile(`^[a-zA-Z0-9]{12}$`)
)

func GetParam(r *http.Request, param string) string {
	s := r.URL.Query().Get(param)
	if s == "" {
		s = r.FormValue(param)
	}
	return s
}

func GetParamByteSlice(r *http.Request, param string) ([]byte, error) {
	s := GetParam(r, param)
	return hex.DecodeString(s)
}

func GetParamInt64(r *http.Request, param string) (int64, error) {
	s := GetParam(r, param)
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errors.Errorf(param, err.Error())
	}
	return i, nil
}

func GetParamInt32(r *http.Request, param string) (int32, error) {
	s := GetParam(r, param)
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, errors.Errorf(param, err.Error())
	}
	return int32(i), nil
}

func GetParamUint64(r *http.Request, param string) (uint64, error) {
	s := GetParam(r, param)
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, errors.Errorf(param, err.Error())
	}
	return i, nil
}

func GetParamUint(r *http.Request, param string) (uint, error) {
	s := GetParam(r, param)
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, errors.Errorf(param, err.Error())
	}
	return uint(i), nil
}

func GetParamRegexp(r *http.Request, param string, re *regexp.Regexp) (string, error) {
	s := GetParam(r, param)
	if !re.MatchString(s) {
		return "", errors.Errorf(param, "Did not match regular expression %v", re.String())
	}
	return s, nil
}

func GetParamFloat64(r *http.Request, param string) (float64, error) {
	s := GetParam(r, param)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, errors.Errorf(param, err.Error())
	}
	return f, nil
}
