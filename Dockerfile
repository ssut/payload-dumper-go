FROM golang:1.27 AS builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends liblzma-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags '-extldflags "-static"' \
    -o /out/payload-dumper-go .

RUN mkdir -p /rootfs/tmp /rootfs/data \
    && chmod 1777 /rootfs/tmp /rootfs/data

FROM scratch
COPY --from=builder /rootfs /
COPY --from=builder /out/payload-dumper-go /payload-dumper-go
USER 65534:65534
WORKDIR /data
ENTRYPOINT ["/payload-dumper-go"]
