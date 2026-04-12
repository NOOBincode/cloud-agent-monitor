FROM alpine:3.20
RUN apk add --no-cache bash curl
COPY inject.sh /scripts/inject.sh
RUN chmod +x /scripts/inject.sh
WORKDIR /scripts
ENV CHECKOUT_URL=http://checkout-sim:18080
ENV CHAOS_TOKEN=dev-chaos-token
# 未传参时打印用法；实际注入用 compose run 覆盖命令，见 docs/开发计划.md
CMD ["/scripts/inject.sh", "help"]
