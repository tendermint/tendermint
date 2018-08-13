package consensus

import "fmt"

// kind of arbitrary
var Spec = "1"     // async
var Major = "0"    //
var Minor = "2"    // replay refactor
var Revision = "2" // validation -> commit

var Version = fmt.Sprintf("v%s/%s.%s.%s", Spec, Major, Minor, Revision)
