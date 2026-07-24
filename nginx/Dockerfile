FROM nginx:alpine

ARG NGINX_CONF=nginx.conf

RUN rm -f /etc/nginx/conf.d/default.conf
COPY ${NGINX_CONF} /etc/nginx/nginx.conf

EXPOSE 80 443
CMD ["nginx", "-g", "daemon off;"]
