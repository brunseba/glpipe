FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build \
  -ldflags "-s -w \
    -X 'github.com/sebrun/glpipe/internal/build.Version=${VERSION}' \
    -X 'github.com/sebrun/glpipe/internal/build.Commit=${COMMIT}' \
    -X 'github.com/sebrun/glpipe/internal/build.BuildDate=${BUILD_DATE}'" \
  -o /glpipe .

FROM scratch
COPY --from=builder /glpipe /glpipe
ENTRYPOINT ["/glpipe"]
