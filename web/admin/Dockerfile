FROM registry.cn-hangzhou.aliyuncs.com/misaka-images/nginx:alpine
COPY dist /opt/frontend/dist
COPY server.conf /etc/nginx/templates/server.conf.template
COPY nginx.conf /etc/nginx/nginx.conf
COPY ssl /etc/nginx/ssl
RUN rm -f /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["/bin/sh", "-c", "envsubst '${OSS_DOWNLOAD_URL}' < /etc/nginx/templates/server.conf.template > /etc/nginx/conf.d/server.conf && exec nginx -g 'daemon off;'"]
