FROM golang:1.26-alpine AS builder
WORKDIR /build
# Copy framework source (needed for replace directive)
COPY . /kruda
# Copy TFB app source
COPY src/ /build/
# Adjust replace directive to point to copied framework source
RUN sed -i 's|../../../../|/kruda|g' /build/go.mod
RUN cd /build && go mod download
RUN CGO_ENABLED=0 GOOS=linux GOAMD64=v3 go build -ldflags="-s -w" -gcflags="-B" -trimpath -o /app .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /app /app
EXPOSE 8080
ENV KRUDA_TURBO=1
CMD ["/app"]
