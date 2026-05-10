FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o runner .

# Playwright official image includes browsers + Node.js
FROM mcr.microsoft.com/playwright:v1.59.1-jammy
WORKDIR /app

# Copy Go binary
COPY --from=builder /app/runner .

# Copy tests (will be mounted as volume in dev)
# In prod, COPY tests are baked in
ENV TESTS_DIR=/tests

EXPOSE 8080
CMD ["./runner"]
