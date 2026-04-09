FROM golang:1.26.2 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/sub2clash ./cmd/sub2clash

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/sub2clash /usr/local/bin/sub2clash
ENV SUB2CLASH_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sub2clash", "serve"]
