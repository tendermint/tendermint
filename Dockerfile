FROM golang:1.9.2

ENV LINTER=/go/src/github.com/alecthomas/gometalinter

RUN echo $GOPATH

RUN go get github.com/alecthomas/gometalinter && \
	cd $LINTER && \
	git checkout v2.0.2 && \
	go install && \
	gometalinter --install

RUN mkdir /code
WORKDIR /code
