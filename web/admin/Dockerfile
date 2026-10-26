FROM registry.cn-hangzhou.aliyuncs.com/misaka-images/golang:1.24.3-alpine AS security-proxy-builder
WORKDIR /src
COPY security-proxy/main.go ./main.go
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /security-proxy main.go

FROM registry.cn-hangzhou.aliyuncs.com/misaka-images/nginx:alpine
COPY --from=security-proxy-builder /security-proxy /usr/local/bin/security-proxy
COPY dist /opt/frontend/dist
COPY docker-entrypoint.d/15-security-hosts.sh /docker-entrypoint.d/15-security-hosts.sh
COPY server.conf /etc/nginx/templates/server.conf.template
COPY nginx.conf /etc/nginx/nginx.conf
COPY ssl /etc/nginx/ssl
RUN rm -f /etc/nginx/conf.d/default.conf
ENV UPSTREAM_URL=http://127.0.0.1:8081
EXPOSE 80
CMD ["/bin/sh", "-c", "envsubst '${OSS_DOWNLOAD_URL}' < /etc/nginx/templates/server.conf.template > /etc/nginx/conf.d/server.conf && nginx && exec /usr/local/bin/security-proxy --no-auth"]
