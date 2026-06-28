FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/knowledge-server ./cmd/server

FROM scratch

WORKDIR /app
COPY --from=build /out/knowledge-server /app/knowledge-server

EXPOSE 8000

USER 65532:65532

ENTRYPOINT ["/app/knowledge-server"]
