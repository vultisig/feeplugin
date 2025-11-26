ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN wget https://github.com/vultisig/go-wrappers/archive/refs/heads/master.tar.gz
RUN tar -xzf master.tar.gz && \
    cd go-wrappers-master && \
    mkdir -p /usr/local/lib/dkls && \
    cp --recursive includes /usr/local/lib/dkls

ENV LD_LIBRARY_PATH=/usr/local/lib/dkls/includes/linux/:$LD_LIBRARY_PATH

# Build the application
RUN go build -o bin/out/server ./cmd/server
RUN go build -o bin/out/worker ./cmd/worker
RUN go build -o bin/out/tx_indexer ./cmd/tx_indexer


FROM debian:bookworm

COPY --from=builder /usr/src/app/bin/out/ /usr/local/bin/
COPY --from=builder /usr/local/lib/dkls /usr/local/lib/dkls
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV LD_LIBRARY_PATH=/usr/local/lib/dkls/includes/linux/:$LD_LIBRARY_PATH

CMD ["server"]
