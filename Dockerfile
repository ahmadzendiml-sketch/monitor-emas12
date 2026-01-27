FROM golang:1.21-alpine as build
WORKDIR /app
COPY . .
RUN go build -o goldmonitor

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/goldmonitor .
COPY static ./static
EXPOSE 8000
CMD ["./goldmonitor"]