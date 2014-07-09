package p2p

import (
	"github.com/cihub/seelog"
)

var log seelog.LoggerInterface

func init() {
	config := `
<seelog type="asyncloop" minlevel="debug">
    <outputs formatid="colored">
        <console/>
    </outputs>
    <formats>
        <format id="main"       format="%Date/%Time [%LEV] %Msg%n"/>
        <format id="colored"    format="%Time %EscM(46)%Level%EscM(49) %EscM(36)%File%EscM(39) %Msg%n%EscM(0)"/>
    </formats>
</seelog>`

	var err error
	log, err = seelog.LoggerFromConfigAsBytes([]byte(config))
	if err != nil {
		panic(err)
	}
}

func SetLogger(l seelog.LoggerInterface) {
	log = l
}
