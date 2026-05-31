FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o runner .

FROM mcr.microsoft.com/playwright:v1.60.0-jammy
WORKDIR /app

# KasmVNC: Xvnc(X サーバー)+VNC+Web を一体提供。x11vnc/novnc/websockify を置き換える。
# arch は dpkg から検出し、arm64(ローカル)/amd64(PaaS) の両方に対応する。
ARG KASMVNC_VERSION=1.4.0
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y curl ca-certificates && \
    ARCH="$(dpkg --print-architecture)" && \
    curl -fsSL -o /tmp/kasmvnc.deb \
      "https://github.com/kasmtech/KasmVNC/releases/download/v${KASMVNC_VERSION}/kasmvncserver_jammy_${KASMVNC_VERSION}_${ARCH}.deb" && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y /tmp/kasmvnc.deb && \
    rm -f /tmp/kasmvnc.deb && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/runner .

ENV TESTS_DIR=/tests

EXPOSE 8080
EXPOSE 6080-6089

CMD ["./runner"]
