FROM registry.cn-hangzhou.aliyuncs.com/misaka-images/nginx:alpine
COPY dist /opt/frontend/dist
COPY server.conf /etc/nginx/conf.d/server.conf
COPY nginx.conf /etc/nginx/nginx.conf
COPY ssl /etc/nginx/ssl
RUN rm -f /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
