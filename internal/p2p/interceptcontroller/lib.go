package interceptcontroller

type Controller interface {
	// TODO: Need to figure out what goes into the channel
	SendChan() chan interface{}
	ReceiveChan() chan interface{}
	Run()
}
