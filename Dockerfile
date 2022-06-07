
FROM golang:1.17.3-alpine3.14 as builder
RUN apk --no-cache add ca-certificates
RUN mkdir /build

WORKDIR /build

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build  -o app

# generate clean, final image for end users
FROM scratch


WORKDIR /build
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/app .

CMD ["/build/app" ]
