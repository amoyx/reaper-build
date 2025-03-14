FROM golang:1.16.5 as build
RUN sed -i -E "s/[a-zA-Z0-9]+.debian.org/mirrors.aliyun.com/g" /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends libsasl2-dev=2.1.27+dfsg-1+deb10u1

WORKDIR /app

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
ENV GOPROXY=https://goproxy.cn,direct

ENV GO111MODULE=on
ENV ROOT_DIR=/app

# download and cache go dependencies
COPY go.* ./

RUN go mod download

COPY . .

RUN go build -v -o /reaper ./main.go

FROM ubuntu:bionic

RUN sed -i -E "s/[a-zA-Z0-9]+.ubuntu.com/mirrors.aliyun.com/g" /etc/apt/sources.list
RUN apt-get clean && apt-get update && apt-get install -y apt-transport-https ca-certificates
RUN DEBIAN_FRONTEND=noninteractive apt install -y tzdata
RUN apt-get install -y \
  curl \
  git \
  netcat-openbsd \
  wget \
  build-essential \
  libfontconfig \
  libsasl2-dev \
  libfreetype6-dev \
  libpcre3-dev \
  pkg-config \
  cmake \
  python \
  librrd-dev \
  sudo

# 修改时区
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

# 安装 docker client
RUN curl -fsSL "http://resources.koderover.com/docker-cli-v19.03.2.tar.gz" -o docker.tgz &&\
    tar -xvzf docker.tgz &&\
    mv docker/* /usr/bin


# 替换tar（适配cephfs）
RUN rm /bin/tar && curl -fsSL http://resource.koderover.com/tar -o /bin/tar && chmod +x /bin/tar

COPY --from=build /reaper /usr/local/bin

ENTRYPOINT ["reaper"]


