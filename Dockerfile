FROM golang:1.24.0 AS builder
RUN apt-get update

# add local repo into the builder
ADD . /opt/build
WORKDIR /opt/build

# build the binary there
RUN CGO_ENABLED=0 go build -tags container -o fpl -v

# begin new container
FROM alpine
WORKDIR /

# add some tools
RUN apk add -U coreutils

# copy binary from builder to your desired location
COPY --from=builder /opt/build/fpl .
ENTRYPOINT ["/fpl", "-c", "config/config.yml"]
