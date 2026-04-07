# base image for git integration tests (dependencies only)
FROM golang:1.25-alpine

RUN apk add --no-cache \
    git \
    openssh-client \
    openssh-keygen \
    bash \
    procps

WORKDIR /app

ENV GOTOOLCHAIN=auto

# cache dependencies
COPY go.mod go.sum ./
RUN go mod download
