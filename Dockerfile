FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
COPY twilio-mock.go .

# @see https://medium.com/@diogok/on-golang-static-binaries-cross-compiling-and-plugins-1aed33499671
RUN apk add --no-cache git \
 && go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o twilio-mock twilio-mock.go


FROM scratch
COPY --from=builder /app/twilio-mock .
COPY public /public
ENTRYPOINT ["/twilio-mock"]
