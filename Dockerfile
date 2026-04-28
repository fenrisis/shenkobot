# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/shenkobot ./

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shenkobot /shenkobot
USER nonroot:nonroot
ENTRYPOINT ["/shenkobot"]
