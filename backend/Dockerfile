FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /iconfirm .

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache tzdata ca-certificates
COPY --from=build /iconfirm /app/iconfirm
COPY --from=build /app/assets /app/assets
RUN mkdir -p /app/uploads
EXPOSE 8080
CMD ["/app/iconfirm"]
