# syntax=docker/dockerfile:1.7
FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/leakshield ./cmd/leakshield

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && \
    addgroup -S leakshield && adduser -S -G leakshield leakshield && \
    mkdir -p /var/lib/leakshield && chown leakshield:leakshield /var/lib/leakshield
COPY --from=build /out/leakshield /usr/local/bin/leakshield
USER leakshield
EXPOSE 8080 8090 9090
ENTRYPOINT ["/usr/local/bin/leakshield"]
CMD ["serve"]
