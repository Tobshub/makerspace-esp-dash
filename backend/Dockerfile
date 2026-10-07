FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api \
	&& CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate

FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
	&& adduser -D -u 1000 app
WORKDIR /app
COPY --from=build /out/api /out/migrate /usr/local/bin/
COPY --from=build /src/migrations ./migrations
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
USER app
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
