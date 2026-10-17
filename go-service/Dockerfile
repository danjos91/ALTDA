FROM golang:1.25-alpine AS builder

ARG BUILD_PACKAGE=./cmd/app
ENV CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /build/app "$BUILD_PACKAGE"

FROM scratch

COPY --from=builder /build/app .

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=5 CMD ["/app", "healthcheck"]

USER 65534:65534
EXPOSE 8081
CMD ["/app"]
