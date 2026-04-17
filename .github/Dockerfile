# Context of the Dockerfile is the root of the repository

FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN echo "nobody:x:65534:65534:Nobody:/:" > /etc_passwd

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -installsuffix cgo -o /app/nastene ./cmd/nastene

FROM scratch

COPY --from=builder /app/nastene /bin/nastene
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc_passwd /etc/passwd

USER nobody
EXPOSE 8080
ENTRYPOINT ["/bin/nastene"]
