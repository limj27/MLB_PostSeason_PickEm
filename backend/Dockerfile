# Build context is the project root (see docker-compose.yml)
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN go build -o pickem-server .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/pickem-server ./pickem-server
COPY frontend/ ./frontend
EXPOSE 8080
CMD ["./pickem-server"]
