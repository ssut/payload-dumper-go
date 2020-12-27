FROM golang as builder

RUN apt-get update \
    && apt-get install liblzma-dev

RUN git clone https://github.com/ssut/payload-dumper-go

RUN cd payload-dumper-go \
    && GOOS=linux go build -a -ldflags '-extldflags "-static"' /go/payload-dumper-go

FROM alpine
COPY --from=builder /go/payload-dumper-go/payload-dumper-go /go/bin/
ENTRYPOINT ["/go/bin/payload-dumper-go"]
