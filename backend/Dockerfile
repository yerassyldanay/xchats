# Build the Go backend (migrations + sample media are go:embed'd into the binary).
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/xchats ./cmd/xchats

FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=build /out/xchats /xchats
EXPOSE 8080
ENTRYPOINT ["/xchats"]
CMD ["serve"]
