FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
COPY . .
RUN sqlc generate
RUN CGO_ENABLED=0 go build -o /out/HPay ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/HPay /HPay
ENTRYPOINT ["/HPay"]
CMD ["--role=all"]
