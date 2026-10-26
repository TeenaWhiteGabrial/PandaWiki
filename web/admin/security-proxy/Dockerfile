FROM registry.cn-hangzhou.aliyuncs.com/misaka-images/golang:1.24.3-alpine AS builder
WORKDIR /src
COPY security-proxy/main.go ./main.go
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /security-proxy main.go

FROM scratch
COPY --from=builder /security-proxy /security-proxy
USER 65532:65532
ENTRYPOINT ["/security-proxy"]
