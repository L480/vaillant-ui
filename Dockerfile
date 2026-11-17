FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build
ARG TARGETOS TARGETARCH TARGETVARIANT VERSION=dev
WORKDIR /src
COPY . .
RUN go test ./... && \
    v="${TARGETVARIANT:-v7}" && v="${v#v}" && if [ "$v" = 8 ]; then v=7; fi && \
    GOARM=$v CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /vaillant-ui .

FROM scratch
COPY --from=build /vaillant-ui /vaillant-ui
USER 6000
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=5s CMD ["/vaillant-ui", "-healthcheck"]
ENTRYPOINT ["/vaillant-ui"]
