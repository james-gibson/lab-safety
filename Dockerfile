# syntax=docker/dockerfile:1.7
#
# Hardened, multi-arch images for the lab-safety edge stack. Each runtime is a
# static Go binary on distroless nonroot: no shell, no package manager, no
# mutable runtime, and UID 65532. Build with buildx for linux/amd64+arm64.
ARG GO_VERSION=1.27.2

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/wan-ingress ./cmd/wan-ingress && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/seed-invite ./cmd/seed-invite && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/seed-announce ./cmd/seed-announce

FROM gcr.io/distroless/static-debian12:nonroot AS wan-ingress
COPY --from=build /out/wan-ingress /wan-ingress
COPY --from=build /out/healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8443
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD ["/healthcheck", "-url", "http://127.0.0.1:8443/healthz"]
ENTRYPOINT ["/wan-ingress"]
CMD ["-config", "/etc/lab-safety/wan-ingress.json"]

FROM gcr.io/distroless/static-debian12:nonroot AS seed-invite
COPY --from=build /out/seed-invite /seed-invite
COPY --from=build /out/healthcheck /healthcheck
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD ["/healthcheck", "-url", "http://127.0.0.1:8080/healthz"]
ENTRYPOINT ["/seed-invite"]
CMD ["-config", "/etc/lab-safety/seed-invite.json"]

FROM gcr.io/distroless/static-debian12:nonroot AS seed-announce
COPY --from=build /out/seed-announce /seed-announce
USER nonroot:nonroot
ENTRYPOINT ["/seed-announce"]
