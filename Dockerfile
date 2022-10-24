FROM golang:1.18 as build
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o serverd main.go && \
    mkdir /config && \
    touch /config/config.yaml

FROM gcr.io/distroless/static-debian11
# x-release-please-start-version
ENV VERSION="1.2.1"
# x-release-please-end

COPY --from=build /app/serverd /
COPY --from=build /config /
COPY --from=build /config/config.yaml /config/config.yaml

EXPOSE 8443

CMD ["/serverd"]