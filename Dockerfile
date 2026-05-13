FROM registry.cn-hangzhou.aliyuncs.com/deepcoin/alpine:latest
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY main .
COPY config ./config

EXPOSE 8080
CMD ["./main"]
