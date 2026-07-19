# syntax=docker/dockerfile:1

# Multi-stage build for GS_PROJECT_NAME.
#
# Stage 1 compiles a static binary; stage 2 ships it on a distroless static base
# (no shell, no package manager, runs as non-root). Adjust GO_VERSION to match
# your toolchain. This Dockerfile is a starting point — edit freely.

ARG GO_VERSION=1.26

# ---- build stage ----
FROM golang:${GO_VERSION} AS build
WORKDIR /src

# Download modules first so they cache across source-only changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO off => a static binary that runs on distroless/static.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/GS_APP_NAME .

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=build /out/GS_APP_NAME /app/GS_APP_NAME
# Runtime assets. The app reads config from ./conf (spring.app.config.dir).
COPY conf /app/conf
COPY public /app/public

# Application port and the actuator management port (probes/metrics).
EXPOSE GS_APP_PORT GS_MGMT_PORT

USER nonroot:nonroot
ENTRYPOINT ["/app/GS_APP_NAME"]
