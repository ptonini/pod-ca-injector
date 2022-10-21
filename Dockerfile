FROM golang:1.18 as build
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o serverd main.go

FROM gcr.io/distroless/static-debian11
# x-release-please-start-version
ENV VERSION="1.0.0"
# x-release-please-end

COPY --from=build /app/serverd /
EXPOSE 8443

CMD ["/serverd"]