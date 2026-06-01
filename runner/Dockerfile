FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o runner .

FROM mcr.microsoft.com/playwright:v1.60.0-jammy
WORKDIR /app

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
    xvfb \
    x11vnc \
    novnc \
    websockify \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/runner .

ENV TESTS_DIR=/tests

EXPOSE 8080
EXPOSE 6080-6089

CMD ["./runner"]
