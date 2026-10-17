# Multi-stage build for ALTDA Go skills service.
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/altda-go ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 altda
WORKDIR /app
COPY --from=build /out/altda-go /app/altda-go
COPY testdata /app/testdata
COPY run.sh /app/run.sh
RUN chmod +x /app/run.sh /app/altda-go
USER altda
ENV PORT=8081
EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/altda-go", "-healthcheck", "-addr=:8081"]
ENTRYPOINT ["/app/run.sh"]
