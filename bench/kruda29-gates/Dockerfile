ARG GO_IMAGE=golang:1.26.9-bookworm
FROM ${GO_IMAGE}

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["bash", "bench/kruda29-gates/run.sh"]
