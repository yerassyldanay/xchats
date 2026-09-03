# Build the Go backend (migrations + sample media are go:embed'd into the binary).
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# VERSION defaults to "dev" for a local/CI build; release.yml passes the
# actual tag so internal/version.Version (the Settings page's "what am I
# running" surface, and GET /settings/update-check's comparison point)
# reflects the real release instead of its hardcoded source default.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-X github.com/yerassyldanay/xchats/backend/internal/version.Version=${VERSION}" \
    -o /out/xchats ./cmd/xchats

FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=build /out/xchats /xchats
EXPOSE 8080
ENTRYPOINT ["/xchats"]
CMD ["serve"]
