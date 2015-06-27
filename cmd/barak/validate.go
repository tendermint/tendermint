package main

import (
	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/cmd/barak/types"
)

func validate(signBytes []byte, validators []Validator, signatures []acm.Signature) bool {
	var signedPower int64
	var totalPower int64
	for i, val := range validators {
		if val.PubKey.VerifyBytes(signBytes, signatures[i]) {
			signedPower += val.VotingPower
			totalPower += val.VotingPower
		} else {
			totalPower += val.VotingPower
		}
	}
	return signedPower > totalPower*2/3
}

/*
NOTE: Not used, just here in case we want it later.
func ValidateHandler(handler http.Handler, validators []Validator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigStrs := r.Header[http.CanonicalHeaderKey("signatures")]
		log.Debug("Woot", "sigstrs", sigStrs, "len", len(sigStrs))
		// from https://medium.com/@xoen/golang-read-from-an-io-readwriter-without-loosing-its-content-2c6911805361
		// Read the content
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = ioutil.ReadAll(r.Body)
		}
		// Restore the io.ReadCloser to its original state
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		// Get body string
		bodyString := string(bodyBytes)
		// Also read the path+"?"+query
		pathQuery := fmt.Sprintf("%v?%v", r.URL.Path, r.URL.RawQuery)
		// Concatenate into tuple of two strings.
		tuple := struct {
			Body string
			Path string
		}{bodyString, pathQuery}
		// Get sign bytes
		signBytes := binary.BinaryBytes(tuple)
		// Validate the sign bytes.
		//if validate(signBytes, validators,
		log.Debug("Should sign", "bytes", signBytes)
		// If validation fails
		// XXX
		// If validation passes
		handler.ServeHTTP(w, r)
	})
}
*/
