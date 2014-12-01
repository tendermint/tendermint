package rpc

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	. "github.com/tendermint/tendermint/common"
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

func panicAPI(err error) {
	panic(APIResponse{API_INVALID_PARAM, err.Error()})
}

func GetParam(r *http.Request, param string) string {
	s := r.URL.Query().Get(param)
	if s == "" {
		s = r.FormValue(param)
	}
	return s
}

func GetParamInt64Safe(r *http.Request, param string) (int64, error) {
	s := GetParam(r, param)
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, Errorf(param, err.Error())
	}
	return i, nil
}
func GetParamInt64(r *http.Request, param string) int64 {
	i, err := GetParamInt64Safe(r, param)
	if err != nil {
		panicAPI(err)
	}
	return i
}

func GetParamInt32Safe(r *http.Request, param string) (int32, error) {
	s := GetParam(r, param)
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, Errorf(param, err.Error())
	}
	return int32(i), nil
}
func GetParamInt32(r *http.Request, param string) int32 {
	i, err := GetParamInt32Safe(r, param)
	if err != nil {
		panicAPI(err)
	}
	return i
}

func GetParamUint64Safe(r *http.Request, param string) (uint64, error) {
	s := GetParam(r, param)
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, Errorf(param, err.Error())
	}
	return i, nil
}
func GetParamUint64(r *http.Request, param string) uint64 {
	i, err := GetParamUint64Safe(r, param)
	if err != nil {
		panicAPI(err)
	}
	return i
}

func GetParamRegexpSafe(r *http.Request, param string, re *regexp.Regexp) (string, error) {
	s := GetParam(r, param)
	if !re.MatchString(s) {
		return "", Errorf(param, "Did not match regular expression %v", re.String())
	}
	return s, nil
}
func GetParamRegexp(r *http.Request, param string, re *regexp.Regexp, required bool) string {
	s, err := GetParamRegexpSafe(r, param, re)
	if (required || s != "") && err != nil {
		panicAPI(err)
	}
	return s
}

func GetParamFloat64Safe(r *http.Request, param string) (float64, error) {
	s := GetParam(r, param)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, Errorf(param, err.Error())
	}
	return f, nil
}
func GetParamFloat64(r *http.Request, param string) float64 {
	f, err := GetParamFloat64Safe(r, param)
	if err != nil {
		panicAPI(err)
	}
	return f
}
