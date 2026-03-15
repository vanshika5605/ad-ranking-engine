FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /ad-server .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /ad-server .
COPY --from=build /app/schema.sql /app/seed.sql .
EXPOSE 8080
CMD ["./ad-server"]
